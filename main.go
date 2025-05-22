package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
	"github.com/teris-io/shortid"
	customlogger "riid.me/pkg/logger"
)

type URLRequest struct {
	LongURL         string `json:"long_url"`
	CustomHandle    string `json:"custom_handle,omitempty"`
	AuthCode        string `json:"auth_code,omitempty"`
	ExpirationDays  *int   `json:"expiration_days,omitempty"` // Pointer to distinguish 0 from not provided
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

type AuthValidationRequest struct {
	AuthCode string `json:"auth_code"`
}

type AuthValidationResponse struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message,omitempty"`
}

type ClickDetail struct {
	Timestamp string `json:"timestamp"`
	UserAgent string `json:"user_agent,omitempty"`
	Referrer  string `json:"referrer,omitempty"`
}

type LinkStatsResponse struct {
	ShortCode   string        `json:"short_code"`
	TotalClicks int           `json:"total_clicks"`
	Clicks      []ClickDetail `json:"clicks"`
}

var (
	rdb *redis.Client
	sid *shortid.Shortid
	statsDB *sql.DB
	config struct {
		Port           string
		Domain         string
		Scheme         string
		RedisURL       string
		RedisPW        string
		RedisDB        int
		ValidAuthCodes []string
	}
	validAuthCodes []string
)

const (
	defaultExpirationDays = 365      // 1 year
	maxExpirationDays     = 365 * 10 // 10 years
	noExpirationValue     = 0        // Represents 'never' or no expiry for Redis TTL
)

func init() {
	customlogger.Init()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		customlogger.Warn().Msg("Warning: .env file not found")
	}

	// Set config from environment
	config.Port = getEnv("PORT", "3000")
	config.Domain = getEnv("APP_DOMAIN", "localhost:3000")
	config.Scheme = getEnv("APP_SCHEME", "http")
	config.RedisURL = getEnv("REDIS_ADDR", "localhost:6379")
	config.RedisPW = getEnv("REDIS_PASSWORD", "")

	authCodesEnv := getEnv("VALID_AUTH_CODES", "")
	if authCodesEnv != "" {
		validAuthCodes = strings.Split(authCodesEnv, ",")
	} else {
		validAuthCodes = []string{}
		customlogger.Info().Msg("No VALID_AUTH_CODES configured. Custom handles via auth code will not be available.")
	}

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
		customlogger.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	customlogger.Info().Msg("Connected to Redis successfully")

	// Initialize shortid generator
	generator, err := shortid.New(1, shortid.DefaultABC, 2342)
	if err != nil {
		customlogger.Fatal().Err(err).Msg("Failed to initialize shortid generator")
	}
	sid = generator

	// Initialize SQLite Database for Statistics
	sqliteDBPath := os.Getenv("SQLITE_DB_PATH")
	if sqliteDBPath == "" {
		sqliteDBPath = "./riidme_stats.db" // Default path
		customlogger.Warn().Msgf("SQLITE_DB_PATH not set, defaulting to %s", sqliteDBPath)
	}

	var errDB error
	// statsDB, errDB = sql.Open("sqlite3", sqliteDBPath) // Old driver name
	statsDB, errDB = sql.Open("sqlite", sqliteDBPath) // Correct driver name for modernc.org/sqlite
	if errDB != nil {
		customlogger.Fatal().Err(errDB).Msg("Failed to open SQLite database for statistics")
	}
	// Ping to ensure connection is alive (optional, but good practice)
	if err := statsDB.Ping(); err != nil {
		customlogger.Fatal().Err(err).Msg("Failed to ping SQLite database")
	}

	customlogger.Info().Msgf("Successfully connected to SQLite database at %s", sqliteDBPath)

	// Create clicks table if it doesn't exist
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS clicks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		short_code TEXT NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		user_agent TEXT,
		referrer TEXT
	);`

	_, errDB = statsDB.Exec(createTableSQL)
	if errDB != nil {
		customlogger.Fatal().Err(errDB).Msg("Failed to create clicks table in SQLite database")
	}
	customlogger.Info().Msg("Clicks table ensured in SQLite database")
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
		customlogger.Error().Err(err).Msg("Invalid request body")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if req.LongURL == "" {
		customlogger.Error().Msg("Empty URL provided")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "URL is required"})
		return
	}

	// Normalize URL
	normalizedURL := normalizeURL(req.LongURL)
	var codeToUse string
	var err error

	// Default expiration
	redisExpirationDuration := time.Duration(defaultExpirationDays) * 24 * time.Hour
	isValidAuthCodeForCustomFeature := false // Flag to track if auth was successful for custom features

	if req.CustomHandle != "" {
		// 1. Check for AuthCode
		if req.AuthCode == "" {
			customlogger.Info().Str("custom_handle", req.CustomHandle).Msg("Attempt to use custom handle without auth code")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Authorization code required for custom handle."})
			return
		}

		// 2. Validate AuthCode
		isValidAuthCode := false
		for _, validCode := range validAuthCodes {
			if req.AuthCode == validCode {
				isValidAuthCode = true
				break
			}
		}
		if !isValidAuthCode {
			customlogger.Info().Str("custom_handle", req.CustomHandle).Str("auth_code", req.AuthCode).Msg("Invalid auth code provided")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid authorization code."})
			return
		}
		isValidAuthCodeForCustomFeature = true // Auth code is valid

		// 3. Validate CustomHandle (basic validation)
		if len(req.CustomHandle) < 3 || len(req.CustomHandle) > 30 { // Example: length 3-30
			customlogger.Error().Str("custom_handle", req.CustomHandle).Msg("Invalid custom handle length")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Custom handle must be between 3 and 30 characters."})
			return
		}
		// Add more validation for characters, reserved words etc. here if needed

		// 4. Check availability in Redis
		ctx := r.Context()
		exists, errDb := rdb.Exists(ctx, req.CustomHandle).Result()
		if errDb != nil {
			customlogger.Error().Err(errDb).Str("custom_handle", req.CustomHandle).Msg("Redis error checking custom handle availability")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Error checking custom handle availability."})
			return
		}
		if exists == 1 {
			customlogger.Info().Str("custom_handle", req.CustomHandle).Msg("Custom handle already taken")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Custom handle '%s' is already taken.", req.CustomHandle)})
			return
		}
		codeToUse = req.CustomHandle
		customlogger.Info().Str("custom_handle", codeToUse).Msg("Using user-provided custom handle")

		// 4. Process custom expiration if auth code was valid
		if isValidAuthCodeForCustomFeature && req.ExpirationDays != nil {
			days := *req.ExpirationDays
			if days == noExpirationValue { // 0 means never expire
				redisExpirationDuration = 0 // Redis TTL 0 means no expiry
				customlogger.Info().Str("code", codeToUse).Msg("Setting custom URL with no expiration")
			} else if days > 0 && days <= maxExpirationDays {
				redisExpirationDuration = time.Duration(days) * 24 * time.Hour
				customlogger.Info().Str("code", codeToUse).Int("days", days).Msg("Setting custom URL with custom expiration")
			} else {
				customlogger.Error().Str("code", codeToUse).Int("days", days).Msg("Invalid expiration days provided")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Expiration must be 0 (for no expiry) or between 1 and %d days.", maxExpirationDays)})
				return
			}
		}

	} else {
		// Generate short code if no custom handle is provided
		codeToUse, err = sid.Generate()
		if err != nil {
			customlogger.Error().Err(err).Msg("Failed to generate short code")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Error generating short code"})
			return
		}
	}

	// Store in Redis with determined expiration
	ctx := r.Context()
	err = rdb.Set(ctx, codeToUse, normalizedURL, redisExpirationDuration).Err()
	if err != nil {
		customlogger.Error().Err(err).Str("code", codeToUse).Msg("Failed to store URL in Redis")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Error storing URL"})
		return
	}

	shortURL := fmt.Sprintf("%s://%s/%s", config.Scheme, config.Domain, codeToUse)
	customlogger.Info().Str("code", codeToUse).Str("long_url", normalizedURL).Str("short_url", shortURL).Msg("URL shortened successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(URLResponse{
		ShortURL: shortURL,
	})
}

func redirectToLongURL(w http.ResponseWriter, r *http.Request) {
	customlogger.Debug().Msg("redirectToLongURL handler invoked") // Added for debugging
	vars := mux.Vars(r)
	code := vars["shortcode"]

	// Get from Redis
	ctx := r.Context()
	longURL, err := rdb.Get(ctx, code).Result()
	if err == redis.Nil {
		customlogger.Error().Str("code", code).Msg("Short URL not found")
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	} else if err != nil {
		customlogger.Error().Err(err).Str("code", code).Msg("Failed to retrieve URL from Redis")
		http.Error(w, "Error retrieving URL", http.StatusInternalServerError)
		return
	}

	// Record the click event before redirecting
	userAgent := r.UserAgent()
	referrer := r.Referer()

	insertSQL := `INSERT INTO clicks (short_code, user_agent, referrer) VALUES (?, ?, ?)`
	_, errExec := statsDB.ExecContext(ctx, insertSQL, code, userAgent, referrer)
	if errExec != nil {
		// Log the error, but don't block the redirect. 
		// Depending on requirements, you might want to handle this differently.
		customlogger.Error().Err(errExec).Str("short_code", code).Msg("Failed to record click event")
	} else {
		customlogger.Info().Str("short_code", code).Msg("Click event recorded")
	}

	customlogger.Info().Str("code", code).Str("long_url", longURL).Msg("Redirecting to long URL")
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

func validateAuthCodeHandler(w http.ResponseWriter, r *http.Request) {
	var req AuthValidationRequest
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		customlogger.Error().Err(err).Msg("Failed to decode request body for validateAuthCode")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AuthValidationResponse{Valid: false, Message: "Invalid request payload"})
		return
	}

	if req.AuthCode == "" {
		customlogger.Warn().Msg("Empty auth_code provided for validation")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AuthValidationResponse{Valid: false, Message: "Authorization code cannot be empty"})
		return
	}

	isValidCode := false // Renamed to avoid conflict with function name if any
	for _, validCode := range validAuthCodes { // Corrected: use global validAuthCodes
		if req.AuthCode == validCode {
			isValidCode = true
			break
		}
	}

	if isValidCode {
		customlogger.Info().Msg("Auth code validated successfully")
		json.NewEncoder(w).Encode(AuthValidationResponse{Valid: true})
	} else {
		customlogger.Warn().Str("auth_code_attempt", req.AuthCode).Msg("Invalid auth code provided for validation")
		json.NewEncoder(w).Encode(AuthValidationResponse{Valid: false, Message: "Invalid authorization code"})
	}
}

func getLinkStatsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shortCode := vars["shortcode"]

	ctx := r.Context()

	rows, err := statsDB.QueryContext(ctx, "SELECT timestamp, user_agent, referrer FROM clicks WHERE short_code = ? ORDER BY timestamp DESC", shortCode)
	if err != nil {
		customlogger.Error().Err(err).Str("short_code", shortCode).Msg("Failed to query click statistics")
		http.Error(w, `{"error":"Failed to retrieve statistics"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var clicks []ClickDetail
	for rows.Next() {
		var cd ClickDetail
		// Ensure to scan into variables that can handle NULLs from DB if applicable, or use sql.NullString etc.
		// For simplicity, assuming user_agent and referrer can be empty strings if NULL.
		var userAgent sql.NullString
		var referrer sql.NullString
		if err := rows.Scan(&cd.Timestamp, &userAgent, &referrer); err != nil {
			customlogger.Error().Err(err).Str("short_code", shortCode).Msg("Failed to scan click detail row")
			// Decide if you want to skip this row or fail the request
			continue // Skipping problematic row for now
		}
		cd.UserAgent = userAgent.String
		cd.Referrer = referrer.String
		clicks = append(clicks, cd)
	}

	if err = rows.Err(); err != nil { // Check for errors during iteration
		customlogger.Error().Err(err).Str("short_code", shortCode).Msg("Error iterating click detail rows")
		http.Error(w, `{"error":"Failed to process statistics"}`, http.StatusInternalServerError)
		return
	}

	response := LinkStatsResponse{
		ShortCode:   shortCode,
		TotalClicks: len(clicks),
		Clicks:      clicks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	router := mux.NewRouter()
	
	// API routes
	router.HandleFunc("/shorten", createShortURL).Methods("POST")
	router.HandleFunc("/validate-auth", validateAuthCodeHandler).Methods("POST")
	router.HandleFunc("/api/stats/{shortcode}", getLinkStatsHandler).Methods("GET")
	router.HandleFunc("/health", healthCheck).Methods("GET")
	router.HandleFunc("/{shortcode}", redirectToLongURL).Methods("GET")
	
	// Serve static files
	fs := http.FileServer(http.Dir("static"))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))
	router.PathPrefix("/").Handler(fs)
	
	port := os.Getenv("PORT")
	if port == "" {
		port = config.Port
	}
	customlogger.Info().Str("port", port).Msg("Server starting")
	customlogger.Fatal().Err(http.ListenAndServe(":"+port, router)).Msg("Server stopped")
}