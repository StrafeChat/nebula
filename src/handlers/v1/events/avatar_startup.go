package events

import (
	"log"
	"os"
	"path/filepath"
)

// CheckUserAvatars scans all user avatar directories and ensures each has a default.webp file
func CheckUserAvatars() {
	log.Println("Starting user avatar check...")

	// Path to user avatars directory
	avatarsDir := filepath.Join("uploads", "avatars")

	// Check if the directory exists
	if _, err := os.Stat(avatarsDir); os.IsNotExist(err) {
		log.Printf("Avatars directory does not exist: %s", avatarsDir)
		return
	}

	// Read all user directories
	userDirs, err := os.ReadDir(avatarsDir)
	if err != nil {
		log.Printf("Failed to read avatars directory: %v", err)
		return
	}

	log.Printf("Found %d user avatar directories", len(userDirs))

	// Check each user directory for default.webp
	for _, userDir := range userDirs {
		if !userDir.IsDir() {
			continue
		}

		userID := userDir.Name()
		userAvatarDir := filepath.Join(avatarsDir, userID)
		defaultAvatarPath := filepath.Join(userAvatarDir, "default.webp")

		// Check if default.webp exists
		if _, err := os.Stat(defaultAvatarPath); os.IsNotExist(err) {
			log.Printf("User %s is missing default avatar, creating one...", userID)
			CreateDefaultAvatar(userID)
		}
	}

	log.Println("User avatar check completed")
}
