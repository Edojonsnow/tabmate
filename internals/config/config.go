package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// DBConfig holds all database configuration
type DBConfig struct {
	Driver          string
	Source          string // DSN: "postgresql://user:password@host:port/dbname?sslmode=disable"
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// Config holds all application configuration
type Config struct {
	Database DBConfig
	HTTPPort string
	// Add other configurations like JWT secrets, AWS configs etc.
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (Config, error) {
	dbDriver := os.Getenv("DB_DRIVER")
	if dbDriver == "" {
		dbDriver = "postgres" // Default to postgres
	}

	dbSource := os.Getenv("DB_SOURCE")
	if dbSource == "" {
		return Config{}, fmt.Errorf("DB_SOURCE environment variable is required")
	}

	maxOpenConnsStr := os.Getenv("DB_MAX_OPEN_CONNS")
	maxOpenConns, _ := strconv.Atoi(maxOpenConnsStr) // Default to 0 (driver default) if not set or invalid
	if maxOpenConns == 0 {
		maxOpenConns = 25 // A sensible default
	}


	maxIdleConnsStr := os.Getenv("DB_MAX_IDLE_CONNS")
	maxIdleConns, _ := strconv.Atoi(maxIdleConnsStr) // Default to 0 (driver default) if not set or invalid
	if maxIdleConns == 0 {
		maxIdleConns = 25 // A sensible default
	}

	connMaxLifetimeStr := os.Getenv("DB_CONN_MAX_LIFETIME_MINUTES")
	connMaxLifetimeMinutes, _ := strconv.Atoi(connMaxLifetimeStr)
	connMaxLifetime := time.Duration(connMaxLifetimeMinutes) * time.Minute
	if connMaxLifetime == 0 {
		connMaxLifetime = time.Hour // A sensible default
	}

	connMaxIdleTimeStr := os.Getenv("DB_CONN_MAX_IDLE_TIME_MINUTES")
	connMaxIdleTimeMinutes, _ := strconv.Atoi(connMaxIdleTimeStr) // convert string to integer
	connMaxIdleTime := time.Duration(connMaxIdleTimeMinutes) * time.Minute
	if connMaxIdleTime == 0 {
		connMaxIdleTime = 5 * time.Minute // A sensible default
	}


	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = "8080" // Default HTTP port
	}

	return Config{
		Database: DBConfig{
			Driver:          dbDriver,
			Source:          dbSource,
			MaxOpenConns:    maxOpenConns,
			MaxIdleConns:    maxIdleConns,
			ConnMaxLifetime: connMaxLifetime,
			ConnMaxIdleTime: connMaxIdleTime,
		},
		HTTPPort: httpPort,
	}, nil
}