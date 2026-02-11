package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ModelConfig represents a user-defined custom model.
type ModelConfig struct {
	Name      string `json:"name"`
	HeaderVal string `json:"header_val"`
}

type Config struct {
	DefaultAccount string                 `json:"default_account"`
	DefaultModel   string                 `json:"default_model"`
	RequestTimeout int                    `json:"request_timeout"`
	RequestDelay   int                    `json:"request_delay_ms"`
	Proxy          string                 `json:"proxy,omitempty"`
	CustomModels   map[string]ModelConfig `json:"custom_models,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		DefaultAccount: "default",
		DefaultModel:   "gemini-2.5-pro",
		RequestTimeout: 120,
		RequestDelay:   500,
	}
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	// XDG_CONFIG_HOME support
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "gemini-web-cli"), nil
	}
	return filepath.Join(home, ".config", "gemini-web-cli"), nil
}

func configPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return DefaultConfig(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
