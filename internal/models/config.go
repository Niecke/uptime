package models

type Config struct {
	Global    GlobalConfig     `yaml:"global"`
	Alertings []AlertingConfig `yaml:"alerting"`
	Endpoints []string         `yaml:"endpoints"`
}

type GlobalConfig struct {
	TimeoutSeconds  int    `yaml:"timeout_seconds"`
	IntervalSeconds int    `yaml:"interval_seconds"`
	LogLevel        string `yaml:"log_level"`
	RetentionDays   int    `yaml:"retention_days"`
}

type AlertingConfig struct {
	Type      string `yaml:"type"`
	Threshold int    `yaml:"threshold"`
	Address   string `yaml:"address"`
}
