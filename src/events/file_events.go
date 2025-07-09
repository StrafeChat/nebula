package events

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/StrafeChat/nebula/src/database"
	"github.com/StrafeChat/nebula/src/types"
)

// FileMetadataRequest represents a file metadata request from Redis
type FileMetadataRequest struct {
	Type      string `json:"type"`
	FileID    string `json:"file_id"`
	RequestID string `json:"request_id"`
	CreatedAt int64  `json:"created_at"`
}

// FileMetadataResponse represents a file metadata response to Redis
type FileMetadataResponse struct {
	Type      string                 `json:"type"`
	RequestID string                 `json:"request_id"`
	FileID    string                 `json:"file_id"`
	Success   bool                   `json:"success"`
	Data      *types.FileMetadata    `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	CreatedAt int64                  `json:"created_at"`
}

// StartFileEventListener starts listening for file events from Redis
func StartFileEventListener() {
	log.Printf("Starting Redis file event listener")

	// Subscribe to FILE_EVENTS channel
	pubsub := database.Rdb.Subscribe(context.Background(), "FILE_EVENTS")
	defer pubsub.Close()

	ch := pubsub.Channel()
	log.Printf("Successfully subscribed to FILE_EVENTS channel")

	for msg := range ch {
		log.Printf("[StartFileEventListener] Received file event: %s", msg.Payload)

		payload := []byte(strings.TrimSpace(msg.Payload))

		if len(payload) == 0 {
			log.Printf("[StartFileEventListener] Received empty payload in file event")
			continue
		}

		var event FileMetadataRequest
		if err := json.Unmarshal(payload, &event); err != nil {
			log.Printf("[StartFileEventListener] Error unmarshaling file event (payload: %s): %v", string(payload), err)
			continue
		}

		if event.Type == "" {
			log.Printf("[StartFileEventListener] Received file event with empty type: %+v", event)
			continue
		}

		log.Printf("[StartFileEventListener] Processing file event: Type=%s, FileID=%s, RequestID=%s", event.Type, event.FileID, event.RequestID)

		switch event.Type {
		case "FILE_METADATA_REQUEST":
			log.Printf("[StartFileEventListener] Handling FILE_METADATA_REQUEST event")
			go handleFileMetadataRequest(event)
		default:
			log.Printf("[StartFileEventListener] Unknown file event type: %s", event.Type)
		}
	}
}

// handleFileMetadataRequest handles file metadata request events
func handleFileMetadataRequest(event FileMetadataRequest) {
	log.Printf("[handleFileMetadataRequest] Starting to handle file metadata request: FileID=%s, RequestID=%s", event.FileID, event.RequestID)

	// Query the file metadata from the database
	fileMetadata, _, err := database.GetFileMetadata(event.FileID)
	if err != nil {
		log.Printf("[handleFileMetadataRequest] Failed to get file metadata for %s: %v", event.FileID, err)
		
		// Send error response
		errorResponse := FileMetadataResponse{
			Type:      "FILE_METADATA_RESPONSE",
			RequestID: event.RequestID,
			FileID:    event.FileID,
			Success:   false,
			Error:     "File not found",
			CreatedAt: time.Now().Unix(),
		}
		publishFileMetadataResponse(errorResponse)
		return
	}

	log.Printf("[handleFileMetadataRequest] Successfully retrieved file metadata for %s", event.FileID)

	// Send success response
	successResponse := FileMetadataResponse{
		Type:      "FILE_METADATA_RESPONSE",
		RequestID: event.RequestID,
		FileID:    event.FileID,
		Success:   true,
		Data:      &fileMetadata,
		CreatedAt: time.Now().Unix(),
	}
	publishFileMetadataResponse(successResponse)
}

// publishFileMetadataResponse publishes a file metadata response to Redis
func publishFileMetadataResponse(response FileMetadataResponse) {
	log.Printf("[publishFileMetadataResponse] Creating response payload for RequestID %s", response.RequestID)
	
	// Marshal the response to JSON
	responseBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("[publishFileMetadataResponse] Failed to marshal response: %v", err)
		return
	}
	log.Printf("[publishFileMetadataResponse] Response JSON: %s", string(responseBytes))

	// Publish to Redis FILE_EVENTS channel
	ctx := context.Background()
	log.Printf("[publishFileMetadataResponse] Publishing to Redis FILE_EVENTS channel")
	if err := database.Rdb.Publish(ctx, "FILE_EVENTS", string(responseBytes)).Err(); err != nil {
		log.Printf("[publishFileMetadataResponse] Failed to publish to Redis: %v", err)
		return
	}
	log.Printf("[publishFileMetadataResponse] Successfully published response to Redis")
}