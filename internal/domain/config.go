package domain

import "time"

type Config struct {
	Version          string
	ConfigPath       string
	DownloadLocation string                     `yaml:"downloadLocation"`
	NamingTemplate   string                     `yaml:"namingTemplate"`
	CheckInterval    time.Duration              `yaml:"checkInterval"`
	PprofEnabled     bool                       `yaml:"pprofEnabled"`
	PprofAddress     string                     `yaml:"pprofAddress"`
	MonitoredManga   map[string]*MonitoredManga `yaml:"monitoredManga"`
	LogPath          string                     `yaml:"logPath"`
	LogLevel         string                     `yaml:"logLevel"`
	LogMaxSize       int                        `yaml:"logMaxSize"` // in megabytes
	LogMaxBackups    int                        `yaml:"logMaxBackups"`
}

type MonitoredManga struct {
	Source    string `yaml:"source"`
	Manga     string `yaml:"manga"`
	Group     string `yaml:"group"`
	Language  string `yaml:"language"`
	Overwrite string `yaml:"overwrite"`
}
