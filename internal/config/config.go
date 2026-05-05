package config

import (
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
	"niecke-it.de/uptime/internal/models"
)

func LoadConfig(path string) (models.Config, error) {
	var conf models.Config
	path, err := findConfigPath(path)
	if err != nil {
		return conf, err
	}

	slog.Info("Loading config", "path", path)
	file, err := os.ReadFile(path)
	if err != nil {
		return conf, fmt.Errorf("Error opening file %v", path)
	}
	if err := yaml.Unmarshal(file, &conf); err != nil {
		return conf, fmt.Errorf("Error parsing config file %v", err.Error())
	}

	if len(conf.Endpoints) == 0 {
		return conf, fmt.Errorf("config has no endpoints defined")
	}
	if conf.Global.IntervalSeconds == 0 {
		return conf, fmt.Errorf("global.interval_seconds must be set")
	}
	return conf, nil
}

func findConfigPath(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}

	candidates := []string{"./config.yml", "/data/config.yml"}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no config file found")
}
