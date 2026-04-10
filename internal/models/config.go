package models

type Config struct {
	Global    GlobalConfig `yaml:"global"`
	Endpoints []string     `yaml:"endpoints"`
}

type GlobalConfig struct {
	TimeoutSeconds  int `yaml:"timeout_seconds"`
	IntervalSeconds int `yaml:"interval_seconds"`
}
