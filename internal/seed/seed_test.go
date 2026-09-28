package seed

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/StrafeChat/nebula/internal/storage"
)

// writeAssets lays out a miniature emoji tree: two sets, one file each, plus a VERSION.
func writeAssets(t *testing.T, version string) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"twemoji/1f600.svg":     "<svg>grin</svg>",
		"noto/emoji_u1f600.svg": "<svg>grin</svg>",
		"VERSION":               version,
	}
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func mustGet(t *testing.T, st storage.Storage, key string) string {
	t.Helper()
	rc, _, err := st.Open(context.Background(), key, nil)
	if err != nil {
		t.Fatalf("open %q: %v", key, err)
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read %q: %v", key, err)
	}
	return string(b)
}

func TestEmojiSeedsAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	assets := writeAssets(t, "v1")
	st, err := storage.NewFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	// First run seeds the two emoji (the VERSION marker is not an object).
	n, err := Emoji(ctx, st, assets)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if n != 2 {
		t.Fatalf("wrote %d objects, want 2", n)
	}
	if got := mustGet(t, st, "emoji/twemoji/1f600.svg"); got != "<svg>grin</svg>" {
		t.Fatalf("seeded content = %q", got)
	}
	if _, err := st.Stat(ctx, "emoji/VERSION"); err != storage.ErrNotFound {
		t.Fatalf("VERSION should not be stored as an object, got %v", err)
	}

	// Second run finds the manifest matching and writes nothing.
	n, err = Emoji(ctx, st, assets)
	if err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if n != 0 {
		t.Fatalf("second run wrote %d objects, want 0", n)
	}

	// A new version re-seeds.
	assets2 := writeAssets(t, "v2")
	n, err = Emoji(ctx, st, assets2)
	if err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	if n != 2 {
		t.Fatalf("re-seed wrote %d objects, want 2", n)
	}
}

func TestEmojiMissingDirIsNoop(t *testing.T) {
	st, err := storage.NewFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	n, err := Emoji(context.Background(), st, filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("missing dir should be a no-op, got %v", err)
	}
	if n != 0 {
		t.Fatalf("wrote %d objects from a missing dir, want 0", n)
	}
}
