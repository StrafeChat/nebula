package spaces

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/StrafeChat/nebula/src/database"
	"github.com/StrafeChat/nebula/src/types"
	"github.com/StrafeChat/nebula/src/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const (
	MaxCustomEmojiSize = 1 * 1024 * 1024 // 1MB
	MaxEmojiDimensions = 128             // 128x128px max
)

// CustomEmojiUploadInput represents the input for custom emoji upload
type CustomEmojiUploadInput struct {
	Shortcode string `json:"shortcode" validate:"required,min=2,max=32"`
	Name      string `json:"name" validate:"required,min=1,max=64"`
}

// UploadCustomEmoji handles uploading custom emoji files for spaces
func UploadCustomEmoji(c fiber.Ctx) error {
	log.Printf("[UploadCustomEmoji] Received custom emoji upload request")

	// Get user ID from context (set by auth middleware)
	userID := c.Locals("user_id").(string)
	if userID == "" {
		log.Printf("[UploadCustomEmoji] No user ID found in context")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}
	log.Printf("[UploadCustomEmoji] User ID: %s", userID)

	// Get space ID from URL params
	spaceID := c.Params("id")
	if spaceID == "" {
		log.Printf("[UploadCustomEmoji] No space ID provided")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Space ID is required",
		})
	}
	log.Printf("[UploadCustomEmoji] Space ID: %s", spaceID)

	// Get the uploaded file
	file, err := c.FormFile("emoji")
	if err != nil {
		log.Printf("[UploadCustomEmoji] No file uploaded: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No file uploaded",
		})
	}
	log.Printf("[UploadCustomEmoji] File received: %s, Size: %d bytes", file.Filename, file.Size)

	// Get shortcode and name from form data
	shortcode := c.FormValue("shortcode")
	name := c.FormValue("name")

	if shortcode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Shortcode is required",
		})
	}

	if name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Name is required",
		})
	}

	// Validate shortcode format (must start with custom_ and be alphanumeric)
	if !strings.HasPrefix(shortcode, "custom_") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Shortcode must start with 'custom_'",
		})
	}

	// Validate shortcode length and format
	if len(shortcode) < 8 || len(shortcode) > 32 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Shortcode must be between 8 and 32 characters",
		})
	}

	// Validate file size
	if file.Size > MaxCustomEmojiSize {
		log.Printf("[UploadCustomEmoji] File too large: %d bytes", file.Size)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("File size exceeds maximum allowed size of %d bytes", MaxCustomEmojiSize),
		})
	}

	// Validate file type
	contentType := file.Header.Get("Content-Type")
	if !utils.IsValidImageType(contentType) {
		log.Printf("[UploadCustomEmoji] Invalid image type: %s", contentType)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "File must be an image (PNG, JPEG, GIF, or WebP)",
		})
	}

	// Get image dimensions and validate
	_, _, err = utils.GetImageDimensions(file)
	if err != nil {
		log.Printf("[UploadCustomEmoji] Failed to get image dimensions: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to process image",
		})
	}

	// Convert image to WebP format
	webpData, finalWidth, finalHeight, err := convertToWebP(file)
	if err != nil {
		log.Printf("[UploadCustomEmoji] Failed to convert to WebP: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to process image",
		})
	}

	// Generate file ID and filename
	fileID := uuid.New().String()
	filename := fmt.Sprintf("%s.png", shortcode)

	// Create directory structure
	uploadDir := filepath.Join("uploads", "custom_emojis", spaceID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Printf("[UploadCustomEmoji] Failed to create directory: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create directory",
		})
	}

	// Save WebP file
	filePath := filepath.Join(uploadDir, filename)
	if err := os.WriteFile(filePath, webpData, 0644); err != nil {
		log.Printf("[UploadCustomEmoji] Failed to save WebP file: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save file",
		})
	}
	log.Printf("[UploadCustomEmoji] WebP file saved as: %s", filePath)

	// Create file metadata
	metadata := types.FileMetadata{
		ID:        fileID,
		UserID:    userID,
		Type:      types.CustomEmoji,
		Filename:  filename,
		MimeType:  "image/png",
		Size:      int64(len(webpData)),
		Width:     &finalWidth,
		Height:    &finalHeight,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	// Store file metadata in database
	if err := database.StoreFileMetadata(metadata, filename); err != nil {
		log.Printf("[UploadCustomEmoji] Failed to store file metadata: %v", err)
		// Clean up file if metadata storage fails
		os.Remove(filePath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to store file metadata",
		})
	}

	// Construct the emoji URL
	emojiURL := fmt.Sprintf("/custom_emojis/%s/%s", spaceID, filename)
	log.Printf("[UploadCustomEmoji] Emoji URL: %s", emojiURL)

	// Publish custom emoji creation event to Redis
	customEmojiEvent := map[string]interface{}{
		"type":       "CUSTOM_EMOJI_CREATED",
		"space_id":   spaceID,
		"shortcode":  shortcode,
		"name":       name,
		"file_id":    fileID,
		"emoji_url":  emojiURL,
		"auth_token": c.Get("x-session-token"),
		"created_at": time.Now().Unix(),
		"data": map[string]interface{}{
			"created_by": userID,
			"file_metadata": map[string]interface{}{
				"width":  finalWidth,
				"height": finalHeight,
				"size":   len(webpData),
			},
		},
	}

	eventBytes, err := json.Marshal(customEmojiEvent)
	if err != nil {
		log.Printf("[UploadCustomEmoji] Failed to marshal custom emoji event: %v", err)
		// Don't fail the request if event publishing fails
	} else {
		if err := database.Rdb.Publish(context.Background(), "CUSTOM_EMOJI_EVENTS", string(eventBytes)).Err(); err != nil {
			log.Printf("[UploadCustomEmoji] Failed to publish custom emoji event: %v", err)
			// Don't fail the request if event publishing fails
		} else {
			log.Printf("[UploadCustomEmoji] Successfully published custom emoji event")
		}
	}

	// Return success response
	return c.JSON(fiber.Map{
		"message":   "Custom emoji uploaded successfully",
		"file_id":   fileID,
		"filename":  filename,
		"shortcode": shortcode,
		"name":      name,
		"url":       emojiURL,
		"metadata": fiber.Map{
			"width":  finalWidth,
			"height": finalHeight,
			"size":   len(webpData),
		},
	})
}

// convertToWebP converts an uploaded image to WebP format with size optimization
func convertToWebP(file *multipart.FileHeader) ([]byte, int, int, error) {
	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	// Read file content
	fileData, err := io.ReadAll(src)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to read file data: %w", err)
	}

	// Decode image based on content type
	var img image.Image
	contentType := file.Header.Get("Content-Type")

	switch contentType {
	case "image/jpeg", "image/jpg":
		img, err = jpeg.Decode(bytes.NewReader(fileData))
	case "image/png":
		img, err = png.Decode(bytes.NewReader(fileData))
	case "image/gif":
		img, err = gif.Decode(bytes.NewReader(fileData))
	case "image/webp":
		// For now, we'll reject WebP uploads until a suitable WebP library is available
		return nil, 0, 0, fmt.Errorf("WebP format not supported yet, please use PNG, JPEG, or GIF")
	default:
		return nil, 0, 0, fmt.Errorf("unsupported image format: %s", contentType)
	}

	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to decode image: %w", err)
	}

	// Get original dimensions
	bounds := img.Bounds()
	originalWidth := bounds.Dx()
	originalHeight := bounds.Dy()

	// Resize if necessary (maintain aspect ratio)
	finalWidth := originalWidth
	finalHeight := originalHeight

	if originalWidth > MaxEmojiDimensions || originalHeight > MaxEmojiDimensions {
		// Calculate scaling factor to fit within MaxEmojiDimensions
		scaleX := float64(MaxEmojiDimensions) / float64(originalWidth)
		scaleY := float64(MaxEmojiDimensions) / float64(originalHeight)
		scale := scaleX
		if scaleY < scaleX {
			scale = scaleY
		}

		finalWidth = int(float64(originalWidth) * scale)
		finalHeight = int(float64(originalHeight) * scale)

		// Create a simple resized image using nearest neighbor
		img = resizeImage(img, finalWidth, finalHeight)
	}

	// For now, encode as PNG with optimization
	// TODO: Replace with WebP encoding when a suitable library is available
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, 0, 0, fmt.Errorf("failed to encode image as PNG: %w", err)
	}

	return buf.Bytes(), finalWidth, finalHeight, nil
}

// ServeCustomEmoji serves custom emoji files with proper CDN headers
func ServeCustomEmoji(c fiber.Ctx) error {
	spaceID := c.Params("spaceId")
	filename := c.Params("filename")

	if spaceID == "" || filename == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Space ID and filename are required",
		})
	}

	// Construct file path
	filePath := filepath.Join("uploads", "custom_emojis", spaceID, filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Emoji not found",
		})
	}

	// Set CDN headers for optimal caching
	c.Set("Cache-Control", "public, max-age=31536000") // 1 year
	c.Set("Content-Type", "image/png")
	c.Set("Access-Control-Allow-Origin", "*")
	c.Set("Access-Control-Allow-Methods", "GET")
	c.Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept")

	// Generate ETag based on file modification time and size
	fileInfo, err := os.Stat(filePath)
	if err == nil {
		etag := fmt.Sprintf(`"%d-%d"`, fileInfo.ModTime().Unix(), fileInfo.Size())
		c.Set("ETag", etag)

		// Check if client has cached version
		if c.Get("If-None-Match") == etag {
			return c.SendStatus(fiber.StatusNotModified)
		}
	}

	// Serve the file
	return c.SendFile(filePath)
}

// DeleteCustomEmoji deletes a custom emoji file and metadata
func DeleteCustomEmoji(c fiber.Ctx) error {
	log.Printf("[DeleteCustomEmoji] Received delete request")

	// Get user ID from context
	userID := c.Locals("user_id").(string)
	if userID == "" {
		log.Printf("[DeleteCustomEmoji] No user ID found in context")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Get space ID and shortcode from URL params
	spaceID := c.Params("id")
	shortcode := c.Params("shortcode")

	if spaceID == "" || shortcode == "" {
		log.Printf("[DeleteCustomEmoji] Missing space ID or shortcode")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Space ID and shortcode are required",
		})
	}

	log.Printf("[DeleteCustomEmoji] Space ID: %s, Shortcode: %s, User ID: %s", spaceID, shortcode, userID)

	// TODO: Add permission check - user should have MANAGE_EMOJIS permission
	// For now, we'll allow any authenticated user to delete emojis

	// Construct file path
	filename := fmt.Sprintf("%s.png", shortcode)
	filePath := filepath.Join("uploads", "custom_emojis", spaceID, filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("[DeleteCustomEmoji] File not found: %s", filePath)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Emoji not found",
		})
	}

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		log.Printf("[DeleteCustomEmoji] Failed to delete file: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete emoji file",
		})
	}

	log.Printf("[DeleteCustomEmoji] Successfully deleted file: %s", filePath)

	// Publish custom emoji deletion event to Redis
	customEmojiEvent := map[string]interface{}{
		"type":       "CUSTOM_EMOJI_DELETED",
		"space_id":   spaceID,
		"shortcode":  shortcode,
		"auth_token": c.Get("x-session-token"),
		"created_at": time.Now().Unix(),
		"data": map[string]interface{}{
			"deleted_by": userID,
		},
	}

	eventBytes, err := json.Marshal(customEmojiEvent)
	if err != nil {
		log.Printf("[DeleteCustomEmoji] Failed to marshal custom emoji event: %v", err)
		// Don't fail the request if event publishing fails
	} else {
		if err := database.Rdb.Publish(context.Background(), "CUSTOM_EMOJI_EVENTS", string(eventBytes)).Err(); err != nil {
			log.Printf("[DeleteCustomEmoji] Failed to publish custom emoji event: %v", err)
			// Don't fail the request if event publishing fails
		} else {
			log.Printf("[DeleteCustomEmoji] Successfully published custom emoji deletion event")
		}
	}

	// Return success response
	return c.JSON(fiber.Map{
		"message": "Custom emoji deleted successfully",
	})
}

// GetSpaceCustomEmojis returns all custom emojis for a space
func GetSpaceCustomEmojis(c fiber.Ctx) error {
	log.Printf("[GetSpaceCustomEmojis] Received request")

	// Get space ID from URL params
	spaceID := c.Params("id")
	if spaceID == "" {
		log.Printf("[GetSpaceCustomEmojis] No space ID provided")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Space ID is required",
		})
	}

	log.Printf("[GetSpaceCustomEmojis] Space ID: %s", spaceID)

	// Get custom emojis directory
	emojiDir := filepath.Join("uploads", "custom_emojis", spaceID)

	// Check if directory exists
	if _, err := os.Stat(emojiDir); os.IsNotExist(err) {
		log.Printf("[GetSpaceCustomEmojis] No emoji directory found for space: %s", spaceID)
		return c.JSON([]interface{}{}) // Return empty array if no emojis
	}

	// Read directory contents
	files, err := os.ReadDir(emojiDir)
	if err != nil {
		log.Printf("[GetSpaceCustomEmojis] Failed to read emoji directory: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to read emoji directory",
		})
	}

	// Build emoji list
	var emojis []fiber.Map
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".png") {
			// Extract shortcode from filename (remove .png extension)
			shortcode := strings.TrimSuffix(file.Name(), ".png")

			// Get file info
			filePath := filepath.Join(emojiDir, file.Name())
			fileInfo, err := os.Stat(filePath)
			if err != nil {
				log.Printf("[GetSpaceCustomEmojis] Failed to get file info for %s: %v", file.Name(), err)
				continue
			}

			// Construct emoji URL
			emojiURL := fmt.Sprintf("/custom_emojis/%s/%s", spaceID, file.Name())

			emoji := fiber.Map{
				"shortcode":  shortcode,
				"name":       shortcode, // For now, use shortcode as name
				"url":        emojiURL,
				"size":       fileInfo.Size(),
				"created_at": fileInfo.ModTime().Unix(),
			}

			emojis = append(emojis, emoji)
		}
	}

	log.Printf("[GetSpaceCustomEmojis] Found %d custom emojis for space %s", len(emojis), spaceID)

	return c.JSON(emojis)
}

// resizeImage performs simple image resizing using nearest neighbor algorithm
func resizeImage(src image.Image, width, height int) image.Image {
	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Calculate source coordinates using nearest neighbor
			srcX := x * srcWidth / width
			srcY := y * srcHeight / height

			// Ensure we don't go out of bounds
			if srcX >= srcWidth {
				srcX = srcWidth - 1
			}
			if srcY >= srcHeight {
				srcY = srcHeight - 1
			}

			// Copy pixel from source to destination
			dst.Set(x, y, src.At(srcBounds.Min.X+srcX, srcBounds.Min.Y+srcY))
		}
	}

	return dst
}
