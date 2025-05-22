package handlers

import (
	"encoding/json"
	"net/http"

	customlogger "riid.me/pkg/logger"
	"riid.me/pkg/models"
	"riid.me/pkg/config"
)

// ValidateAuthCodeHandler handles requests to validate an authorization code.
// It checks the provided AuthCode against the list of valid codes loaded from configuration.
func ValidateAuthCodeHandler(w http.ResponseWriter, r *http.Request) {
	var req models.AuthValidationRequest
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		customlogger.Error().Err(err).Msg("Failed to decode request body for validateAuthCode")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.AuthValidationResponse{Valid: false, Message: "Invalid request payload"})
		return
	}

	if req.AuthCode == "" {
		customlogger.Warn().Msg("Empty auth_code provided for validation")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.AuthValidationResponse{Valid: false, Message: "Authorization code cannot be empty"})
		return
	}

	isValidCode := false
	for _, validCode := range config.GlobalAppConfig.ValidAuthCodes {
		if req.AuthCode == validCode {
			isValidCode = true
			break
		}
	}

	if isValidCode {
		customlogger.Info().Msg("Auth code validated successfully")
		json.NewEncoder(w).Encode(models.AuthValidationResponse{Valid: true})
	} else {
		customlogger.Warn().Str("auth_code_attempt", req.AuthCode).Msg("Invalid auth code provided for validation")
		json.NewEncoder(w).Encode(models.AuthValidationResponse{Valid: false, Message: "Invalid authorization code"})
	}
}
