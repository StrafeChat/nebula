package main

import (
	"context"
	"log"
	"os"

	"github.com/StrafeChat/nebula/internal/config"
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
