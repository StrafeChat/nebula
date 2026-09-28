package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FS stores objects as files under Root.
type FS struct {
	Root string
}

// NewFS creates the root directory if needed.
func NewFS(root string) (*FS, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	return &FS{Root: abs}, nil
}

func (f *FS) Name() string { return "fs" }

// Path maps a key to an absolute path under Root, refusing anything that would resolve
// outside it. Implements LocalPath.
func (f *FS) Path(key string) (string, error) {
	key, err := CleanKey(key)
	if err != nil {
		return "", err
	}
	p := filepath.Join(f.Root, filepath.FromSlash(key))
	// CleanKey already rejects "..", but the belt-and-braces check costs nothing and is the
	// one that matters if CleanKey is ever loosened.
	rel, err := filepath.Rel(f.Root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrInvalidKey
	}
	return p, nil
}

func (f *FS) Stat(_ context.Context, key string) (ObjectInfo, error) {
	p, err := f.Path(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	st, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return ObjectInfo{}, ErrNotFound
		}
		return ObjectInfo{}, err
	}
	if st.IsDir() {
		return ObjectInfo{}, ErrNotFound
	}
	return ObjectInfo{Size: st.Size(), ModTime: st.ModTime()}, nil
}

func (f *FS) Open(_ context.Context, key string, rng *ByteRange) (io.ReadCloser, ObjectInfo, error) {
	p, err := f.Path(key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	file, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ObjectInfo{}, ErrNotFound
		}
		return nil, ObjectInfo{}, err
	}
	st, err := file.Stat()
	if err != nil || st.IsDir() {
		_ = file.Close()
		if err == nil {
			err = ErrNotFound
		}
		return nil, ObjectInfo{}, err
	}
	info := ObjectInfo{Size: st.Size(), ModTime: st.ModTime()}
	if rng == nil {
		return file, info, nil
	}
	if _, err := file.Seek(rng.Start, io.SeekStart); err != nil {
		_ = file.Close()
		return nil, ObjectInfo{}, err
	}
	return &limitedFile{File: file, Reader: io.LimitReader(file, rng.End-rng.Start+1)}, info, nil
}

// limitedFile reads a byte range but still closes the underlying file.
type limitedFile struct {
	*os.File
	io.Reader
}

func (l *limitedFile) Read(p []byte) (int, error) { return l.Reader.Read(p) }

func (f *FS) Put(_ context.Context, key string, r io.Reader, _ int64, _ string) (int64, error) {
	p, err := f.Path(key)
	if err != nil {
		return 0, err
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}
	// Write to a temp file in the same directory and rename: a reader never sees a
	// half-written object, and a failed upload leaves nothing behind.
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return 0, err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	written, err := io.Copy(tmp, r)
	if err != nil {
		_ = tmp.Close()
		return 0, err
	}
	if err := tmp.Close(); err != nil {
		return 0, err
	}
	if err := os.Rename(tmpPath, p); err != nil {
		return 0, err
	}
	return written, nil
}

func (f *FS) Delete(_ context.Context, key string) error {
	p, err := f.Path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}
