package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	customlogger "riid.me/pkg/logger"
	"riid.me/pkg/models"
	"riid.me/pkg/storage"
)

// GetLinkStatsHandler retrieves and returns click statistics for a given shortcode.
// It queries the SQLite database for click details and aggregates them.
func GetLinkStatsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shortCode := vars["shortcode"]

	ctx := r.Context()

	rows, err := storage.StatsDB.QueryContext(ctx, "SELECT timestamp, user_agent, referrer FROM clicks WHERE short_code = ? ORDER BY timestamp DESC", shortCode)
	if err != nil {
		customlogger.Error().Err(err).Str("short_code", shortCode).Msg("Failed to query click statistics")
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"Failed to retrieve statistics"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var clicks []models.ClickDetail
	for rows.Next() {
		var cd models.ClickDetail
		// Scan into sql.NullString for UserAgent and Referrer to handle potential NULLs from DB.
		if err := rows.Scan(&cd.Timestamp, &cd.UserAgent, &cd.Referrer); err != nil {
			customlogger.Error().Err(err).Str("short_code", shortCode).Msg("Failed to scan click detail row")
			continue // Skipping problematic row
		}
		clicks = append(clicks, cd)
	}

	if err = rows.Err(); err != nil { // Check for errors during iteration
		customlogger.Error().Err(err).Str("short_code", shortCode).Msg("Error iterating click detail rows")
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"Failed to process statistics"}`, http.StatusInternalServerError)
		return
	}

	response := models.LinkStatsResponse{
		ShortCode:   shortCode,
		TotalClicks: len(clicks),
		Clicks:      clicks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
