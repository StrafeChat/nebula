package emojis

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// GetTwemojiSVG serves Twemoji SVG files from local storage
func GetTwemojiSVG(c fiber.Ctx) error {
	unicode := c.Params("unicode")
	if unicode == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Unicode parameter is required",
		})
	}

	// Validate unicode format (should be hex codes separated by hyphens)
	if !isValidUnicodeFormat(unicode) {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid unicode format",
		})
	}

	// Construct local file path
	filePath := filepath.Join("static", "twemoji", unicode+".svg")

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return c.Status(404).JSON(fiber.Map{
			"error": "Emoji not found",
		})
	}

	// Set appropriate headers
	c.Set("Content-Type", "image/svg+xml")
	c.Set("Cache-Control", "public, max-age=31536000") // Cache for 1 year

	// Serve the local SVG file
	return c.SendFile(filePath)
}

// Helper function to validate unicode format
func isValidUnicodeFormat(unicode string) bool {
	// Should contain only hex characters and hyphens
	for _, char := range unicode {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F') || char == '-') {
			return false
		}
	}

	// Should not start or end with hyphen
	if strings.HasPrefix(unicode, "-") || strings.HasSuffix(unicode, "-") {
		return false
	}

	// Should not have consecutive hyphens
	if strings.Contains(unicode, "--") {
		return false
	}

	return len(unicode) > 0
}