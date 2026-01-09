package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	DefaultCacheDir = ".cache/rfc-mcp"
	DefaultCacheTTL = "1h"
	DefaultHost     = "localhost"
	DefaultPort     = 7991
	DefaultPath     = "/mcp"
)

type Config struct {
	Cache  CacheConfig  `yaml:"cache"`
	Server ServerConfig `yaml:"server"`
}

type CacheConfig struct {
	Dir string `yaml:"dir"`
	TTL string `yaml:"ttl"`
}

type ModeType string

var (
	ModeStdio ModeType = "stdio"
	ModeHTTP  ModeType = "http"
)

type ServerConfig struct {
	Mode ModeType `yaml:"mode"`
	Port int      `yaml:"port"`
	Host string   `yaml:"host"`
	Path string   `yaml:"path"`
}

func Default() *Config {
	return &Config{
		Cache: CacheConfig{
			Dir: DefaultCacheDir,
			TTL: DefaultCacheTTL,
		},
		Server: ServerConfig{
			Mode: ModeStdio,
			Host: DefaultHost,
			Port: DefaultPort,
			Path: DefaultPath,
		},
	}
}

func (c *Config) ApplyDefaults() {
	if c.Cache.Dir == "" {
		c.Cache.Dir = DefaultCacheDir
	}
	if c.Cache.TTL == "" {
		c.Cache.TTL = DefaultCacheTTL
	}

	if c.Server.Mode == "" {
		c.Server.Mode = ModeStdio
	}
	if c.Server.Host == "" {
		c.Server.Host = DefaultHost
	}
	if c.Server.Port == 0 {
		c.Server.Port = DefaultPort
	}
	if c.Server.Path == "" {
		c.Server.Path = DefaultPath
	}
}

func (s *ServerConfig) Valid() error {
	if s.Mode != ModeStdio && s.Mode != ModeHTTP {
		return fmt.Errorf("invalid server.mode in config: %q (expected 'stdio' or 'http')", s.Mode)
	}
	if s.Mode == ModeHTTP {
		if s.Port <= 0 || s.Port > 65535 {
			return fmt.Errorf("invalid server.port in config: %d (expected 1-65535)", s.Port)
		}
		if s.Host == "" {
			return errors.New("server.host cannot be empty in http mode")
		}
		if s.Path == "" {
			return errors.New("server.path cannot be empty in http mode")
		}
		// パスがスラッシュで始まっていることを確認
		if s.Path[0] != '/' {
			return fmt.Errorf("server.path must start with '/': %q", s.Path)
		}
	}
	return nil
}

func Load(path string) (*Config, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		// 設定ファイルが存在しない場合はデフォルト設定を返す
		return Default(), nil
	}

	conf := Default()
	if err := yaml.Unmarshal(buf, conf); err != nil {
		return nil, fmt.Errorf("failed to parse config file at %s: %w", path, err)
	}

	conf.ApplyDefaults()
	if err := conf.Server.Valid(); err != nil {
		return nil, fmt.Errorf("invalid server configuration in %s: %w", path, err)
	}

	return conf, nil
}
