package events

import (
	"log"
	"os"
	"path/filepath"

	"github.com/StrafeChat/nebula/src/database"
	"github.com/scylladb/gocqlx/v3/qb"
)

// EnsureBotAvatars queries all bots from the database and ensures each has an avatar directory
// with a default avatar file
func EnsureBotAvatars() {
	log.Println("Starting bot avatar check from database...")

	// Ensure uploads directory exists
	uploadsDir := "uploads"
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		log.Fatalf("Failed to create uploads directory: %v", err)
	}

	// Ensure defaultAvatars directory exists
	defaultAvatarsDir := filepath.Join(uploadsDir, "defaultAvatars")
	if err := os.MkdirAll(defaultAvatarsDir, 0755); err != nil {
		log.Fatalf("Failed to create defaultAvatars directory: %v", err)
	}

	// Check if default avatar exists
	defaultAvatarPath := filepath.Join(defaultAvatarsDir, "default.webp")
	if _, err := os.Stat(defaultAvatarPath); os.IsNotExist(err) {
		// Create an empty file as placeholder
		if file, err := os.Create(defaultAvatarPath); err != nil {
			log.Printf("Failed to create default avatar placeholder: %v", err)
		} else {
			file.Close()
			// This is just a placeholder message - in production you would use a real image file
			log.Printf("Warning: No default bot avatar image found. Please add a default.webp file to %s", defaultAvatarsDir)
			// In a real implementation, you would copy a default image here
		}
	}

	// Query all bot user IDs from the database
	var botUserIDs []string
	q := qb.Select("bots").Columns("user_id").AllowFiltering().Query(*database.Session)
	if err := q.Select(&botUserIDs); err != nil {
		log.Printf("Failed to query bot user IDs from database: %v", err)
		return
	}

	log.Printf("Found %d bots in database", len(botUserIDs))

	// Check each bot and ensure they have an avatar directory with a default avatar
	botAvatarsDir := filepath.Join(uploadsDir, "bot_avatars")
	if err := os.MkdirAll(botAvatarsDir, 0755); err != nil {
		log.Fatalf("Failed to create bot_avatars directory: %v", err)
	}

	for _, botUserID := range botUserIDs {
		botAvatarDir := filepath.Join(botAvatarsDir, botUserID)
		if err := os.MkdirAll(botAvatarDir, 0755); err != nil {
			log.Printf("Failed to create bot avatar directory for %s: %v", botUserID, err)
			continue
		}

		// Check if default avatar exists for this bot
		botDefaultAvatarPath := filepath.Join(botAvatarDir, "default.webp")
		if _, err := os.Stat(botDefaultAvatarPath); os.IsNotExist(err) {
			// Copy default avatar to bot directory
			if err := copyFile(defaultAvatarPath, botDefaultAvatarPath); err != nil {
				log.Printf("Failed to copy default avatar for bot %s: %v", botUserID, err)
			} else {
				log.Printf("Created default avatar for bot %s", botUserID)
			}
		}
	}

	log.Println("Bot avatar check completed")
}

// CreateDefaultBotAvatar creates a default avatar for a new bot
func CreateDefaultBotAvatar(botUserID string) {
	// Get a random default avatar
	defaultAvatar, err := GetRandomDefaultAvatar()
	if err != nil {
		log.Printf("Error getting random default avatar: %v", err)
		return
	}

	// Create bot avatar directory
	botAvatarDir := filepath.Join("uploads", "bot_avatars", botUserID)
	if err := os.MkdirAll(botAvatarDir, 0755); err != nil {
		log.Printf("Failed to create bot avatar directory: %v", err)
		return
	}

	// Use default.webp as the filename
	filename := "default.webp"
	destPath := filepath.Join(botAvatarDir, filename)

	// Copy the default avatar to the bot's directory
	if err := copyFile(defaultAvatar, destPath); err != nil {
		log.Printf("Failed to copy default avatar for bot %s: %v", botUserID, err)
		return
	}

	log.Printf("Created default avatar for bot %s", botUserID)
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = destFile.ReadFrom(sourceFile)
	return err
}