package main

import (
	"context"
	"log"
	"os"

	"github.com/StrafeChat/nebula/internal/config"
	"github.com/StrafeChat/nebula/internal/seed"
	"github.com/StrafeChat/nebula/internal/server"
	"github.com/StrafeChat/nebula/internal/storage"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	st, err := newStorage(cfg)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("nebula: storage backend %s", st.Name())

	// Seed the bundled emoji sets in the background so the client renders emoji from this
	// instance, not a third-party CDN. It is idempotent and non-fatal, and the server starts
	// serving immediately - on a fresh store emoji 404 (client falls back to the system font)
	// only until the first seed finishes.
	if cfg.SeedEmoji {
		go func() {
			if _, err := seed.Emoji(context.Background(), st, cfg.EmojiAssetsDir); err != nil {
				log.Printf("nebula: emoji seed failed: %v", err)
			}
		}()
	}

	srv, err := server.New(cfg, st)
	if err != nil {
		log.Fatal(err)
	}
	if err := srv.ListenAndShutdown(); err != nil {
		log.Fatal(err)
	}
}

func newStorage(cfg *config.Config) (storage.Storage, error) {
	switch cfg.StorageBackend {
	case "s3":
		return storage.NewS3(context.Background(), storage.S3Config{
			Bucket:          cfg.S3.Bucket,
			Region:          cfg.S3.Region,
			Endpoint:        cfg.S3.Endpoint,
			Prefix:          cfg.S3.Prefix,
			AccessKeyID:     cfg.S3.AccessKeyID,
			SecretAccessKey: cfg.S3.SecretAccessKey,
			ForcePathStyle:  cfg.S3.ForcePathStyle,
		})
	default:
		return storage.NewFS(cfg.DataDir)
	}
}
