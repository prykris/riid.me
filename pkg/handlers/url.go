package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/teris-io/shortid"
	customlogger "riid.me/pkg/logger"
	"riid.me/pkg/models"
	"riid.me/pkg/config"
	"riid.me/pkg/storage"
)

var (
	// Sid is the global shortid generator instance.
	Sid *shortid.Shortid
)

// InitShortIDService initializes the shortid generator.
// It should be called once at application startup.
func InitShortIDService() error {
	generator, err := shortid.New(1, shortid.DefaultABC, 2342)
	if err != nil {
		customlogger.Error().Err(err).Msg("Failed to initialize shortid generator")
		return err
	}
	Sid = generator
	customlogger.Info().Msg("Shortid generator initialized")
	return nil
}

// NormalizeURL ensures a URL has a scheme (http or https).
// It defaults to https if no scheme is present.
func NormalizeURL(url string) string {
	url = strings.TrimSpace(url)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "https://" + url
	}
	return url
}

// CreateShortURL handles requests to shorten a long URL.
// It supports custom handles and expiration times if an appropriate auth code is provided.
func CreateShortURL(w http.ResponseWriter, r *http.Request) {
	var req models.URLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		customlogger.Error().Err(err).Msg("Invalid request body for CreateShortURL")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if req.LongURL == "" {
		customlogger.Error().Msg("Empty URL provided for CreateShortURL")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "URL is required"})
		return
	}

	normalizedURL := NormalizeURL(req.LongURL)
	var codeToUse string
	var err error

	redisExpirationDuration := time.Duration(config.DefaultExpirationDays) * 24 * time.Hour
	isValidAuthCodeForCustomFeature := false

	if req.CustomHandle != "" {
		if req.AuthCode == "" {
			customlogger.Info().Str("custom_handle", req.CustomHandle).Msg("Attempt to use custom handle without auth code")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Authorization code required for custom handle."})
			return
		}

		isValidAuthCode := false
		for _, validCode := range config.GlobalAppConfig.ValidAuthCodes {
			if req.AuthCode == validCode {
				isValidAuthCode = true
				break
			}
		}
		if !isValidAuthCode {
			customlogger.Info().Str("custom_handle", req.CustomHandle).Str("auth_code", req.AuthCode).Msg("Invalid auth code provided for custom handle")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid authorization code."})
			return
		}
		isValidAuthCodeForCustomFeature = true

		if len(req.CustomHandle) < 3 || len(req.CustomHandle) > 30 {
			customlogger.Error().Str("custom_handle", req.CustomHandle).Msg("Invalid custom handle length")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Custom handle must be between 3 and 30 characters."})
			return
		}

		ctx := r.Context()
		exists, errDb := storage.Rdb.Exists(ctx, req.CustomHandle).Result()
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

		if isValidAuthCodeForCustomFeature && req.ExpirationDays != nil {
			days := *req.ExpirationDays
			if days == config.NoExpirationValue {
				redisExpirationDuration = 0
				customlogger.Info().Str("code", codeToUse).Msg("Setting custom URL with no expiration")
			} else if days > 0 && days <= config.MaxExpirationDays {
				redisExpirationDuration = time.Duration(days) * 24 * time.Hour
				customlogger.Info().Str("code", codeToUse).Int("days", days).Msg("Setting custom URL with custom expiration")
			} else {
				customlogger.Error().Str("code", codeToUse).Int("days", days).Msg("Invalid expiration days provided")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Expiration must be 0 (for no expiry) or between 1 and %d days.", config.MaxExpirationDays)})
				return
			}
		}

	} else {
		codeToUse, err = Sid.Generate()
		if err != nil {
			customlogger.Error().Err(err).Msg("Failed to generate short code")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Error generating short code"})
			return
		}
	}

	ctx := r.Context()
	err = storage.Rdb.Set(ctx, codeToUse, normalizedURL, redisExpirationDuration).Err()
	if err != nil {
		customlogger.Error().Err(err).Str("code", codeToUse).Msg("Failed to store URL in Redis")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Error storing URL"})
		return
	}

	shortURL := fmt.Sprintf("%s://%s/%s", config.GlobalAppConfig.Scheme, config.GlobalAppConfig.Domain, codeToUse)
	customlogger.Info().Str("code", codeToUse).Str("long_url", normalizedURL).Str("short_url", shortURL).Msg("URL shortened successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.URLResponse{
		ShortURL: shortURL,
	})
}

// RedirectToLongURL handles requests to a shortcode, retrieves the original long URL,
// records the click, and redirects the user.
func RedirectToLongURL(w http.ResponseWriter, r *http.Request) {
	// Skip processing for known paths
	path := r.URL.Path
	if path == "/" || strings.HasPrefix(path, "/api/") || 
	   strings.HasPrefix(path, "/static/") || 
	   path == "/favicon.ico" ||
	   path == "/health" || 
	   path == "/test-route" ||
	   strings.HasSuffix(path, ".ico") ||
	   strings.HasSuffix(path, ".png") ||
	   strings.HasSuffix(path, ".jpg") ||
	   strings.HasSuffix(path, ".css") ||
	   strings.HasSuffix(path, ".js") {
		http.NotFound(w, r)
		return
	}

	// Extract code from URL path (remove leading slash)
	code := strings.TrimPrefix(path, "/")
	if code == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	ctx := r.Context()
	longURL, err := storage.Rdb.Get(ctx, code).Result()
	if err == redis.Nil {
		customlogger.Error().Str("code", code).Msg("Short URL not found for redirection")
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	} else if err != nil {
		customlogger.Error().Err(err).Str("code", code).Msg("Failed to retrieve URL from Redis for redirection")
		http.Error(w, "Error retrieving URL", http.StatusInternalServerError)
		return
	}

	userAgent := r.UserAgent()
	referrer := r.Referer()

	insertSQL := `INSERT INTO clicks (short_code, user_agent, referrer) VALUES (?, ?, ?)`
	_, errExec := storage.StatsDB.ExecContext(ctx, insertSQL, code, userAgent, referrer)
	if errExec != nil {
		customlogger.Error().Err(errExec).Str("short_code", code).Msg("Failed to record click event")
	} else {
		customlogger.Info().Str("short_code", code).Msg("Click event recorded")
	}

	customlogger.Info().Str("code", code).Str("long_url", longURL).Msg("Redirecting to long URL")
	http.Redirect(w, r, longURL, http.StatusMovedPermanently)
}
