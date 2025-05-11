package config

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the service
type Config struct {
	// GRPCServerConfig configuration
	GRPCServerConfig GRPCServerConfig `mapstructure:"grpc_server"`
	// LoggerConfig configuration
	LoggerConfig LoggerConfig `mapstructure:"log"`
	// SubPub configuration
	SubPubConfig SubPubConfig `mapstructure:"sub_pub"`
}

// GRPCServerConfig holds the configuration related to the gRPC server
type GRPCServerConfig struct {
	// GRPCPort is the port on which the gRPC server will listen
	Port int `mapstructure:"port"`
	// MaxMessageSize is the maximum size of a message in bytes
	MaxMessageSize int `mapstructure:"max_message_size"`
	// Timeout for shutdown
	Timeout time.Duration `mapstructure:"timeout"`
}

// LoggerConfig holds the configuration for the logger
type LoggerConfig struct {
	// Level is the minimum log level to output
	Level string `mapstructure:"level"`
	// Format is the format of the logs (e.g., "json", "text")
	Format string `mapstructure:"format"`
	// Output is where the logs are written to
	Output string `mapstructure:"output"`
}

// SubPubConfig holds configuration for the SubPub system.
type SubPubConfig struct {
	BufferSize int `mapstructure:"buffer_size"`
}

// New creates a new configuration with default values
func New() Config {
	return Config{
		GRPCServerConfig: GRPCServerConfig{
			Port:           8080,
			MaxMessageSize: 1024 * 1024, // 1MB
			Timeout:        5 * time.Second,
		},
		LoggerConfig: LoggerConfig{
			Level:  "info",
			Format: "text",
			Output: "stdout",
		},
	}
}

func LoadConfig() (Config, error) {
	var config Config
	var defaultConfig = New()

	// Config path
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	// Defaults
	viper.SetDefault("grpc_server.port", defaultConfig.GRPCServerConfig.Port)
	viper.SetDefault("grpc_server.max_message_size", defaultConfig.GRPCServerConfig.MaxMessageSize)
	viper.SetDefault("grpc_server.timeout", defaultConfig.GRPCServerConfig.Timeout)

	viper.SetDefault("log.level", defaultConfig.LoggerConfig.Level)
	viper.SetDefault("log.output", defaultConfig.LoggerConfig.Output)
	viper.SetDefault("log.format", defaultConfig.LoggerConfig.Format)

	if err := viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			log.Println("Config file not found; using defaults and environment variables.")
		}
	}

	viper.SetEnvPrefix("SUBPUB")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.Unmarshal(&config); err != nil {
		return defaultConfig, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return config, nil
}
