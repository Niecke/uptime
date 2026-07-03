package config

import (
	"fmt"
	"log/slog"
	"math"
	"os"

	"gopkg.in/yaml.v3"
	"niecke-it.de/uptime/internal/models"
)

const (
	defaultThreshold = 3
	maxThreshold     = math.MaxUint8 // 255
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
	if conf.Global.RetentionDays == 0 {
		conf.Global.RetentionDays = 30
	}
	// threshold default = 3
	// threshold max = 255 (uint8)
	for i := range conf.Alertings {
		if conf.Alertings[i].Threshold == 0 {
			conf.Alertings[i].Threshold = defaultThreshold
		}
		if conf.Alertings[i].Threshold > maxThreshold {
			slog.Warn("Threshold exceeds max. Capping.", "threshold", conf.Alertings[i].Threshold, "max_threshold", maxThreshold, "type", conf.Alertings[i].Type)
			conf.Alertings[i].Threshold = maxThreshold
		}
	}
	return conf, nil
}

func findConfigPath(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}

	candidates := []string{"./config.yml", "/config.yml", "/data/config.yml"}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no config file found")
}
