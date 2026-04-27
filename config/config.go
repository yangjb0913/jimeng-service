package config

import (
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Jimeng   JimengConfig   `yaml:"jimeng"`
	Upload   UploadConfig   `yaml:"upload"`
	KeyPool  KeyPoolConfig  `yaml:"keypool"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type JimengConfig struct {
	APIHost  string `yaml:"api_host"`
	Region   string `yaml:"region"`
	Service  string `yaml:"service"`
	Version  string `yaml:"version"`
}

type UploadConfig struct {
	Path    string `yaml:"path"`
	MaxSize int64  `yaml:"max_size"`
}

type KeyPoolConfig struct {
	DataFile      string         `yaml:"data_file"`
	CreateSample  bool           `yaml:"create_sample"`
	DefaultQuotas DefaultQuotas  `yaml:"default_quotas"`
}

type DefaultQuotas struct {
	VideoSeconds int `yaml:"video_seconds"`
	ImageCount   int `yaml:"image_count"`
}

var (
	cfg  *Config
	once sync.Once
)

func Load(path string) (*Config, error) {
	var err error
	once.Do(func() {
		cfg, err = loadConfig(path)
	})
	return cfg, err
}

func Get() *Config {
	return cfg
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
