package utils

import (
	"regexp"
	"strings"
)

// IsValidEmojiName validates that an emoji name contains only alphanumeric characters and underscores
// and is between 2 and 32 characters long
func IsValidEmojiName(name string) bool {
	// Check length
	if len(name) < 2 || len(name) > 32 {
		return false
	}

	// Check for valid characters (alphanumeric and underscores only)
	validName := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	if !validName.MatchString(name) {
		return false
	}

	// Ensure it doesn't start or end with underscore
	if strings.HasPrefix(name, "_") || strings.HasSuffix(name, "_") {
		return false
	}

	// Ensure it doesn't have consecutive underscores
	if strings.Contains(name, "__") {
		return false
	}

	return true
}

// IsValidImageType checks if the content type is a valid image type for emojis
func IsValidImageType(contentType string) bool {
	validTypes := []string{
		"image/png",
		"image/jpeg",
		"image/jpg",
		"image/gif",
		"image/webp",
	}

	for _, validType := range validTypes {
		if contentType == validType {
			return true
		}
	}

	return false
}