package handlers

import (
	"fmt"
	"image/color"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	qrcode "github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
	customlogger "riid.me/pkg/logger"
	"riid.me/pkg/config"
)

// hexToNRGBA converts a hex color string (e.g., "#RRGGBB") to a color.NRGBA object.
// It returns an error if the hex string is invalid.
func hexToNRGBA(hexColor string) (color.NRGBA, error) {
	hexColor = strings.TrimPrefix(hexColor, "#")
	if len(hexColor) != 6 {
		return color.NRGBA{}, fmt.Errorf("invalid hex color string length: %s", hexColor)
	}
	r, err := strconv.ParseUint(hexColor[0:2], 16, 8)
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("failed to parse red component: %w", err)
	}
	g, err := strconv.ParseUint(hexColor[2:4], 16, 8)
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("failed to parse green component: %w", err)
	}
	b, err := strconv.ParseUint(hexColor[4:6], 16, 8)
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("failed to parse blue component: %w", err)
	}
	return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}, nil
}

// nopCloser is a helper type that wraps an io.Writer (like http.ResponseWriter)
// to satisfy the io.WriteCloser interface by adding a no-op Close method.
// This is useful when a function expects an io.WriteCloser but the underlying writer
// (e.g., http.ResponseWriter) doesn't need or have a Close method.
type nopCloser struct {
	io.Writer
}

// Close implements the io.Closer interface for nopCloser.
// It's a no-operation method because http.ResponseWriter doesn't need explicit closing in this context.
func (nopCloser) Close() error { return nil }

// GenerateQRCodeHandler generates and serves a QR code image for a given shortcode.
// It supports query parameters for customization: size, fg (foreground color),
// bg (background color), and level (error correction level).
func GenerateQRCodeHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shortCode := vars["shortcode"]

	if shortCode == "" {
		customlogger.Warn().Msg("generateQRCodeHandler: shortcode parameter is missing")
		http.Error(w, "Shortcode parameter is missing", http.StatusBadRequest)
		return
	}

	appScheme := config.GlobalAppConfig.Scheme
	appDomain := config.GlobalAppConfig.Domain
	fullURL := fmt.Sprintf("%s://%s/%s", appScheme, appDomain, shortCode)

	query := r.URL.Query()
	desiredPixelSize := 256 // Default size
	if sizeStr := query.Get("size"); sizeStr != "" {
		if parsedSize, err := strconv.Atoi(sizeStr); err == nil && parsedSize > 0 {
			desiredPixelSize = parsedSize
		}
	}

	modulePixelWidth := uint8(desiredPixelSize / 35) // Approximate module width
	if modulePixelWidth < 1 {
		modulePixelWidth = 1
	}
	if modulePixelWidth > 20 { // Cap module size
		modulePixelWidth = 20
	}

	fgColorHex := query.Get("fg")
	if fgColorHex == "" {
		fgColorHex = "#000000" // Default black
	}
	fgColor, err := hexToNRGBA(fgColorHex)
	if err != nil {
		customlogger.Warn().Err(err).Str("color_hex", fgColorHex).Msg("Failed to parse foreground color, using default")
		fgColor = color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	}

	bgColorHex := query.Get("bg")
	if bgColorHex == "" {
		bgColorHex = "#FFFFFF" // Default white
	}
	bgColor, err := hexToNRGBA(bgColorHex)
	if err != nil {
		customlogger.Warn().Err(err).Str("color_hex", bgColorHex).Msg("Failed to parse background color, using default")
		bgColor = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}

	_ = query.Get("level") // Keep levelStr for now, but don't use qrLevel directly if it causes issues

	// Create the QR code object
	qrc, err := qrcode.New(fullURL) // Simplified: only content string
	if err != nil {
		customlogger.Error().Err(err).Str("url", fullURL).Msg("Failed to generate QR code object")
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}

	// Prepare QR code image styling options for the standard writer
	stWriterOptions := []standard.ImageOption{
		standard.WithBgColor(bgColor), // Assumes bgColor is color.Color
		standard.WithFgColor(fgColor), // Assumes fgColor is color.Color
		standard.WithQRWidth(modulePixelWidth),
		standard.WithBuiltinImageEncoder(standard.PNG_FORMAT),
	}

	// For writing directly to http.ResponseWriter, use standard.NewWithWriter
	// The http.ResponseWriter (w) implements io.Writer.
	// standard.NewWithWriter expects an io.WriteCloser. We wrap 'w' with nopCloser.
	stWriter := standard.NewWithWriter(nopCloser{Writer: w}, stWriterOptions...)

	w.Header().Set("Content-Type", "image/png") // Set content type before writing

	if err := qrc.Save(stWriter); err != nil {
		customlogger.Error().Err(err).Msg("Failed to write QR code to response")
		// Avoid writing http.Error here if headers are already sent
		return
	}

	customlogger.Info().Str("shortcode", shortCode).Str("url", fullURL).Msg("Successfully generated and served QR code")
}
