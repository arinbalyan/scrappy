package config

import (
	"bufio"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

var ConfigPaths = []string{
	"./config.toml",
	filepath.Join(os.Getenv("HOME"), ".scrappy", "config.toml"),
}

func Load(p string) (*RunConfig, error) {
	if p == "" {
		for _, c := range ConfigPaths {
			if _, err := os.Stat(c); err == nil {
				p = c
				break
			}
		}
	}
	if p == "" {
		return nil, ErrNoConfig
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg RunConfig
	if _, err := toml.NewDecoder(bufio.NewReader(f)).Decode(&cfg); err != nil {
		return nil, err
	}
	if cfg.Defaults.Concurrency == 0 {
		cfg.Defaults.Concurrency = 4
	}
	if cfg.Defaults.DelayMs == 0 {
		cfg.Defaults.DelayMs = 1000
	}
	return &cfg, nil
}

var ErrNoConfig = configLoadError("no config.toml found (expected ./config.toml or ~/.scrappy/config.toml)")

type configLoadError string

func (e configLoadError) Error() string { return string(e) }
