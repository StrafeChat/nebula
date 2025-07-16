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
	MaxSpaceIconSize = 5 * 1024 * 1024 // 5MB
)

func UploadIcon(c fiber.Ctx) error {
	log.Printf("[UploadIcon] Received space icon upload request")

	// Get user ID from context (this should be set by auth middleware)
	userID := c.Locals("user_id").(string)
	if userID == "" {
		log.Printf("[UploadIcon] No user ID found in context")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}
	log.Printf("[UploadIcon] User ID: %s", userID)

	// Get space ID from URL params
	spaceID := c.Params("id")
	if spaceID == "" {
		log.Printf("[UploadIcon] No space ID provided")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Space ID is required",
		})
	}
	log.Printf("[UploadIcon] Space ID: %s", spaceID)

	// Get the uploaded file
	file, err := c.FormFile("icon")
	if err != nil {
		log.Printf("[UploadIcon] No file uploaded: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No file uploaded",
		})
	}
	log.Printf("[UploadIcon] File received: %s, Size: %d bytes", file.Filename, file.Size)

	// Validate file
	if validateErr := utils.ValidateAttachment(file, MaxSpaceIconSize); validateErr != nil {
		log.Printf("[UploadIcon] File validation failed: %v", validateErr)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": validateErr.Error(),
		})
	}

	// Check if it's an image
	contentType := file.Header.Get("Content-Type")
	if !utils.IsValidImageType(contentType) {
		log.Printf("[UploadIcon] Invalid image type: %s", contentType)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "File must be an image (PNG, JPEG, GIF, or WebP)",
		})
	}

	// Get image dimensions
	width, height, err := utils.GetImageDimensions(file)
	if err != nil {
		log.Printf("[UploadIcon] Failed to get image dimensions: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to process image",
		})
	}

	// Save file to disk
    storedFilename, err := utils.SaveFile(file, spaceID, "space_icons")
	if err != nil {
		log.Printf("[UploadIcon] Failed to save file: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save file",
		})
	}
	log.Printf("[UploadIcon] File saved as: %s", storedFilename)

	// Create file metadata
	fileID := uuid.New().String()
	metadata := types.FileMetadata{
		ID:        fileID,
		UserID:    userID,
		Type:      types.SpaceIcon,
		Filename:  file.Filename,
		MimeType:  contentType,
		Size:      file.Size,
		Width:     width,
		Height:    height,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	// Store file metadata in database
	if err := database.StoreFileMetadata(metadata, storedFilename); err != nil {
		log.Printf("[UploadIcon] Failed to store file metadata: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to store file metadata",
		})
	}

	// Construct the icon URL
    iconURL := fmt.Sprintf("/space_icons/%s/%s", spaceID, storedFilename)
	log.Printf("[UploadIcon] Icon URL: %s", iconURL)

	// Publish space icon update event to Redis
	spaceIconEvent := map[string]interface{}{
		"type":       "SPACE_ICON_UPDATE",
		"space_id":   spaceID,
		"icon_url":   iconURL,
		"file_id":    fileID,
		"auth_token": c.Get("x-session-token"),
		"created_at": time.Now().Unix(),
		"data": map[string]interface{}{
			"updated_by": userID,
			"file_metadata": map[string]interface{}{
				"width":  width,
				"height": height,
				"size":   file.Size,
			},
		},
	}

	eventBytes, err := json.Marshal(spaceIconEvent)
	if err != nil {
		log.Printf("[UploadIcon] Failed to marshal space icon event: %v", err)
		// Don't fail the request if event publishing fails
	} else {
		if err := database.Rdb.Publish(context.Background(), "SPACE_EVENTS", string(eventBytes)).Err(); err != nil {
			log.Printf("[UploadIcon] Failed to publish space icon event: %v", err)
			// Don't fail the request if event publishing fails
		} else {
			log.Printf("[UploadIcon] Successfully published space icon event")
		}
	}

	// Return success response
	return c.JSON(fiber.Map{
		"message":  "Space icon uploaded successfully",
		"file_id":  fileID,
		"filename": storedFilename,
		"icon_url": iconURL,
		"metadata": fiber.Map{
			"width":  width,
			"height": height,
			"size":   file.Size,
		},
	})
}