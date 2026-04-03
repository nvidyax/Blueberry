package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	AllowedRoots []string
}

func LoadConfig() *Config {
	return &Config{
		AllowedRoots: []string{},
	}
}

func EnsureBerryHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	berryHome := filepath.Join(home, ".berry")
	os.MkdirAll(berryHome, 0755)
	return berryHome
}

func RunsDir() string {
	d := filepath.Join(EnsureBerryHome(), "runs")
	os.MkdirAll(d, 0755)
	return d
}

func RunDir(runID string) string {
	d := filepath.Join(RunsDir(), runID)
	os.MkdirAll(d, 0755)
	return d
}
