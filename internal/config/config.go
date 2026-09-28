package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port               string
	UploadSecret       string
	BodyLimitBytes     int
	CORSOrigins        []string
	CacheControlMaxAge int // seconds for successful GET/HEAD

	// StorageBackend is "fs" (default: files under DataDir) or "s3" (any S3-compatible
	// bucket, for instances whose uploads outgrow a disk).
	StorageBackend string
	DataDir        string
	S3             S3Config

	// SeedEmoji, when true (the default), copies the bundled Unicode emoji sets into the
	// store on start so the web client renders emoji from this instance rather than a
	// third-party CDN. EmojiAssetsDir is where those bundled files live in the image.
	SeedEmoji      bool
	EmojiAssetsDir string
}

// S3Config is read only when StorageBackend is "s3".
type S3Config struct {
	Bucket          string
	Region          string
	Endpoint        string // empty = AWS; set for MinIO / R2 / B2 / Ceph
	Prefix          string // optional key prefix inside the bucket
	AccessKeyID     string
	SecretAccessKey string
	ForcePathStyle  bool // required by MinIO and most self-hosted services
}

func Load() (*Config, error) {
	bodyMB := getEnvInt("HTTP_BODY_LIMIT_MB", 25)
	if bodyMB < 1 {
		bodyMB = 25
	}
	cfg := &Config{
		Port:               getEnvString("PORT", "4010"),
		DataDir:            getEnvString("DATA_DIR", "./data"),
		UploadSecret:       strings.TrimSpace(os.Getenv("UPLOAD_SECRET")),
		BodyLimitBytes:     bodyMB * 1024 * 1024,
		CORSOrigins:        getEnvArray("CORS_ORIGINS", nil),
		CacheControlMaxAge: getEnvInt("CACHE_CONTROL_MAX_AGE", 86400),
		StorageBackend:     strings.ToLower(getEnvString("STORAGE_BACKEND", "fs")),
		SeedEmoji:          getEnvBool("SEED_EMOJI", true),
		EmojiAssetsDir:     getEnvString("EMOJI_ASSETS_DIR", "./emoji"),
		S3: S3Config{
			Bucket:          getEnvString("S3_BUCKET", ""),
			Region:          getEnvString("S3_REGION", "us-east-1"),
			Endpoint:        getEnvString("S3_ENDPOINT", ""),
			Prefix:          getEnvString("S3_PREFIX", ""),
			AccessKeyID:     getEnvString("S3_ACCESS_KEY_ID", ""),
			SecretAccessKey: getEnvString("S3_SECRET_ACCESS_KEY", ""),
			ForcePathStyle:  getEnvBool("S3_FORCE_PATH_STYLE", false),
		},
	}
	switch cfg.StorageBackend {
	case "fs":
	case "s3":
		if cfg.S3.Bucket == "" {
			return nil, errors.New("STORAGE_BACKEND=s3 requires S3_BUCKET")
		}
	default:
		return nil, errors.New("STORAGE_BACKEND must be fs or s3, got " + cfg.StorageBackend)
	}
	if cfg.Port == "" {
		return nil, errors.New("PORT is empty")
	}
	if _, err := strconv.Atoi(cfg.Port); err != nil {
		return nil, errors.New("PORT must be numeric")
	}
	if cfg.CacheControlMaxAge < 0 {
		cfg.CacheControlMaxAge = 0
	}
	return cfg, nil
}

func getEnvString(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getEnvArray(key string, def []string) []string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}

func getEnvBool(key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}
