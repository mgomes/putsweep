package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ScheduleMode string

const (
	ScheduleAnyTime       ScheduleMode = "any_time"
	ScheduleAfterMidnight ScheduleMode = "after_midnight"
	ScheduleBusinessHours ScheduleMode = "business_hours"
)

type Config struct {
	OAuthToken   string       `json:"oauth_token"`
	DownloadDir  string       `json:"download_dir"`
	ScheduleMode ScheduleMode `json:"schedule_mode"`
	Boost        int          `json:"boost"`
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "putsweep"), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaultConfig(), nil
	}
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.Boost == 0 {
		cfg.Boost = 8
	}
	if cfg.ScheduleMode == "" {
		cfg.ScheduleMode = ScheduleAnyTime
	}
	if cfg.DownloadDir == "" {
		cfg.DownloadDir = defaultDownloadDir()
	}

	return &cfg, nil
}

func (c *Config) Save() error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	data = append(data, '\n')
	return os.WriteFile(path, data, 0600)
}

func defaultConfig() *Config {
	return &Config{
		DownloadDir:  defaultDownloadDir(),
		ScheduleMode: ScheduleAnyTime,
		Boost:        8,
	}
}

func defaultDownloadDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Downloads")
}
