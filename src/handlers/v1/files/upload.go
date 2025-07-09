package files

import (
	"time"

	"github.com/StrafeChat/nebula/src/database"
	"github.com/StrafeChat/nebula/src/types"
	"github.com/StrafeChat/nebula/src/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const (
	MaxAttachmentSize = 10 * 1024 * 1024 // 10MB
)

func UploadFile(c fiber.Ctx) error {
	// Get user ID from context (this should be set by auth middleware)
	userID := c.Locals("user_id").(string)
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Get the file from form-data
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No file uploaded",
		})
	}

	// Validate file
	if validateErr := utils.ValidateAttachment(file, MaxAttachmentSize); validateErr != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": validateErr.Error(),
		})
	}

	// Generate unique file ID
	fileID := uuid.New().String()

	// Extract image dimensions if it's an image
	width, height, err := utils.GetImageDimensions(file)
	if err != nil {
		// Log the error but don't fail the upload
		// Some images might not be decodable but still valid
		width, height = nil, nil
	}

	// Save the file with the file ID as the filename
	filename, err := utils.SaveFile(file, userID, "attachments")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save file",
		})
	}

	// Create file metadata
	metadata := types.FileMetadata{
		ID:        fileID,
		UserID:    userID,
		Type:      types.Upload,
		Filename:  file.Filename, // Original filename
		MimeType:  file.Header.Get("Content-Type"),
		Size:      file.Size,
		Width:     width,
		Height:    height,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	// Store file metadata in database
	if err := database.StoreFileMetadata(metadata, filename); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to store file metadata",
		})
	}

	// Return success response
	response := fiber.Map{
		"id":           fileID,
		"user_id":      userID,
		"filename":     file.Filename,
		"content_type": file.Header.Get("Content-Type"),
		"size":         file.Size,
	}

	// Add dimensions if available
	if width != nil && height != nil {
		response["width"] = *width
		response["height"] = *height
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

func GetFile(c fiber.Ctx) error {
	// Get file ID from URL params
	fileID := c.Params("id")
	if fileID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "File ID is required",
		})
	}

	// Get file metadata from database
	metadata, _, err := database.GetFileMetadata(fileID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "File not found",
		})
	}

	// Return file metadata
	response := fiber.Map{
		"id":           metadata.ID,
		"user_id":      metadata.UserID,
		"filename":     metadata.Filename,
		"content_type": metadata.MimeType,
		"size":         metadata.Size,
		"created_at":   metadata.CreatedAt,
	}

	// Add dimensions if available
	if metadata.Width != nil && metadata.Height != nil {
		response["width"] = *metadata.Width
		response["height"] = *metadata.Height
	}

	return c.Status(fiber.StatusOK).JSON(response)
}
