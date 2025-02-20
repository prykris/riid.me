package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/teris-io/shortid"
	"riid.me/pkg/logger"
)

type URLRequest struct {
	LongURL string `json:"long_url"`
}

type URLResponse struct {
	ShortURL string `json:"short_url"`
}

type URLCheckRequest struct {
	URL string `json:"url"`
}

type URLCheckResponse struct {
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

var (
	rdb *redis.Client
	sid *shortid.Shortid
	config struct {
		Port     string
		Domain   string
		Scheme   string
		RedisURL string
		RedisPW  string
		RedisDB  int
	}
)

func init() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		logger.Debug().Msg("Warning: .env file not found")
	}

	// Initialize logger
	logger.Init()

	// Set config from environment
	config.Port = getEnv("PORT", "3000")
	config.Domain = getEnv("APP_DOMAIN", "localhost:3000")
	config.Scheme = getEnv("APP_SCHEME", "http")
	config.RedisURL = getEnv("REDIS_ADDR", "localhost:6379")
	config.RedisPW = getEnv("REDIS_PASSWORD", "")

	// Initialize Redis
	rdb = redis.NewClient(&redis.Options{
		Addr:     config.RedisURL,
		Password: config.RedisPW,
		DB:       0,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	logger.Info().Msg("Connected to Redis successfully")

	// Initialize shortid generator
	generator, err := shortid.New(1, shortid.DefaultABC, 2342)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize shortid generator")
	}
	sid = generator
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func normalizeURL(url string) string {
	url = strings.TrimSpace(url)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "https://" + url
	}
	return url
}

func createShortURL(w http.ResponseWriter, r *http.Request) {
	var req URLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error().Err(err).Msg("Invalid request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.LongURL == "" {
		logger.Error().Msg("Empty URL provided")
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Normalize URL
	normalizedURL := normalizeURL(req.LongURL)

	// Generate short code
	code, err := sid.Generate()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to generate short code")
		http.Error(w, "Error generating short code", http.StatusInternalServerError)
		return
	}

	// Store in Redis with 1 year expiration
	ctx := r.Context()
	err = rdb.Set(ctx, code, normalizedURL, 365*24*time.Hour).Err()
	if err != nil {
		logger.Error().Err(err).Str("code", code).Msg("Failed to store URL in Redis")
		http.Error(w, "Error storing URL", http.StatusInternalServerError)
		return
	}

	shortURL := fmt.Sprintf("%s://%s/%s", config.Scheme, config.Domain, code)
	logger.Info().Str("code", code).Str("long_url", normalizedURL).Str("short_url", shortURL).Msg("URL shortened successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(URLResponse{
		ShortURL: shortURL,
	})
}

func redirectToLongURL(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	code := vars["shortcode"]

	// Get from Redis
	ctx := r.Context()
	longURL, err := rdb.Get(ctx, code).Result()
	if err == redis.Nil {
		logger.Error().Str("code", code).Msg("Short URL not found")
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	} else if err != nil {
		logger.Error().Err(err).Str("code", code).Msg("Failed to retrieve URL from Redis")
		http.Error(w, "Error retrieving URL", http.StatusInternalServerError)
		return
	}

	logger.Info().Str("code", code).Str("long_url", longURL).Msg("Redirecting to long URL")
	http.Redirect(w, r, longURL, http.StatusMovedPermanently)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status := map[string]interface{}{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	}

	// Check Redis
	if err := rdb.Ping(ctx).Err(); err != nil {
		status["redis"] = map[string]string{
			"status": "error",
			"error":  err.Error(),
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		status["redis"] = map[string]string{
			"status": "ok",
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func main() {
	router := mux.NewRouter()
	
	// API routes
	router.HandleFunc("/shorten", createShortURL).Methods("POST")
	router.HandleFunc("/health", healthCheck).Methods("GET")
	router.HandleFunc("/{shortcode}", redirectToLongURL).Methods("GET")
	
	// Serve static files
	fs := http.FileServer(http.Dir("static"))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))
	router.PathPrefix("/").Handler(fs)
	
	logger.Info().Str("port", config.Port).Msg("Server starting")
	logger.Fatal().Err(http.ListenAndServe(":"+config.Port, router)).Msg("Server stopped")
}