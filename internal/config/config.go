// Package config gerencia a configuração persistente do aplicativo.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	// DefaultMaxConcurrent é o número padrão de conversões simultâneas.
	DefaultMaxConcurrent = 8
	// DefaultQuality é a qualidade padrão do WebP (lossy, 0-100).
	DefaultQuality = 90.0
)

// Config representa as opções persistidas em config.json.
type Config struct {
	WatchDir       string  `json:"watch_dir"`
	OutputDir      string  `json:"output_dir"`
	MaxConcurrent  int     `json:"max_concurrent"`
	Quality        float64 `json:"quality"`
	DeleteOriginal bool    `json:"delete_original"`
}

// Default retorna uma configuração com valores padrão.
func Default() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		WatchDir:       filepath.Join(home, "Pictures", "image_reduce", "in"),
		OutputDir:      filepath.Join(home, "Pictures", "image_reduce", "out"),
		MaxConcurrent:  DefaultMaxConcurrent,
		Quality:        DefaultQuality,
		DeleteOriginal: false,
	}
}

// ConfigDir retorna o diretório de configuração do app (~/.config/image_reduce).
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "image_reduce"), nil
}

// ConfigPath retorna o caminho do arquivo config.json.
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// HistoryPath retorna o caminho do arquivo de histórico (JSONL).
func HistoryPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "history.jsonl"), nil
}

// Load carrega a configuração do disco. Se não existir, cria com padrões.
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := cfg.Save(); err != nil {
				return nil, err
			}
			return cfg, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.EnsureDefaults()
	return cfg, nil
}

// EnsureDefaults corrige valores inválidos ou vazios.
func (c *Config) EnsureDefaults() {
	d := Default()
	if c.MaxConcurrent < 1 {
		c.MaxConcurrent = DefaultMaxConcurrent
	}
	if c.Quality <= 0 || c.Quality > 100 {
		c.Quality = DefaultQuality
	}
	if c.WatchDir == "" {
		c.WatchDir = d.WatchDir
	}
	if c.OutputDir == "" {
		c.OutputDir = d.OutputDir
	}
}

// Save persiste a configuração em disco.
func (c *Config) Save() error {
	c.EnsureDefaults()
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// EnsureDirs cria as pastas de entrada e saída se não existirem.
func (c *Config) EnsureDirs() error {
	if err := os.MkdirAll(c.WatchDir, 0o755); err != nil {
		return err
	}
	return os.MkdirAll(c.OutputDir, 0o755)
}

// ProcessedDir retorna a subpasta onde originais mantidos são movidos.
func (c *Config) ProcessedDir() string {
	return filepath.Join(c.WatchDir, "processed")
}