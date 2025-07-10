package emojis

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/StrafeChat/nebula/src/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// CustomEmoji represents a custom emoji file
type CustomEmoji struct {
	ID        string    `json:"id"`
	SpaceID   string    `json:"space_id"`
	Name      string    `json:"name"`
	Filename  string    `json:"filename"`
	URL       string    `json:"url"`
	UploadedBy string   `json:"uploaded_by"`
	CreatedAt time.Time `json:"created_at"`
}

// UploadCustomEmoji handles uploading custom emojis for spaces
func UploadCustomEmoji(c fiber.Ctx) error {
	spaceID := c.Params("spaceId")
	if spaceID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Space ID is required",
		})
	}

	// Get user from context (set by auth middleware)
	userID := c.Locals("user_id").(string)

	// Check if user has permission to upload emojis to this space
	// For now, we'll assume any member can upload emojis
	// In the future, this should check for specific permissions

	// Get the uploaded file
	file, err := c.FormFile("emoji")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "No file uploaded",
		})
	}

	// Get emoji name from form
	emojiName := c.FormValue("name")
	if emojiName == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Emoji name is required",
		})
	}

	// Validate emoji name (alphanumeric and underscores only)
	if !utils.IsValidEmojiName(emojiName) {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid emoji name. Use only letters, numbers, and underscores",
		})
	}

	// Validate file type (only images)
	if !strings.HasPrefix(file.Header.Get("Content-Type"), "image/") {
		return c.Status(400).JSON(fiber.Map{
			"error": "Only image files are allowed",
		})
	}

	// Validate file size (max 1MB)
	if file.Size > 1024*1024 {
		return c.Status(400).JSON(fiber.Map{
			"error": "File size must be less than 1MB",
		})
	}

	// Check if emoji name already exists in this space
	emojiDir := filepath.Join("uploads", "emojis", spaceID)
	if exists := checkEmojiNameExists(emojiDir, emojiName); exists {
		return c.Status(400).JSON(fiber.Map{
			"error": "An emoji with this name already exists in this space",
		})
	}

	// Generate unique filename
	emojiID := uuid.New().String()
	fileExt := filepath.Ext(file.Filename)
	filename := fmt.Sprintf("%s%s", emojiID, fileExt)

	// Create space emojis directory if it doesn't exist
	if err := os.MkdirAll(emojiDir, 0755); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to create emoji directory",
		})
	}

	// Save file
	filePath := filepath.Join(emojiDir, filename)
	if err := saveUploadedFile(file, filePath); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to save emoji file",
		})
	}

	// Create emoji metadata
	emoji := CustomEmoji{
		ID:        emojiID,
		SpaceID:   spaceID,
		Name:      emojiName,
		Filename:  filename,
		URL:       fmt.Sprintf("/uploads/emojis/%s/%s", spaceID, filename),
		UploadedBy: userID,
		CreatedAt: time.Now(),
	}

	// Save emoji metadata to JSON file
	metadataPath := filepath.Join(emojiDir, emojiID+".json")
	if err := saveEmojiMetadata(emoji, metadataPath); err != nil {
		// Clean up file if metadata save fails
		os.Remove(filePath)
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to save emoji metadata",
		})
	}

	return c.JSON(emoji)
}

// GetSpaceEmojis returns all custom emojis for a space
func GetSpaceEmojis(c fiber.Ctx) error {
	spaceID := c.Params("spaceId")
	if spaceID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Space ID is required",
		})
	}

	emojiDir := filepath.Join("uploads", "emojis", spaceID)
	emojis, err := loadSpaceEmojis(emojiDir)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to fetch emojis",
		})
	}

	return c.JSON(emojis)
}

// DeleteCustomEmoji deletes a custom emoji
func DeleteCustomEmoji(c fiber.Ctx) error {
	spaceID := c.Params("spaceId")
	emojiID := c.Params("emojiId")

	if spaceID == "" || emojiID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Space ID and Emoji ID are required",
		})
	}

	// Get user from context
	userID := c.Locals("user_id").(string)

	// Load emoji metadata
	emojiDir := filepath.Join("uploads", "emojis", spaceID)
	metadataPath := filepath.Join(emojiDir, emojiID+".json")
	emoji, err := loadEmojiMetadata(metadataPath)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "Emoji not found",
		})
	}

	// Check if user has permission to delete (owner or space admin)
	if emoji.UploadedBy != userID {
		// TODO: Check if user is space admin
		return c.Status(403).JSON(fiber.Map{
			"error": "You don't have permission to delete this emoji",
		})
	}

	// Delete files
	filePath := filepath.Join(emojiDir, emoji.Filename)
	os.Remove(filePath)
	os.Remove(metadataPath)

	return c.JSON(fiber.Map{
		"message": "Emoji deleted successfully",
	})
}

// Helper function to save uploaded file
func saveUploadedFile(file *multipart.FileHeader, dst string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

// Helper function to save emoji metadata
func saveEmojiMetadata(emoji CustomEmoji, path string) error {
	data, err := json.MarshalIndent(emoji, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Helper function to load emoji metadata
func loadEmojiMetadata(path string) (CustomEmoji, error) {
	var emoji CustomEmoji
	data, err := os.ReadFile(path)
	if err != nil {
		return emoji, err
	}
	err = json.Unmarshal(data, &emoji)
	return emoji, err
}

// Helper function to check if emoji name exists in space
func checkEmojiNameExists(emojiDir, name string) bool {
	files, err := os.ReadDir(emojiDir)
	if err != nil {
		return false
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") {
			metadataPath := filepath.Join(emojiDir, file.Name())
			emoji, err := loadEmojiMetadata(metadataPath)
			if err == nil && emoji.Name == name {
				return true
			}
		}
	}
	return false
}

// Helper function to load all emojis for a space
func loadSpaceEmojis(emojiDir string) ([]CustomEmoji, error) {
	var emojis []CustomEmoji

	files, err := os.ReadDir(emojiDir)
	if err != nil {
		// If directory doesn't exist, return empty slice
		if os.IsNotExist(err) {
			return emojis, nil
		}
		return nil, err
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") {
			metadataPath := filepath.Join(emojiDir, file.Name())
			emoji, err := loadEmojiMetadata(metadataPath)
			if err == nil {
				emojis = append(emojis, emoji)
			}
		}
	}

	return emojis, nil
}