package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/StrafeChat/nebula/src/database"
)

type UserRegisteredEvent struct {
	Type      string `json:"type"`
	UserID    string `json:"user_id"`
	CreatedAt int64  `json:"created_at"`
}

// StartAvatarEventListener begins listening to Redis pub/sub events for user registration
func StartAvatarEventListener() {
	log.Println("Starting Avatar Event Listener")

	ctx := context.Background()
	pubsub := database.Rdb.Subscribe(ctx, "USER_EVENTS")
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		log.Printf("Received Redis pub/sub message: Channel=%s, Payload=%s",
			msg.Channel, msg.Payload)

		payload := []byte(strings.TrimSpace(msg.Payload))

		if len(payload) == 0 {
			log.Printf("Received empty payload in pub/sub message")
			continue
		}

		var event UserRegisteredEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			log.Printf("Error unmarshaling event (payload: %s): %v", string(payload), err)
			continue
		}

		// Check if this is a user registration event
		if event.Type == "USER_REGISTERED" && event.UserID != "" {
			log.Printf("Processing user registration event for user ID: %s", event.UserID)
			CreateDefaultAvatar(event.UserID)
		}
	}
}

// CreateDefaultAvatar creates a default avatar for a new user
func CreateDefaultAvatar(userID string) {
	// Get a random default avatar
	defaultAvatar, err := GetRandomDefaultAvatar()
	if err != nil {
		log.Printf("Error getting random default avatar: %v", err)
		return
	}

	// Create user avatar directory
	userAvatarDir := filepath.Join("uploads", "avatars", userID)
	if err := os.MkdirAll(userAvatarDir, 0755); err != nil {
		log.Printf("Failed to create user avatar directory: %v", err)
		return
	}

	// Use default.webp as the filename
	filename := "default.webp"
	destPath := filepath.Join(userAvatarDir, filename)

	// Copy the default avatar to the user's avatar directory
	srcFile, err := os.ReadFile(defaultAvatar)
	if err != nil {
		log.Printf("Failed to read default avatar: %v", err)
		return
	}

	if err := os.WriteFile(destPath, srcFile, 0644); err != nil {
		log.Printf("Failed to write user avatar: %v", err)
		return
	}

	// // Create file metadata
	// metadata := types.FileMetadata{
	// 	ID:        filename,
	// 	UserID:    userID,
	// 	Type:      types.Avatar,
	// 	Filename:  filename,
	// 	MimeType:  "image/webp",
	// 	Size:      int64(len(srcFile)),
	// 	CreatedAt: time.Now().Unix(),
	// 	UpdatedAt: time.Now().Unix(),
	// }

	log.Printf("Created default avatar for user %s: %s", userID, filename)
}

// GetRandomDefaultAvatar returns the path to a random default avatar
func GetRandomDefaultAvatar() (string, error) {
	// Get all default avatars
	defaultAvatarsDir := filepath.Join("uploads", "defaultAvatars")
	files, err := os.ReadDir(defaultAvatarsDir)
	if err != nil {
		return "", fmt.Errorf("failed to read default avatars directory: %w", err)
	}

	var avatars []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".webp") {
			avatars = append(avatars, filepath.Join(defaultAvatarsDir, file.Name()))
		}
	}

	if len(avatars) == 0 {
		return "", fmt.Errorf("no default avatars found")
	}

	// Return a random avatar
	return avatars[rand.Intn(len(avatars))], nil
}
