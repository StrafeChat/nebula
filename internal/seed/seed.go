// Package seed populates the object store with assets that ship with nebula rather than
// being uploaded by users. Right now that is the Unicode emoji sets the web client renders:
// seeding them here lets an instance serve emoji from its own CDN under `/v1/emoji/<set>/…`
// instead of a third-party host, so no viewer's IP or referrer leaks to an outside CDN and
// an instance with no outbound internet still renders every emoji.
package seed

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/StrafeChat/nebula/internal/storage"
)

// KeyPrefix is where seeded emoji live, matching the web client's `/v1/emoji/<set>/…` URLs.
const KeyPrefix = "emoji"

// manifestKey records the version of the emoji set last seeded. When it matches the version
// on disk the walk is skipped entirely, so a restart against an already-seeded store (an s3
// bucket in particular) costs a single read rather than thousands of writes.
const manifestKey = KeyPrefix + "/.seed-manifest"

// versionFile is the marker the fetch script writes at the root of the assets dir. It is
// bookkeeping, not an emoji, so it is never stored as an object.
const versionFile = "VERSION"

// Emoji copies every file under dir into the store under `emoji/<relative path>`, unless the
// store already holds this version of the set. It returns the number of objects written.
//
// A missing or empty dir is not an error: the instance simply serves no bundled emoji and
// the client falls back to the system font. Run it in the background - on a fresh s3 bucket
// the first seed uploads thousands of small objects.
func Emoji(ctx context.Context, st storage.Storage, dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("nebula: emoji seed skipped: assets dir %q does not exist (run scripts/fetch-emoji.sh)", dir)
			return 0, nil
		}
		return 0, err
	}
	if len(entries) == 0 {
		log.Printf("nebula: emoji seed skipped: assets dir %q is empty", dir)
		return 0, nil
	}

	version := readVersion(dir)
	if have := currentManifest(ctx, st); have != "" && have == version {
		log.Printf("nebula: emoji already seeded (%s)", version)
		return 0, nil
	}

	log.Printf("nebula: seeding emoji from %q (version %s)...", dir, version)
	written := 0
	err = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return ctx.Err()
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == versionFile {
			return nil
		}
		if err := putFile(ctx, st, KeyPrefix+"/"+rel, p); err != nil {
			return err
		}
		written++
		return nil
	})
	if err != nil {
		return written, err
	}

	// Record the version only after every object landed, so an interrupted seed re-runs
	// next start rather than being wrongly treated as complete.
	if _, err := st.Put(ctx, manifestKey, strings.NewReader(version), int64(len(version)), "text/plain"); err != nil {
		return written, err
	}
	log.Printf("nebula: seeded %d emoji objects (version %s)", written, version)
	return written, nil
}

// putFile streams one file into the store, closing it before returning so a large tree does
// not hold thousands of descriptors open at once.
func putFile(ctx context.Context, st storage.Storage, key, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	// Content type is left empty: nebula derives it from the key's extension when serving,
	// the same for a seeded object as for an uploaded one.
	_, err = st.Put(ctx, key, f, info.Size(), "")
	return err
}

// readVersion returns the trimmed contents of the assets dir's VERSION file, or a stable
// fallback so a set shipped without one still seeds exactly once.
func readVersion(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, versionFile))
	if err != nil {
		return "unversioned"
	}
	if v := strings.TrimSpace(string(b)); v != "" {
		return v
	}
	return "unversioned"
}

// currentManifest reads the version last seeded, or "" if none is recorded yet.
func currentManifest(ctx context.Context, st storage.Storage) string {
	rc, _, err := st.Open(ctx, manifestKey, nil)
	if err != nil {
		return ""
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, 256))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
