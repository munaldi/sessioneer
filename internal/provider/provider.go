package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/munaldi/sessioneer/pkg/types"
)

// Detect returns providers whose default directories currently exist.
func Detect() []types.Provider {
	providers := make([]types.Provider, 0, 2)
	if dir, err := DefaultBaseDir(types.ProviderClaude); err == nil {
		if exists(dir) {
			providers = append(providers, types.ProviderClaude)
		}
	}
	if dir, err := DefaultBaseDir(types.ProviderCopilot); err == nil {
		if exists(dir) {
			providers = append(providers, types.ProviderCopilot)
		}
	}
	return providers
}

// DefaultBaseDir returns the default session directory for a provider.
func DefaultBaseDir(p types.Provider) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	switch p {
	case types.ProviderClaude:
		return filepath.Join(home, ".claude", "projects"), nil
	case types.ProviderCopilot:
		switch runtime.GOOS {
		case "darwin":
			return filepath.Join(home, "Library", "Application Support", "github-copilot", "conversations"), nil
		case "windows":
			if appData := os.Getenv("APPDATA"); appData != "" {
				return filepath.Join(appData, "github-copilot", "conversations"), nil
			}
			return filepath.Join(home, "AppData", "Roaming", "github-copilot", "conversations"), nil
		default:
			return filepath.Join(home, ".config", "github-copilot", "conversations"), nil
		}
	default:
		return "", fmt.Errorf("unknown provider %q", p)
	}
}

func exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}