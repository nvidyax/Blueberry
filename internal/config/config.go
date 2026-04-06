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

func EnsureBlueberryHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	blueberryHome := filepath.Join(home, ".blueberry")
	os.MkdirAll(blueberryHome, 0755)
	return blueberryHome
}

func RunsDir() string {
	d := filepath.Join(EnsureBlueberryHome(), "runs")
	os.MkdirAll(d, 0755)
	return d
}

func RunDir(runID string) string {
	d := filepath.Join(RunsDir(), runID)
	os.MkdirAll(d, 0755)
	return d
}
