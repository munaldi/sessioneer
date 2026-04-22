package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/munaldi/sessioneer/internal/provider"
	"github.com/munaldi/sessioneer/pkg/types"
)

// Resolve converts CLI flags into a final application config.
func Resolve(flagProvider, flagProject, flagBase string) (types.AppConfig, error) {
	cfg := types.AppConfig{ProjectPath: flagProject}
	if cfg.ProjectPath == "" {
		if cwd, err := os.Getwd(); err == nil {
			cfg.ProjectPath = cwd
		}
	}

	if flagProvider != "" {
		p := types.Provider(strings.ToLower(flagProvider))
		switch p {
		case types.ProviderClaude, types.ProviderCopilot:
			cfg.Provider = p
		default:
			return types.AppConfig{}, fmt.Errorf("unknown provider %q", flagProvider)
		}
	} else {
		detected := provider.Detect()
		if len(detected) == 1 {
			cfg.Provider = detected[0]
		}
	}

	if flagBase != "" {
		cfg.BaseDir = flagBase
		return cfg, nil
	}

	if cfg.Provider != "" {
		baseDir, err := provider.DefaultBaseDir(cfg.Provider)
		if err != nil {
			return types.AppConfig{}, err
		}
		cfg.BaseDir = baseDir
	}

	return cfg, nil
}