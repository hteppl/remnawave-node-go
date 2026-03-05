package config

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
)

const (
	DefaultNodePort         = 2222
	DefaultInternalRestPort = 61001
	DefaultLogLevel         = "info"
)

var (
	ErrConfigSecretKeyRequired = errors.New("SECRET_KEY environment variable is required")
)

type Config struct {
	SecretKey        string
	NodePort         int
	InternalRestPort int
	LogLevel         string

	Payload *NodePayload
}

func Load() (*Config, error) {
	cfg := &Config{
		NodePort:         DefaultNodePort,
		InternalRestPort: DefaultInternalRestPort,
		LogLevel:         DefaultLogLevel,
	}

	if configPath := os.Getenv("CONFIG_PATH"); configPath != "" {
		loadFromFile(cfg, configPath)
	}

	loadFromEnv(cfg)

	if cfg.SecretKey == "" {
		return nil, ErrConfigSecretKeyRequired
	}

	payload, err := ParseSecretKey(cfg.SecretKey)
	if err != nil {
		return nil, err
	}
	cfg.Payload = payload

	return cfg, nil
}

func loadFromFile(cfg *Config, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var file struct {
		NodePort         *int    `json:"nodePort"`
		InternalRestPort *int    `json:"internalRestPort"`
		LogLevel         *string `json:"logLevel"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return
	}

	if file.NodePort != nil {
		cfg.NodePort = *file.NodePort
	}
	if file.InternalRestPort != nil {
		cfg.InternalRestPort = *file.InternalRestPort
	}
	if file.LogLevel != nil {
		cfg.LogLevel = *file.LogLevel
	}
}

func loadFromEnv(cfg *Config) {
	if v := os.Getenv("SECRET_KEY"); v != "" {
		cfg.SecretKey = v
	}
	if v := os.Getenv("NODE_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			cfg.NodePort = port
		}
	}
	if v := os.Getenv("INTERNAL_REST_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			cfg.InternalRestPort = port
		}
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
}
