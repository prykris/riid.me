package main

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"

	customlogger "riid.me/pkg/logger"
	"riid.me/pkg/config"
	"riid.me/pkg/handlers"
	"riid.me/pkg/storage"
)

// healthCheck checks the status of the application and its dependencies (e.g., Redis).
func healthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status := map[string]interface{}{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	}

	// Check Redis connection using the global Rdb client from the storage package
	if storage.Rdb == nil {
		status["redis"] = map[string]string{
			"status": "error",
			"error":  "Redis client not initialized",
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	} else if err := storage.Rdb.Ping(ctx).Err(); err != nil {
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
	// 1. Initialize Logger
	customlogger.Init()

	// 2. Load Configuration
	config.LoadEnv() // This populates config.GlobalAppConfig

	// 3. Initialize Storage (Redis & SQLite)
	if err := storage.InitRedis(config.GlobalAppConfig); err != nil {
		customlogger.Fatal().Err(err).Msg("Failed to initialize Redis during startup")
	}
	if err := storage.InitSQLite(config.GlobalAppConfig); err != nil {
		customlogger.Fatal().Err(err).Msg("Failed to initialize SQLite during startup")
	}

	// 4. Initialize Short ID Service
	if err := handlers.InitShortIDService(); err != nil {
		customlogger.Fatal().Err(err).Msg("Failed to initialize ShortID service during startup")
	}

	// 5. Setup Router with request logging
	router := mux.NewRouter()

	// Log all incoming requests
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			customlogger.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("host", r.Host).
				Str("remote", r.RemoteAddr).
				Msg("Incoming request")
			next.ServeHTTP(w, r)
		})
	})

	// API routes - order is important (specific routes before catch-all)
	router.HandleFunc("/shorten", handlers.CreateShortURL).Methods("POST")
	router.HandleFunc("/api/stats/{shortcode}", handlers.GetLinkStatsHandler).Methods("GET")
	router.HandleFunc("/api/qr/{shortcode}", handlers.GenerateQRCodeHandler).Methods("GET")
	router.HandleFunc("/health", healthCheck).Methods("GET")
	router.HandleFunc("/validate-auth", handlers.ValidateAuthCodeHandler).Methods("POST").Name("validate-auth") // Named route for debugging

	// Add a test route for debugging routing
	router.HandleFunc("/test-route", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Test route is working!"))
	}).Methods("GET").Name("test-route")

	// Serve static files (e.g., index.html)
	// The path "./static/" is relative to where the binary is run.
	staticFileDirectory := http.Dir("./static/")
	// PathPrefix needs to end with a slash if it's matching a directory.
	// StripPrefix also needs to match that slash.
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(staticFileDirectory)))

	// Serve index.html at the root path "/"
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/index.html")
	}).Methods("GET")

	// IMPORTANT: Redirection for shortcodes must be the last route to act as a catch-all for root paths.
	router.HandleFunc("/{shortcode}", handlers.RedirectToLongURL).Methods("GET")

	// 6. Start Server
	portToUse := config.GlobalAppConfig.Port
	envPort := os.Getenv("PORT") // Allow direct PORT env var to override for deployment scenarios
	if envPort != "" {
		portToUse = envPort
	}

	customlogger.Info().Str("port", portToUse).Msgf("Server starting on :%s", portToUse)
	if err := http.ListenAndServe(":"+portToUse, router); err != nil {
		customlogger.Fatal().Err(err).Msg("Server failed to start")
	}
}