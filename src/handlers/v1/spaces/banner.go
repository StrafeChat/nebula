package spaces

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/StrafeChat/nebula/src/database"
	"github.com/StrafeChat/nebula/src/types"
	"github.com/StrafeChat/nebula/src/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const (
	MaxSpaceBannerSize = 10 * 1024 * 1024 // 10MB
)

func UploadBanner(c fiber.Ctx) error {
	log.Printf("[UploadBanner] Received space banner upload request")

	// Get user ID from context (this should be set by auth middleware)
	userID := c.Locals("user_id").(string)
	if userID == "" {
		log.Printf("[UploadBanner] No user ID found in context")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}
	log.Printf("[UploadBanner] User ID: %s", userID)

	// Get space ID from URL params
	spaceID := c.Params("id")
	if spaceID == "" {
		log.Printf("[UploadBanner] No space ID provided")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Space ID is required",
		})
	}
	log.Printf("[UploadBanner] Space ID: %s", spaceID)

	// Get the file from form-data
	file, err := c.FormFile("file")
	if err != nil {
		log.Printf("[UploadBanner] No file uploaded: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No file uploaded",
		})
	}
	log.Printf("[UploadBanner] File received: %s, Size: %d bytes", file.Filename, file.Size)

	// Validate file
	if validateErr := utils.ValidateAttachment(file, MaxSpaceBannerSize); validateErr != nil {
		log.Printf("[UploadBanner] File validation failed: %v", validateErr)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": validateErr.Error(),
		})
	}

	// Validate file type (must be image)
	contentType := file.Header.Get("Content-Type")
	if len(contentType) < 6 || contentType[:6] != "image/" {
		log.Printf("[UploadBanner] Invalid file type: %s", contentType)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "File must be an image",
		})
	}

	// Save file to disk
    storedFilename, err := utils.SaveFile(file, spaceID, "space_banners")
	if err != nil {
		log.Printf("[UploadBanner] Failed to save file: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save file",
		})
	}
	log.Printf("[UploadBanner] File saved as: %s", storedFilename)

	// Get image dimensions
	width, height, err := utils.GetImageDimensions(file)
	if err != nil {
		log.Printf("[UploadBanner] Failed to get image dimensions: %v", err)
		// Continue without dimensions
	}

	// Create file metadata
	fileID := uuid.New().String()
	metadata := types.FileMetadata{
		ID:       fileID,
		UserID:   userID,
		Type:     types.SpaceBanner,
		Filename: file.Filename,
		MimeType: file.Header.Get("Content-Type"),
		Size:     file.Size,
		Width:    width,
		Height:   height,
	}

	// Store file metadata in database
	if err := database.StoreFileMetadata(metadata, storedFilename); err != nil {
		log.Printf("[UploadBanner] Failed to store file metadata: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to store file metadata",
		})
	}
	log.Printf("[UploadBanner] File metadata stored successfully")

	// Create banner URL
    bannerURL := fmt.Sprintf("/space_banners/%s/%s", spaceID, storedFilename)
	log.Printf("[UploadBanner] Banner URL: %s", bannerURL)

	// Publish space banner update event to Redis
	if err := publishSpaceBannerUpdate(spaceID, bannerURL, c.Get("X-Session-Token")); err != nil {
		log.Printf("[UploadBanner] Failed to publish space banner update event: %v", err)
		// Don't return error, as the file was uploaded successfully
	}

	// Return success response
    return c.Status(fiber.StatusOK).JSON(fiber.Map{
        "message":    "Space banner uploaded successfully",
        "file_id":    fileID,
        "filename":   storedFilename,
        "banner_url": bannerURL,
    })
}

func publishSpaceBannerUpdate(spaceID, bannerURL, authToken string) error {
	log.Printf("[publishSpaceBannerUpdate] Creating event payload for space %s", spaceID)
	// Create the event payload
	event := map[string]interface{}{
		"type":       "SPACE_BANNER_UPDATE",
		"space_id":   spaceID,
		"banner_url": bannerURL,
		"auth_token": authToken,
		"created_at": time.Now().Unix(),
	}

	// Marshal the event to JSON
	eventBytes, err := json.Marshal(event)
	if err != nil {
		log.Printf("[publishSpaceBannerUpdate] Failed to marshal event: %v", err)
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	log.Printf("[publishSpaceBannerUpdate] Event JSON: %s", string(eventBytes))

	// Publish to Redis SPACE_EVENTS channel
	ctx := context.Background()
	result := database.Rdb.Publish(ctx, "SPACE_EVENTS", string(eventBytes))
	if result.Err() != nil {
		log.Printf("[publishSpaceBannerUpdate] Failed to publish to Redis: %v", result.Err())
		return fmt.Errorf("failed to publish to Redis: %w", result.Err())
	}

	log.Printf("[publishSpaceBannerUpdate] Successfully published space banner update event")
	return nil
}