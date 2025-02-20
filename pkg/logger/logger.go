package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func Init() {
	zerolog.TimeFieldFormat = time.RFC3339
	
	// Pretty logging for development
	if os.Getenv("APP_ENV") != "production" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	}

	// Set global log level
	level := zerolog.InfoLevel
	if os.Getenv("LOG_LEVEL") != "" {
		level, _ = zerolog.ParseLevel(os.Getenv("LOG_LEVEL"))
	}
	zerolog.SetGlobalLevel(level)
}

func Error() *zerolog.Event {
	return log.Error()
}

func Info() *zerolog.Event {
	return log.Info()
}

func Debug() *zerolog.Event {
	return log.Debug()
}

func Fatal() *zerolog.Event {
	return log.Fatal()
} 