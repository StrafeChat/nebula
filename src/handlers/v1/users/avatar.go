package users

import (
	"fmt"
	"time"

	"github.com/StrafeChat/nebula/src/types"
	"github.com/StrafeChat/nebula/src/utils"
	"github.com/gofiber/fiber/v3"
)

const (
	MaxAvatarSize = 5 * 1024 * 1024 // 5MB
)

func UploadAvatar(c fiber.Ctx) error {
	// Get user ID from context (this should be set by auth middleware)
	userID := c.Locals("user_id").(string)
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Get the file from form-data
	file, err := c.FormFile("avatar")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No file uploaded",
		})
	}

	// Validate file
	if err := utils.ValidateFile(file, MaxAvatarSize); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Delete old avatar files
	if err := utils.DeleteOldFiles(userID, "avatars"); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete old avatar files",
		})
	}

	// Save the file
	filename, err := utils.SaveFile(file, userID, "avatars")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save file",
		})
	}

	// Create file metadata
	metadata := types.FileMetadata{
		ID:        filename,
		UserID:    userID,
		Type:      types.Avatar,
		Filename:  filename,
		MimeType:  file.Header.Get("Content-Type"),
		Size:      file.Size,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	// Return success response
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Avatar uploaded successfully",
		"file":    metadata,
		"url":     fmt.Sprintf("/avatars/%s/%s", userID, filename),
	})
}
