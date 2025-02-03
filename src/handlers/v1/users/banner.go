package users

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const (
	maxBannerSize = 5 << 20 // 5MB
	bannerPath    = "uploads/banners"
)

// HandleBannerUpload handles the banner upload for a user
func HandleBannerUpload(c fiber.Ctx) error {
	// Get user from context (set by auth middleware)
	userID := c.Locals("user_id").(string)
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Create banners directory if it doesn't exist
	userBannerPath := filepath.Join(bannerPath, userID)
	if err := os.MkdirAll(userBannerPath, 0755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create directory",
		})
	}

	// Clean up old banner files
	entries, err := os.ReadDir(userBannerPath)
	if err == nil { // Only try to delete if we can read the directory
		for _, entry := range entries {
			if !entry.IsDir() { // Only delete files, not subdirectories
				oldFile := filepath.Join(userBannerPath, entry.Name())
				if err := os.Remove(oldFile); err != nil {
					// Log the error but continue with the upload
					fmt.Printf("Failed to delete old banner file %s: %v\n", oldFile, err)
				}
			}
		}
	}

	// Get the file from form
	file, err := c.FormFile("banner")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No file uploaded",
		})
	}

	if file.Size > maxBannerSize {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "File too large",
		})
	}

	// Generate unique filename
	ext := filepath.Ext(file.Filename)
	filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	filepath := filepath.Join(userBannerPath, filename)

	// Save the file
	if err := c.SaveFile(file, filepath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save file",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Banner uploaded successfully",
		"file": fiber.Map{
			"id":   filename,
			"name": file.Filename,
			"size": file.Size,
		},
	})
}