// Package storage holds objects for nebula. Two backends share one interface: the local
// filesystem (the default, one directory, nothing to configure) and any S3-compatible
// bucket (AWS S3, MinIO, Cloudflare R2, Backblaze B2, ...) for instances whose uploads
// outgrow a disk.
//
// Keys are the same on both: "avatars/x.png", "attachments/<id>/<name>". A key that
// escapes its root ("..", absolute paths) is rejected before either backend sees it.
package storage

import (
	"context"
	"errors"
	"io"
	"path"
	"strings"
	"time"
)

var (
	ErrInvalidKey = errors.New("invalid object key")
	ErrNotFound   = errors.New("object not found")
)

// ObjectInfo is what a HEAD needs: size, modification time and the stored content type
// (empty when the backend does not record one - the filesystem does not).
type ObjectInfo struct {
	Size        int64
	ModTime     time.Time
	ContentType string
	ETag        string
}

// ByteRange is a single HTTP range, already parsed and clamped by the caller; End is
// inclusive, as in the header.
type ByteRange struct {
	Start, End int64
}

// Storage is one object store.
type Storage interface {
	// Stat returns metadata without a body. ErrNotFound if the key does not exist.
	Stat(ctx context.Context, key string) (ObjectInfo, error)
	// Open streams the object, or the given byte range of it when rng is non-nil. The
	// returned info describes the whole object, not the range.
	Open(ctx context.Context, key string, rng *ByteRange) (io.ReadCloser, ObjectInfo, error)
	// Put stores the body under key, replacing any existing object. size is a hint (-1
	// when unknown); contentType may be empty.
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) (int64, error)
	// Delete removes the object. ErrNotFound if it did not exist.
	Delete(ctx context.Context, key string) error
	// Name identifies the backend in logs ("fs", "s3").
	Name() string
}

// LocalPath is implemented by backends that keep objects as files on disk, which lets the
// HTTP layer hand the path to the framework's SendFile (kernel sendfile, ETags, ranges) and
// skip the streaming path entirely.
type LocalPath interface {
	// Path returns the absolute filesystem path of key, without checking it exists.
	Path(key string) (string, error)
}

// CleanKey normalises and validates a key: no leading/trailing slashes, no empty segments,
// no "." or ".." segments, no NUL. Returns ErrInvalidKey otherwise. Both backends apply it,
// so a traversal attempt is refused identically whether the store is a directory or a bucket.
func CleanKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	key = strings.Trim(key, "/")
	if key == "" || strings.ContainsRune(key, 0) || strings.Contains(key, "\\") {
		return "", ErrInvalidKey
	}
	for _, seg := range strings.Split(key, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return "", ErrInvalidKey
		}
	}
	// path.Clean is a second opinion: a key it would rewrite is not one we want to store.
	if path.Clean(key) != key {
		return "", ErrInvalidKey
	}
	return key, nil
}
