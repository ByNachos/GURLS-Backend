package config

import (
	"log"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

// Config holds all the configuration for the application.
type Config struct {
	Env          string `yaml:"env" env:"ENV" env-default:"production"`
	GRPCServer   `yaml:"grpc_server"`
	URLShortener `yaml:"url_shortener"`
}

// GRPCServer holds gRPC server specific configuration.
type GRPCServer struct {
	Port int `yaml:"port" env:"GRPC_SERVER_PORT" env-default:"50051"`
}

// URLShortener holds service-specific configuration.
type URLShortener struct {
	AliasLength int `yaml:"alias_length" env:"ALIAS_LENGTH" env-default:"4"`
}

// MustLoad loads the application configuration.
func MustLoad() *Config {
	// Try to load .env file (ignore error in production)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment variables")
	}

	var cfg Config

	// Check if config file path is specified
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config/local.yml" // default path
	}

	// Try to load config file
	if _, err := os.Stat(configPath); err == nil {
		if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
			log.Fatalf("cannot read config: %s", err)
		}
	} else {
		// If config file doesn't exist, use environment variables only
		log.Println("Config file not found, using environment variables only")
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			log.Fatalf("cannot read config from environment: %s", err)
		}
	}

	return &cfg
}
