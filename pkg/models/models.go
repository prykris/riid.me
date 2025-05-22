package models

import "database/sql"

// URLRequest is the structure for incoming URL shortening requests.
// It includes the original URL, an optional custom handle, an auth code for custom features,
// and optional expiration days.
// ExpirationDays is a pointer to distinguish between 0 (no expiry) and not provided (default expiry).
type URLRequest struct {
	LongURL         string `json:"long_url"`
	CustomHandle    string `json:"custom_handle,omitempty"`
	AuthCode        string `json:"auth_code,omitempty"`
	ExpirationDays  *int   `json:"expiration_days,omitempty"`
}

// URLResponse is the structure for the response after successfully shortening a URL.
// It contains the generated short URL.
// Example: {"short_url": "http://localhost:3000/abcdef"}
type URLResponse struct {
	ShortURL string `json:"short_url"`
}

// URLCheckRequest is used for checking if a custom handle is available.
// Currently not implemented as a separate endpoint but could be in the future.
type URLCheckRequest struct {
	URL string `json:"url"`
}

// URLCheckResponse indicates the availability of a custom handle.
// Example: {"available": true}
type URLCheckResponse struct {
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

// AuthValidationRequest is the structure for validating an authorization code.
// It contains the auth code to be validated.
type AuthValidationRequest struct {
	AuthCode string `json:"auth_code"`
}

// AuthValidationResponse indicates whether the provided auth code is valid.
// Example: {"valid": true} or {"valid": false, "message": "Invalid authorization code"}
type AuthValidationResponse struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message,omitempty"`
}

// ClickDetail stores information about a single click on a shortened URL.
// It includes the timestamp of the click, the user agent of the client,
// and the referrer URL if available.
type ClickDetail struct {
	Timestamp string         `json:"timestamp"`
	UserAgent sql.NullString `json:"user_agent,omitempty"` // Use sql.NullString for fields that can be NULL in DB
	Referrer  sql.NullString `json:"referrer,omitempty"`   // Use sql.NullString for fields that can be NULL in DB
}

// LinkStatsResponse is the structure for returning statistics for a shortened URL.
// It includes the short code, the total number of clicks, and a list of individual click details.
type LinkStatsResponse struct {
	ShortCode   string        `json:"short_code"`
	TotalClicks int           `json:"total_clicks"`
	Clicks      []ClickDetail `json:"clicks"`
}
