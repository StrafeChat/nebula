package storage

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestCleanKeyRefusesEscapes(t *testing.T) {
	for _, k := range []string{"..", "../x", "a/../../b", "a/./b", "/", "", "a//b", "a\x00b", "a\\b", " "} {
		if _, err := CleanKey(k); err != ErrInvalidKey {
			t.Errorf("CleanKey(%q) = %v, want ErrInvalidKey", k, err)
		}
	}
	for in, want := range map[string]string{
		"avatars/x.png":                      "avatars/x.png",
		"/attachments/1/f.bin/":              "attachments/1/f.bin",
		"  emoji/blob.png  ":                 "emoji/blob.png",
		"attachments/1/name with spaces.txt": "attachments/1/name with spaces.txt",
	} {
		got, err := CleanKey(in)
		if err != nil || got != want {
			t.Errorf("CleanKey(%q) = (%q, %v), want (%q, nil)", in, got, err, want)
		}
	}
}

func TestFSRoundTripAndRange(t *testing.T) {
	st, err := NewFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	body := []byte("0123456789abcdef")
	if _, err := st.Put(ctx, "attachments/9/data.bin", bytes.NewReader(body), -1, ""); err != nil {
		t.Fatalf("put: %v", err)
	}
	info, err := st.Stat(ctx, "attachments/9/data.bin")
	if err != nil || info.Size != int64(len(body)) {
		t.Fatalf("stat: %v, size %d", err, info.Size)
	}
	rc, _, err := st.Open(ctx, "attachments/9/data.bin", &ByteRange{Start: 10, End: 13})
	if err != nil {
		t.Fatalf("open range: %v", err)
	}
	got, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(got) != "abcd" {
		t.Fatalf("range read %q, want %q", got, "abcd")
	}
	if err := st.Delete(ctx, "attachments/9/data.bin"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := st.Delete(ctx, "attachments/9/data.bin"); err != ErrNotFound {
		t.Fatalf("second delete: got %v, want ErrNotFound", err)
	}
}

// A traversal key must never turn into a path outside the root, whatever the OS does with
// it - this is the property that keeps a PUT from writing over the host's files.
func TestFSPathStaysUnderRoot(t *testing.T) {
	st, err := NewFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"../../etc/passwd", "avatars/../../x"} {
		if _, err := st.Path(k); err != ErrInvalidKey {
			t.Errorf("Path(%q) = %v, want ErrInvalidKey", k, err)
		}
	}
	p, err := st.Path("avatars/x.png")
	if err != nil || !strings.HasPrefix(p, st.Root) {
		t.Fatalf("Path = %q, %v; want a path under %q", p, err, st.Root)
	}
}
