package database

import (
	"fmt"
	"log"

	"github.com/StrafeChat/nebula/src/types"
	"github.com/scylladb/gocqlx/v3/qb"
)

// StoreFileMetadata stores file metadata in the database
func StoreFileMetadata(metadata types.FileMetadata, storedFilename string) error {
	// Insert file metadata into files table
	stmt, names := qb.Insert("strafechatgo.files").
		Columns("id", "user_id", "type", "filename", "stored_filename", "mime_type", "size", "width", "height", "created_at", "updated_at").
		ToCql()

	log.Printf("Storing file metadata: ID=%s, UserID=%s, Type=%s, Filename=%s", metadata.ID, metadata.UserID, string(metadata.Type), metadata.Filename)

	q := Session.Query(stmt, names).BindMap(map[string]interface{}{
		"id":              metadata.ID,
		"user_id":         metadata.UserID,
		"type":            string(metadata.Type),
		"filename":        metadata.Filename,
		"stored_filename": storedFilename,
		"mime_type":       metadata.MimeType,
		"size":            metadata.Size,
		"width":           metadata.Width,
		"height":          metadata.Height,
		"created_at":      metadata.CreatedAt,
		"updated_at":      metadata.UpdatedAt,
	})

	if err := q.ExecRelease(); err != nil {
		log.Printf("Database error storing file metadata: %v", err)
		return fmt.Errorf("failed to store file metadata: %w", err)
	}

	log.Printf("File metadata stored successfully for ID: %s", metadata.ID)
	return nil
}

// GetFileMetadata retrieves file metadata from the database
func GetFileMetadata(fileID string) (types.FileMetadata, string, error) {
	var metadata types.FileMetadata

	stmt, names := qb.Select("strafechatgo.files").
		Columns("id", "user_id", "type", "filename", "stored_filename", "mime_type", "size", "width", "height", "created_at", "updated_at").
		Where(qb.Eq("id")).
		ToCql()

	q := Session.Query(stmt, names).BindMap(map[string]interface{}{
		"id": fileID,
	})

	var result struct {
		ID             string `db:"id"`
		UserID         string `db:"user_id"`
		Type           string `db:"type"`
		Filename       string `db:"filename"`
		StoredFilename string `db:"stored_filename"`
		MimeType       string `db:"mime_type"`
		Size           int64  `db:"size"`
		Width          *int   `db:"width"`
		Height         *int   `db:"height"`
		CreatedAt      int64  `db:"created_at"`
		UpdatedAt      int64  `db:"updated_at"`
	}

	if err := q.GetRelease(&result); err != nil {
		return metadata, "", fmt.Errorf("failed to get file metadata: %w", err)
	}

	metadata.ID = result.ID
	metadata.UserID = result.UserID
	metadata.Type = types.FileType(result.Type)
	metadata.Filename = result.Filename
	metadata.MimeType = result.MimeType
	metadata.Size = result.Size
	metadata.Width = result.Width
	metadata.Height = result.Height
	metadata.CreatedAt = result.CreatedAt
	metadata.UpdatedAt = result.UpdatedAt

	return metadata, result.StoredFilename, nil
}

// DeleteFileMetadata removes file metadata from the database
func DeleteFileMetadata(fileID string) error {
	stmt, names := qb.Delete("strafechatgo.files").
		Where(qb.Eq("id")).
		ToCql()

	q := Session.Query(stmt, names).BindMap(map[string]interface{}{
		"id": fileID,
	})

	if err := q.ExecRelease(); err != nil {
		return fmt.Errorf("failed to delete file metadata: %w", err)
	}

	return nil
}