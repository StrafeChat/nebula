package events

import (
	"log"
	"os"
	"path/filepath"

	"github.com/StrafeChat/nebula/src/database"
	"github.com/scylladb/gocqlx/v3/qb"
)

// EnsureUserAvatars queries all users from the database and ensures each has an avatar directory
// with a default avatar file
func EnsureUserAvatars() {
	log.Println("Starting user avatar check from database...")

	// Ensure uploads directory exists
	uploadsDir := "uploads"
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		log.Fatalf("Failed to create uploads directory: %v", err)
	}

	// Ensure avatars directory exists
	avatarsDir := filepath.Join(uploadsDir, "avatars")
	if err := os.MkdirAll(avatarsDir, 0755); err != nil {
		log.Fatalf("Failed to create avatars directory: %v", err)
	}

	// Ensure defaultAvatars directory exists
	defaultAvatarsDir := filepath.Join(uploadsDir, "defaultAvatars")
	if err := os.MkdirAll(defaultAvatarsDir, 0755); err != nil {
		log.Fatalf("Failed to create defaultAvatars directory: %v", err)
	}

	// Check if there's at least one default avatar
	defaultAvatars, err := os.ReadDir(defaultAvatarsDir)
	if err != nil {
		log.Fatalf("Failed to read defaultAvatars directory: %v", err)
	}

	// If no default avatars exist, create a placeholder
	if len(defaultAvatars) == 0 {
		log.Println("No default avatars found, creating a placeholder...")
		// Create a simple placeholder file - in a real implementation, you would add an actual avatar image
		placeholderPath := filepath.Join(defaultAvatarsDir, "default.webp")
		if _, err := os.Stat(placeholderPath); os.IsNotExist(err) {
			// This is just a placeholder message - in production you would use a real image file
			log.Printf("Warning: No default avatar image found. Please add a default.webp file to %s", defaultAvatarsDir)
			// In a real implementation, you would copy a default image here
		}
	}

	// Query all user IDs from the database
	var userIDs []string
	q := qb.Select("users").Columns("id").AllowFiltering().Query(*database.Session)
	if err := q.Select(&userIDs); err != nil {
		log.Printf("Failed to query user IDs from database: %v", err)
		return
	}

	log.Printf("Found %d users in database", len(userIDs))

	// Check each user and ensure they have an avatar directory with a default avatar
	for _, userID := range userIDs {
		userAvatarDir := filepath.Join(avatarsDir, userID)
		defaultAvatarPath := filepath.Join(userAvatarDir, "default.webp")

		// Check if user avatar directory exists
		if _, err := os.Stat(userAvatarDir); os.IsNotExist(err) {
			log.Printf("Creating avatar directory for user %s", userID)
			if err := os.MkdirAll(userAvatarDir, 0755); err != nil {
				log.Printf("Failed to create avatar directory for user %s: %v", userID, err)
				continue
			}
		}

		// Check if default avatar exists
		if _, err := os.Stat(defaultAvatarPath); os.IsNotExist(err) {
			log.Printf("User %s is missing default avatar, creating one...", userID)
			CreateDefaultAvatar(userID)
		}
	}

	log.Println("User avatar check from database completed")
}
