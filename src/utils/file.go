package utils

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

var AllowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

var AllowedAttachmentTypes = map[string]bool{
	// Images
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
	"image/svg+xml": true,
	// Videos
	"video/mp4":  true,
	"video/webm": true,
	"video/ogg":  true,
	"video/avi":  true,
	"video/mov":  true,
	// Audio
	"audio/mp3":  true,
	"audio/mpeg": true,
	"audio/wav":  true,
	"audio/ogg":  true,
	"audio/m4a":  true,
	"audio/flac": true,
	// Documents
	"application/pdf":                                                 true,
	"application/msword":                                              true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.ms-excel":                                        true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
	"application/vnd.ms-powerpoint":                                   true,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
	"text/plain": true,
	"text/csv":   true,
	// Archives
	"application/zip":  true,
	"application/x-rar-compressed": true,
	"application/x-7z-compressed":  true,
	"application/x-tar": true,
	"application/gzip": true,
}

func SaveFile(file *multipart.FileHeader, userID string, fileType string) (string, error) {
	// Generate unique filename
	ext := filepath.Ext(file.Filename)
	filename := fmt.Sprintf("%s_%s%s", userID, uuid.New().String(), ext)

	// Create directory structure
	uploadDir := filepath.Join("uploads", fileType, userID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Save file
	dst := filepath.Join(uploadDir, filename)
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer out.Close()

	// Copy the uploaded file to the destination file
	buffer := make([]byte, 1024*1024) // 1MB buffer
	for {
		n, err := src.Read(buffer)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return "", fmt.Errorf("error reading uploaded file: %w", err)
		}

		if _, err := out.Write(buffer[:n]); err != nil {
			return "", fmt.Errorf("error writing to destination file: %w", err)
		}
	}

	return filename, nil
}

func ValidateFile(file *multipart.FileHeader, maxSize int64) error {
	if file.Size > maxSize {
		return fmt.Errorf("file size exceeds maximum allowed size of %d bytes", maxSize)
	}

	contentType := file.Header.Get("Content-Type")
	if !AllowedImageTypes[contentType] {
		return fmt.Errorf("invalid file type: %s", contentType)
	}

	return nil
}

func ValidateAttachment(file *multipart.FileHeader, maxSize int64) error {
	if file.Size > maxSize {
		return fmt.Errorf("file size exceeds maximum allowed size of %d bytes", maxSize)
	}

	contentType := file.Header.Get("Content-Type")
	if !AllowedAttachmentTypes[contentType] {
		return fmt.Errorf("invalid file type: %s", contentType)
	}

	return nil
}

// GetImageDimensions extracts width and height from image files
func GetImageDimensions(file *multipart.FileHeader) (*int, *int, error) {
	// Check if it's an image file
	contentType := file.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return nil, nil, nil // Not an image, return nil dimensions
	}

	// Open the file
	src, err := file.Open()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	// Decode image to get dimensions
	img, _, err := image.DecodeConfig(src)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode image: %w", err)
	}

	width := img.Width
	height := img.Height
	return &width, &height, nil
}

func DeleteOldFiles(userID string, fileType string) error {
	// Get the directory path
	dirPath := filepath.Join("uploads", fileType, userID)
	fmt.Printf("[DeleteOldFiles] Attempting to delete files in directory: %s\n", dirPath)

	// Check if directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		fmt.Printf("[DeleteOldFiles] Directory does not exist, nothing to delete: %s\n", dirPath)
		return nil // No directory means no files to delete
	} else if err != nil {
		fmt.Printf("[DeleteOldFiles] Error checking directory: %v\n", err)
		return fmt.Errorf("failed to check directory: %w", err)
	}

	// Read directory
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		fmt.Printf("[DeleteOldFiles] Failed to read directory %s: %v\n", dirPath, err)
		return fmt.Errorf("failed to read directory: %w", err)
	}

	fmt.Printf("[DeleteOldFiles] Found %d entries in directory\n", len(entries))

	// Delete all files in the directory
	// Don't fail the entire operation if some files can't be deleted (e.g., file in use)
	var deletionErrors []string
	for _, entry := range entries {
		if !entry.IsDir() {
			filePath := filepath.Join(dirPath, entry.Name())
			fmt.Printf("[DeleteOldFiles] Deleting file: %s\n", filePath)
			if err := os.Remove(filePath); err != nil {
				fmt.Printf("[DeleteOldFiles] Failed to delete file %s: %v (continuing...)\n", filePath, err)
				deletionErrors = append(deletionErrors, fmt.Sprintf("%s: %v", entry.Name(), err))
			} else {
				fmt.Printf("[DeleteOldFiles] Successfully deleted file: %s\n", filePath)
			}
		}
	}

	if len(deletionErrors) > 0 {
		fmt.Printf("[DeleteOldFiles] Some files could not be deleted: %v (this is non-fatal)\n", deletionErrors)
		// Log the errors but don't fail the upload - the new file can still be saved
	}

	fmt.Printf("[DeleteOldFiles] Completed deletion process\n")
	return nil
}
