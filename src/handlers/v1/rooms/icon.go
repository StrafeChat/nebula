package rooms

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
)

const (
	MaxIconSize = 5 * 1024 * 1024 // 5MB
)

func UploadIcon(c fiber.Ctx) error {
	log.Printf("[UploadIcon] Received room icon upload request")

	// Get user ID from context (this should be set by auth middleware)
	userID := c.Locals("user_id").(string)
	if userID == "" {
		log.Printf("[UploadIcon] No user ID found in context")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}
	log.Printf("[UploadIcon] User ID: %s", userID)

	// Get room ID from URL params
	roomID := c.Params("id")
	if roomID == "" {
		log.Printf("[UploadIcon] No room ID provided")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Room ID is required",
		})
	}
	log.Printf("[UploadIcon] Room ID: %s", roomID)

	// Get the file from form-data
	file, err := c.FormFile("icon")
	if err != nil {
		log.Printf("[UploadIcon] Error getting file from form: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No file uploaded",
		})
	}
	log.Printf("[UploadIcon] File received: %s, size: %d", file.Filename, file.Size)

	// Validate file
	if err := utils.ValidateFile(file, MaxIconSize); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Delete old icon files
	if err := utils.DeleteOldFiles(roomID, "icons"); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete old icon files",
		})
	}

	// Save the file
	filename, err := utils.SaveFile(file, roomID, "icons")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save file",
		})
	}

	// Create file metadata
	metadata := types.FileMetadata{
		ID:        filename,
		UserID:    userID,
		Type:      types.RoomIcon,
		Filename:  filename,
		MimeType:  file.Header.Get("Content-Type"),
		Size:      file.Size,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	// Update room icon in Equinox via Redis pub/sub
	// Only pass the filename, not the full path, since the client constructs the full URL
	log.Printf("[UploadIcon] Publishing Redis event for room %s with icon %s", roomID, filename)
	if err := publishRoomIconUpdate(roomID, filename, c.Get("X-Session-Token")); err != nil {
		// Log the error but don't fail the upload since the file was saved successfully
		log.Printf("[UploadIcon] Failed to publish Redis event: %v", err)
		fmt.Printf("Warning: Failed to publish room icon update event: %v\n", err)
	} else {
		log.Printf("[UploadIcon] Successfully published Redis event")
	}

	// Return success response
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Room icon uploaded successfully",
		"file":    metadata,
		"url":     filename,
	})
}

// publishRoomIconUpdate publishes a room icon update event to Redis
func publishRoomIconUpdate(roomID, iconURL, authToken string) error {
	log.Printf("[publishRoomIconUpdate] Creating event payload for room %s", roomID)
	// Create the event payload
	event := map[string]interface{}{
		"type":       "ROOM_ICON_UPDATE",
		"room_id":    roomID,
		"icon_url":   iconURL,
		"auth_token": authToken,
		"created_at": time.Now().Unix(),
	}

	// Marshal the event to JSON
	eventBytes, err := json.Marshal(event)
	if err != nil {
		log.Printf("[publishRoomIconUpdate] Failed to marshal event: %v", err)
		return fmt.Errorf("failed to marshal event: %v", err)
	}
	log.Printf("[publishRoomIconUpdate] Event JSON: %s", string(eventBytes))

	// Publish to Redis ROOM_EVENTS channel
	ctx := context.Background()
	log.Printf("[publishRoomIconUpdate] Publishing to Redis ROOM_EVENTS channel")
	if err := database.Rdb.Publish(ctx, "ROOM_EVENTS", string(eventBytes)).Err(); err != nil {
		log.Printf("[publishRoomIconUpdate] Failed to publish to Redis: %v", err)
		return fmt.Errorf("failed to publish to Redis: %v", err)
	}
	log.Printf("[publishRoomIconUpdate] Successfully published to Redis")

	fmt.Printf("Published room icon update event for room %s\n", roomID)
	return nil
}
