package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	customlogger "riid.me/pkg/logger" // Assuming logger is already in pkg/logger
)

// AppConfig holds all configuration for the application.
// These values are typically loaded from environment variables.
type AppConfig struct {
	Port           string // Port the server will listen on (e.g., "3000")
	Domain         string // Domain name for constructing short URLs (e.g., "localhost:3000")
	Scheme         string // URL scheme (e.g., "http" or "https")
	RedisURL       string // Address of the Redis server (e.g., "localhost:6379")
	RedisPW        string // Password for the Redis server (empty if none)
	RedisDB        int    // Redis database number (typically 0)
	SQLiteDBPath   string // Filesystem path to the SQLite database file
	ValidAuthCodes []string // Slice of valid authorization codes for protected features
}

// GlobalAppConfig is a package-level variable that stores the loaded application configuration.
// It is populated by the LoadEnv function.
var GlobalAppConfig AppConfig

const (
	// DefaultExpirationDays is the default number of days a short URL will be valid if no custom expiration is set.
	DefaultExpirationDays = 365 // 1 year
	// MaxExpirationDays is the maximum allowed custom expiration period in days.
	MaxExpirationDays = 365 * 10 // 10 years
	// NoExpirationValue is used in requests to indicate that a URL should never expire.
	// For Redis, a TTL of 0 means no expiry.
	NoExpirationValue = 0
)

// getEnv retrieves an environment variable or returns a fallback value if not set.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// LoadEnv loads configuration from a .env file and environment variables into GlobalAppConfig.
// It should be called once at application startup.
func LoadEnv() {
	if err := godotenv.Load(); err != nil {
		customlogger.Warn().Msg("Warning: .env file not found, relying on environment variables")
	}

	GlobalAppConfig.Port = getEnv("PORT", "3000")
	GlobalAppConfig.Domain = getEnv("APP_DOMAIN", "localhost:3000")
	GlobalAppConfig.Scheme = getEnv("APP_SCHEME", "http")
	GlobalAppConfig.RedisURL = getEnv("REDIS_ADDR", "localhost:6379")
	GlobalAppConfig.RedisPW = getEnv("REDIS_PASSWORD", "")
	redisDBStr := getEnv("REDIS_DB", "0")
	redisDB, err := strconv.Atoi(redisDBStr)
	if err != nil {
		customlogger.Warn().Str("redis_db_str", redisDBStr).Msg("Invalid REDIS_DB value, defaulting to 0")
		GlobalAppConfig.RedisDB = 0
	} else {
		GlobalAppConfig.RedisDB = redisDB
	}

	GlobalAppConfig.SQLiteDBPath = getEnv("SQLITE_DB_PATH", "./riidme_stats.db")

	authCodesEnv := getEnv("VALID_AUTH_CODES", "")
	if authCodesEnv != "" {
		GlobalAppConfig.ValidAuthCodes = strings.Split(authCodesEnv, ",")
	} else {
		GlobalAppConfig.ValidAuthCodes = []string{}
		customlogger.Info().Msg("No VALID_AUTH_CODES configured. Custom handles via auth code will not be available.")
	}

	customlogger.Info().Msg("Application configuration loaded")
}
