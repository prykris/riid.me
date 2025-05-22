package storage

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-redis/redis/v8"
	_ "modernc.org/sqlite" // SQLite driver
	customlogger "riid.me/pkg/logger"
	"riid.me/pkg/config"
)

var (
	// Rdb is the global Redis client instance.
	Rdb *redis.Client
	// StatsDB is the global SQLite database connection for statistics.
	StatsDB *sql.DB
)

// InitRedis initializes the connection to the Redis server using settings from AppConfig.
// It pings the server to ensure connectivity and stores the client in the global Rdb variable.
func InitRedis(cfg config.AppConfig) error {
	Rdb = redis.NewClient(&redis.Options{
		Addr:     cfg.RedisURL,
		Password: cfg.RedisPW,
		DB:       cfg.RedisDB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := Rdb.Ping(ctx).Err(); err != nil {
		customlogger.Error().Err(err).Msg("Failed to connect to Redis")
		return err
	}
	customlogger.Info().Msg("Connected to Redis successfully")
	return nil
}

// InitSQLite initializes the connection to the SQLite database using the path from AppConfig.
// It also ensures that the necessary 'clicks' table exists for storing statistics.
// The connection is stored in the global StatsDB variable.
func InitSQLite(cfg config.AppConfig) error {
	var err error
	StatsDB, err = sql.Open("sqlite", cfg.SQLiteDBPath) // Use "sqlite" for modernc.org/sqlite
	if err != nil {
		customlogger.Error().Err(err).Msgf("Failed to open SQLite database at %s", cfg.SQLiteDBPath)
		return err
	}

	if err = StatsDB.Ping(); err != nil {
		customlogger.Error().Err(err).Msg("Failed to ping SQLite database")
		return err
	}
	customlogger.Info().Msgf("Successfully connected to SQLite database at %s", cfg.SQLiteDBPath)

	// Create clicks table if it doesn't exist
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS clicks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		short_code TEXT NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		user_agent TEXT,
		referrer TEXT
	);`

	_, err = StatsDB.Exec(createTableSQL)
	if err != nil {
		customlogger.Error().Err(err).Msg("Failed to create clicks table in SQLite database")
		return err
	}
	customlogger.Info().Msg("Clicks table ensured in SQLite database")
	return nil
}
