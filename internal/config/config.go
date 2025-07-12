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
	Database     `yaml:"database"`
	Payment      `yaml:"payment"`
}

// GRPCServer holds gRPC server specific configuration.
type GRPCServer struct {
	Port    int `yaml:"port" env:"GRPC_SERVER_PORT" env-default:"50051"`
	WebPort int `yaml:"web_port" env:"GRPC_WEB_PORT" env-default:"50052"`
}

// URLShortener holds service-specific configuration.
type URLShortener struct {
	AliasLength int    `yaml:"alias_length" env:"ALIAS_LENGTH" env-default:"4"`
	BaseURL     string `yaml:"base_url" env:"BASE_URL" env-default:"http://localhost:8080"`
}

// Database holds database specific configuration.
type Database struct {
	Host            string `yaml:"host" env:"DATABASE_HOST" env-default:"localhost"`
	Port            int    `yaml:"port" env:"DATABASE_PORT" env-default:"5432"`
	User            string `yaml:"user" env:"DATABASE_USER" env-default:"postgres"`
	Password        string `yaml:"password" env:"DATABASE_PASSWORD" env-required:"true"`
	DBName          string `yaml:"dbname" env:"DATABASE_NAME" env-default:"gurls"`
	SSLMode         string `yaml:"sslmode" env:"DATABASE_SSLMODE" env-default:"disable"`
	Timezone        string `yaml:"timezone" env:"DATABASE_TIMEZONE" env-default:"UTC"`
	MaxIdleConns    int    `yaml:"max_idle_conns" env:"DATABASE_MAX_IDLE_CONNS" env-default:"10"`
	MaxOpenConns    int    `yaml:"max_open_conns" env:"DATABASE_MAX_OPEN_CONNS" env-default:"100"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime" env:"DATABASE_CONN_MAX_LIFETIME" env-default:"1h"`
	// Migration settings
	AutoMigrate bool `yaml:"auto_migrate" env:"DATABASE_AUTO_MIGRATE" env-default:"true"`
	SeedData    bool `yaml:"seed_data" env:"DATABASE_SEED_DATA" env-default:"true"`
}

// Payment holds payment service specific configuration.
type Payment struct {
	ShopID    string `yaml:"shop_id" env:"YOOKASSA_SHOP_ID" env-default:"test"`
	SecretKey string `yaml:"secret_key" env:"YOOKASSA_SECRET_KEY" env-default:"test"`
	APIURL    string `yaml:"api_url" env:"YOOKASSA_API_URL" env-default:"https://api.yookassa.ru/v3"`
	TestMode  bool   `yaml:"test_mode" env:"YOOKASSA_TEST_MODE" env-default:"true"`
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
