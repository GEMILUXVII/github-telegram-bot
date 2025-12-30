// Package config provides configuration management for the application.
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the application configuration.
type Config struct {
	Telegram TelegramConfig `mapstructure:"telegram"`
	GitHub   GitHubConfig   `mapstructure:"github"`
	Database DatabaseConfig `mapstructure:"database"`
	Server   ServerConfig   `mapstructure:"server"`
	Log      LogConfig      `mapstructure:"log"`
}

// TelegramConfig holds Telegram bot configuration.
type TelegramConfig struct {
	Token string `mapstructure:"token"`
	Debug bool   `mapstructure:"debug"`
}

// GitHubConfig holds GitHub API configuration.
type GitHubConfig struct {
	Token         string `mapstructure:"token"`
	WebhookSecret string `mapstructure:"webhook_secret"`
	Mode          string `mapstructure:"mode"`          // webhook, polling, or both
	PollInterval  int    `mapstructure:"poll_interval"` // Polling interval in seconds
}

// DatabaseConfig holds database configuration.
type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level string `mapstructure:"level"`
	File  string `mapstructure:"file"`
}

// Load reads configuration from file and environment variables.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("database.path", "./data/bot.db")
	v.SetDefault("log.level", "info")
	v.SetDefault("telegram.debug", false)
	v.SetDefault("github.mode", "polling")    // Default to polling for monitoring any repo
	v.SetDefault("github.poll_interval", 300) // 5 minutes default

	// Read config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./configs")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Read environment variables
	v.SetEnvPrefix("GHBOT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks if all required configuration fields are set.
func (c *Config) Validate() error {
	if c.Telegram.Token == "" {
		return fmt.Errorf("telegram token is required")
	}
	return nil
}

// ServerAddress returns the full server address.
func (c *Config) ServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
