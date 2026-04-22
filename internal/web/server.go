package web

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/munaldi/sessioneer/internal/actions"
	"github.com/munaldi/sessioneer/internal/provider"
	"github.com/munaldi/sessioneer/internal/session"
	"github.com/munaldi/sessioneer/pkg/types"
)

//go:embed static
var staticFiles embed.FS

// Server serves the Sessioneer web UI and its REST API.
type Server struct {
	defaultProvider types.Provider
	defaultBase     string
	port            int
}

// New creates a Server. provider and baseDir may be empty for auto-detect.
func New(prov types.Provider, baseDir string, port int) *Server {
	return &Server{
		defaultProvider: prov,
		defaultBase:     baseDir,
		port:            port,
	}
}

// Run starts the HTTP server and opens the browser.
func (s *Server) Run() error {
	mux := http.NewServeMux()

	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("embed fs: %w", err)
	}
	mux.Handle("/", http.FileServerFS(sub))

	mux.HandleFunc("GET /api/providers", s.handleProviders)
	mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	mux.HandleFunc("GET /api/sessions/{id}", s.handleSessionDetail)
	mux.HandleFunc("POST /api/sessions/merge", s.handleMerge)
	mux.HandleFunc("POST /api/sessions/{id}/fork", s.handleFork)
	mux.HandleFunc("POST /api/sessions/{id}/rename", s.handleRename)
	mux.HandleFunc("POST /api/sessions/{id}/prune", s.handlePrune)
	mux.HandleFunc("POST /api/sessions/{id}/trim", s.handleTrim)
	mux.HandleFunc("DELETE /api/sessions/{id}", s.handleDelete)
	mux.HandleFunc("GET /api/stats", s.handleStats)
	mux.HandleFunc("POST /api/clean-empty", s.handleCleanEmpty)

	addr := fmt.Sprintf(":%d", s.port)
	url := fmt.Sprintf("http://localhost%s", addr)
	fmt.Printf("Sessioneer web UI → %s\n", url)

	go openBrowser(url)
	return http.ListenAndServe(addr, mux)
}

// resolveProvider reads ?provider and ?base from the request, falling back to
// server defaults and then to auto-detect / platform defaults.
func (s *Server) resolveProvider(r *http.Request) (types.Provider, string, error) {
	prov := types.Provider(r.URL.Query().Get("provider"))
	base := r.URL.Query().Get("base")

	if prov == "" {
		if s.defaultProvider != "" {
			prov = s.defaultProvider
		} else {
			detected := provider.Detect()
			if len(detected) == 0 {
				return "", "", fmt.Errorf("no providers detected")
			}
			prov = detected[0]
		}
	}

	if base == "" {
		if s.defaultBase != "" && s.defaultProvider == prov {
			base = s.defaultBase
		} else {
			var err error
			base, err = provider.DefaultBaseDir(prov)
			if err != nil {
				return "", "", err
			}
		}
	}

	return prov, base, nil
}

// --- session ID encoding (base64url of the file path) ---

func encodeID(filePath string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(filePath))
}

func decodeID(id string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return "", fmt.Errorf("invalid session id")
	}
	return string(b), nil
}

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, struct {
		Error string `json:"error"`
	}{Error: msg})
}

func decodeBody(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return fmt.Errorf("invalid request body")
	}
	return nil
}

// --- API response types ---

type sessionSummary struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Provider     string `json:"provider"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	IsEmpty      bool   `json:"isEmpty"`
	MessageCount int    `json:"messageCount"`
}

type messageJSON struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	IsTool    bool   `json:"isTool"`
	ToolName  string `json:"toolName"`
}

type sessionDetail struct {
	ID        string        `json:"id"`
	Title     string        `json:"title"`
	Provider  string        `json:"provider"`
	CreatedAt string        `json:"createdAt"`
	UpdatedAt string        `json:"updatedAt"`
	IsEmpty   bool          `json:"isEmpty"`
	Messages  []messageJSON `json:"messages"`
}

type statsJSON struct {
	SessionCount int            `json:"sessionCount"`
	MessageCount int            `json:"messageCount"`
	UserMessages int            `json:"userMessages"`
	LLMMessages  int            `json:"llmMessages"`
	DurationSec  float64        `json:"durationSec"`
	TotalTokens  int            `json:"totalTokens"`
	ToolUsage    map[string]int `json:"toolUsage"`
}

// --- conversion helpers ---

const timeFmt = time.RFC3339

func toSummary(s types.Session) sessionSummary {
	return sessionSummary{
		ID:           encodeID(s.FilePath),
		Title:        s.Title,
		Provider:     string(s.Provider),
		CreatedAt:    s.CreatedAt.Format(timeFmt),
		UpdatedAt:    s.UpdatedAt.Format(timeFmt),
		IsEmpty:      s.IsEmpty,
		MessageCount: len(s.Messages),
	}
}

func toDetail(s types.Session) sessionDetail {
	msgs := make([]messageJSON, 0, len(s.Messages))
	for _, m := range s.Messages {
		msgs = append(msgs, messageJSON{
			Role:      string(m.Role),
			Content:   m.Content,
			Timestamp: m.Timestamp.Format(timeFmt),
			IsTool:    m.IsTool,
			ToolName:  m.ToolName,
		})
	}
	return sessionDetail{
		ID:        encodeID(s.FilePath),
		Title:     s.Title,
		Provider:  string(s.Provider),
		CreatedAt: s.CreatedAt.Format(timeFmt),
		UpdatedAt: s.UpdatedAt.Format(timeFmt),
		IsEmpty:   s.IsEmpty,
		Messages:  msgs,
	}
}

func toStats(st types.SessionStats) statsJSON {
	total := st.Tokens.InputTokens + st.Tokens.OutputTokens +
		st.Tokens.CacheCreation + st.Tokens.CacheRead
	return statsJSON{
		SessionCount: st.SessionCount,
		MessageCount: st.MessageCount,
		UserMessages: st.UserMessages,
		LLMMessages:  st.LLMMessages,
		DurationSec:  st.Duration.Seconds(),
		TotalTokens:  total,
		ToolUsage:    st.ToolUsage,
	}
}

// findSession loads all sessions and returns the one matching filePath.
func findSession(prov types.Provider, baseDir, filePath string) (types.Session, error) {
	sessions, err := session.List(prov, baseDir)
	if err != nil {
		return types.Session{}, err
	}
	for _, s := range sessions {
		if s.FilePath == filePath {
			return s, nil
		}
	}
	return types.Session{}, fmt.Errorf("session not found")
}

// --- Handlers ---

func (s *Server) handleProviders(w http.ResponseWriter, _ *http.Request) {
	detected := provider.Detect()
	names := make([]string, 0, len(detected))
	for _, p := range detected {
		names = append(names, string(p))
	}
	writeJSON(w, http.StatusOK, struct {
		Providers []string `json:"providers"`
	}{Providers: names})
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	prov, base, err := s.resolveProvider(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	sessions, err := session.List(prov, base)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	q := r.URL.Query().Get("q")
	if q != "" {
		results := actions.Search(sessions, q)
		seen := map[string]bool{}
		summaries := make([]sessionSummary, 0, len(results))
		for _, res := range results {
			id := encodeID(res.Session.FilePath)
			if !seen[id] {
				summaries = append(summaries, toSummary(res.Session))
				seen[id] = true
			}
		}
		writeJSON(w, http.StatusOK, struct {
			Sessions []sessionSummary `json:"sessions"`
		}{Sessions: summaries})
		return
	}

	summaries := make([]sessionSummary, 0, len(sessions))
	for _, sess := range sessions {
		summaries = append(summaries, toSummary(sess))
	}
	writeJSON(w, http.StatusOK, struct {
		Sessions []sessionSummary `json:"sessions"`
	}{Sessions: summaries})
}

func (s *Server) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	filePath, err := decodeID(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	prov, base, err := s.resolveProvider(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	sess, err := findSession(prov, base, filePath)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}

	st := session.Stats(sess)
	writeJSON(w, http.StatusOK, struct {
		Session sessionDetail `json:"session"`
		Stats   statsJSON     `json:"stats"`
	}{Session: toDetail(sess), Stats: toStats(st)})
}

func (s *Server) handleFork(w http.ResponseWriter, r *http.Request) {
	filePath, err := decodeID(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		Title string `json:"title"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	prov, base, err := s.resolveProvider(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	sess, err := findSession(prov, base, filePath)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}

	forked, err := actions.Fork(sess, body.Title)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, struct {
		Session sessionSummary `json:"session"`
	}{Session: toSummary(forked)})
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	filePath, err := decodeID(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		Title string `json:"title"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	prov, base, err := s.resolveProvider(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	sess, err := findSession(prov, base, filePath)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}

	renamed, err := actions.Rename(sess, body.Title)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, struct {
		Session sessionSummary `json:"session"`
	}{Session: toSummary(renamed)})
}

func (s *Server) handlePrune(w http.ResponseWriter, r *http.Request) {
	filePath, err := decodeID(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		RemoveToolBlocks    bool `json:"removeToolBlocks"`
		RemoveEmptyMessages bool `json:"removeEmptyMessages"`
		RemoveSystemTags    bool `json:"removeSystemTags"`
		RemoveShortMessages bool `json:"removeShortMessages"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	prov, base, err := s.resolveProvider(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	sess, err := findSession(prov, base, filePath)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}

	pruned, err := actions.Prune(sess, actions.PruneOptions{
		RemoveToolBlocks:    body.RemoveToolBlocks,
		RemoveEmptyMessages: body.RemoveEmptyMessages,
		RemoveSystemTags:    body.RemoveSystemTags,
		RemoveShortMessages: body.RemoveShortMessages,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, struct {
		Session sessionDetail `json:"session"`
	}{Session: toDetail(pruned)})
}

func (s *Server) handleTrim(w http.ResponseWriter, r *http.Request) {
	filePath, err := decodeID(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	prov, base, err := s.resolveProvider(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	sess, err := findSession(prov, base, filePath)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}

	cutoff := len(sess.Messages) - 2
	if cutoff < 0 {
		writeErr(w, http.StatusBadRequest, "not enough messages to trim")
		return
	}

	trimmed, err := actions.Trim(sess, cutoff)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, struct {
		Session sessionDetail `json:"session"`
	}{Session: toDetail(trimmed)})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	filePath, err := decodeID(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	prov, base, err := s.resolveProvider(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	sess, err := findSession(prov, base, filePath)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}

	if err := actions.Delete(sess); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, struct {
		OK bool `json:"ok"`
	}{OK: true})
}

func (s *Server) handleMerge(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs   []string `json:"ids"`
		Title string   `json:"title"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	prov, base, err := s.resolveProvider(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	allSessions, err := session.List(prov, base)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	wanted := map[string]bool{}
	for _, id := range body.IDs {
		if fp, err := decodeID(id); err == nil {
			wanted[fp] = true
		}
	}

	toMerge := make([]types.Session, 0, len(body.IDs))
	for _, sess := range allSessions {
		if wanted[sess.FilePath] {
			toMerge = append(toMerge, sess)
		}
	}

	if len(toMerge) < 2 {
		writeErr(w, http.StatusBadRequest, "select at least 2 sessions to merge")
		return
	}

	merged, err := actions.Merge(toMerge, body.Title)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, struct {
		Session sessionSummary `json:"session"`
	}{Session: toSummary(merged)})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	prov, base, err := s.resolveProvider(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	sessions, err := session.List(prov, base)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toStats(session.ProjectStats(sessions)))
}

func (s *Server) handleCleanEmpty(w http.ResponseWriter, r *http.Request) {
	prov, base, err := s.resolveProvider(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	sessions, err := session.List(prov, base)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	removed, err := actions.CleanEmpty(sessions)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, struct {
		Removed int `json:"removed"`
	}{Removed: removed})
}

func openBrowser(url string) {
	time.Sleep(150 * time.Millisecond)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start() //nolint:errcheck
}
