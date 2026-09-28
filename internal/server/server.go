package server

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/StrafeChat/nebula/internal/config"
	"github.com/StrafeChat/nebula/internal/middleware"
	"github.com/StrafeChat/nebula/internal/storage"
	"github.com/gofiber/fiber/v3"
	fiberrecover "github.com/gofiber/fiber/v3/middleware/recover"
)

type Server struct {
	cfg *config.Config
	app *fiber.App
	st  storage.Storage
}

func New(cfg *config.Config, st storage.Storage) (*Server, error) {
	f := fiber.New(fiber.Config{
		BodyLimit: cfg.BodyLimitBytes,
		AppName:   "nebula",
		// Uploads are streamed straight into the store instead of being buffered whole in
		// memory first; with a 25 MB default limit and a bucket behind us that matters.
		StreamRequestBody: true,
	})
	f.Use(fiberrecover.New())
	f.Use(middleware.SecurityHeaders())
	f.Use(middleware.CORS(cfg.CORSOrigins))

	s := &Server{cfg: cfg, app: f, st: st}
	s.routes()
	return s, nil
}

func (s *Server) routes() {
	s.app.Get("/health", s.handleHealth)
	s.app.Get("/", s.handleRoot)

	s.app.All("/v1/*", s.handleV1Object)
}

func (s *Server) handleHealth(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"ok": true, "service": "nebula", "storage": s.st.Name()})
}

func (s *Server) handleRoot(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"service": "nebula",
		"role":    "strafe asset host / cdn",
		"objects": "/v1/<path>",
		"storage": s.st.Name(),
	})
}

func objectKeyFromPath(p string) string {
	k := strings.TrimPrefix(p, "/v1/")
	return strings.Trim(k, "/")
}

func (s *Server) handleV1Object(c fiber.Ctx) error {
	key := objectKeyFromPath(c.Path())
	if key == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "missing object key"})
	}

	switch c.Method() {
	case http.MethodGet:
		return s.serveObject(c, key, false)
	case http.MethodHead:
		return s.serveObject(c, key, true)
	case http.MethodPut:
		return s.putObject(c, key)
	case http.MethodDelete:
		return s.deleteObject(c, key)
	default:
		return c.SendStatus(http.StatusMethodNotAllowed)
	}
}

// contentTypeFor derives the type from the key's extension for both backends, so an
// object is served the same way wherever it lives - and so the "force download" decision
// below cannot be steered by a Content-Type an uploader chose.
func contentTypeFor(key string) string {
	if ct := mime.TypeByExtension(path.Ext(key)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

func (s *Server) serveObject(c fiber.Ctx, key string, headOnly bool) error {
	ctx := c.Context()

	// Files on disk go through the framework's SendFile: kernel sendfile, ETag/
	// If-Modified-Since handling and byte ranges for free. Buckets are streamed below.
	if local, ok := s.st.(storage.LocalPath); ok {
		return s.serveLocalFile(c, local, key, headOnly)
	}

	info, err := s.st.Stat(ctx, key)
	if err != nil {
		return s.storageError(c, key, err)
	}
	ct := contentTypeFor(key)
	c.Set("Content-Type", ct)
	c.Set("Accept-Ranges", "bytes")
	c.Set("Last-Modified", info.ModTime.UTC().Format(http.TimeFormat))
	if info.ETag != "" {
		c.Set("ETag", info.ETag)
	}
	if s.cfg.CacheControlMaxAge > 0 {
		c.Set("Cache-Control", fmt.Sprintf("public, max-age=%d", s.cfg.CacheControlMaxAge))
	}
	s.setDisposition(c, key, ct)

	if headOnly {
		c.Set("Content-Length", strconv.FormatInt(info.Size, 10))
		return c.SendStatus(http.StatusOK)
	}

	// A single byte range (what media players ask for when seeking). Multi-range requests
	// are rare from browsers and fall back to the whole object, which is still correct.
	rng, ok := parseRange(c.Get("Range"), info.Size)
	if c.Get("Range") != "" && !ok {
		c.Set("Content-Range", fmt.Sprintf("bytes */%d", info.Size))
		return c.SendStatus(http.StatusRequestedRangeNotSatisfiable)
	}

	body, _, err := s.st.Open(ctx, key, rng)
	if err != nil {
		return s.storageError(c, key, err)
	}
	if rng != nil {
		c.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", rng.Start, rng.End, info.Size))
		c.Set("Content-Length", strconv.FormatInt(rng.End-rng.Start+1, 10))
		c.Status(http.StatusPartialContent)
	} else {
		c.Set("Content-Length", strconv.FormatInt(info.Size, 10))
	}
	return c.SendStream(body)
}

func (s *Server) serveLocalFile(c fiber.Ctx, local storage.LocalPath, key string, headOnly bool) error {
	info, err := s.st.Stat(c.Context(), key)
	if err != nil {
		return s.storageError(c, key, err)
	}
	p, err := local.Path(key)
	if err != nil {
		return s.storageError(c, key, err)
	}
	ct := contentTypeFor(key)
	if headOnly {
		c.Set("Content-Type", ct)
		c.Set("Content-Length", strconv.FormatInt(info.Size, 10))
		c.Set("Last-Modified", info.ModTime.UTC().Format(http.TimeFormat))
		if s.cfg.CacheControlMaxAge > 0 {
			c.Set("Cache-Control", fmt.Sprintf("public, max-age=%d", s.cfg.CacheControlMaxAge))
		}
		return c.SendStatus(http.StatusOK)
	}
	s.setDisposition(c, key, ct)
	return c.SendFile(p, fiber.SendFile{
		MaxAge:    s.cfg.CacheControlMaxAge,
		ByteRange: true,
	})
}

// setDisposition forces user-uploaded message attachments that a browser would *execute*
// rather than display (HTML, SVG with scripts, unknown binaries) to download. Images,
// video, audio, PDFs and plain text render inline as expected.
func (s *Server) setDisposition(c fiber.Ctx, key, ct string) {
	if strings.HasPrefix(key, "attachments/") && !inlineSafe(ct) {
		c.Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(path.Base(key), `"`, "")+`"`)
	}
}

func (s *Server) storageError(c fiber.Ctx, key string, err error) error {
	switch err {
	case storage.ErrInvalidKey:
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid key"})
	case storage.ErrNotFound:
		return c.SendStatus(http.StatusNotFound)
	}
	log.Printf("nebula: %s %q: %v", c.Method(), key, err)
	return c.SendStatus(http.StatusInternalServerError)
}

// parseRange understands "bytes=start-end", "bytes=start-" and "bytes=-suffix". Returns
// (nil, true) when there is no usable range and the whole object should be served, and
// (nil, false) when the header is present but unsatisfiable.
func parseRange(h string, size int64) (*storage.ByteRange, bool) {
	if h == "" || !strings.HasPrefix(h, "bytes=") || strings.Contains(h, ",") {
		return nil, h == ""
	}
	spec := strings.TrimPrefix(h, "bytes=")
	start, end, found := strings.Cut(spec, "-")
	if !found {
		return nil, false
	}
	var r storage.ByteRange
	switch {
	case start == "" && end != "":
		n, err := strconv.ParseInt(end, 10, 64)
		if err != nil || n <= 0 {
			return nil, false
		}
		if n > size {
			n = size
		}
		r = storage.ByteRange{Start: size - n, End: size - 1}
	case start != "":
		a, err := strconv.ParseInt(start, 10, 64)
		if err != nil || a < 0 || a >= size {
			return nil, false
		}
		b := size - 1
		if end != "" {
			if b, err = strconv.ParseInt(end, 10, 64); err != nil || b < a {
				return nil, false
			}
			if b > size-1 {
				b = size - 1
			}
		}
		r = storage.ByteRange{Start: a, End: b}
	default:
		return nil, false
	}
	return &r, true
}

func inlineSafe(contentType string) bool {
	ct := strings.ToLower(contentType)
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	if ct == "image/svg+xml" {
		return false
	}
	return strings.HasPrefix(ct, "image/") ||
		strings.HasPrefix(ct, "video/") ||
		strings.HasPrefix(ct, "audio/") ||
		ct == "application/pdf" ||
		ct == "text/plain"
}

func (s *Server) putObject(c fiber.Ctx, key string) error {
	if s.cfg.UploadSecret == "" {
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "uploads are disabled (UPLOAD_SECRET not set)",
		})
	}
	if !constantTimeBearer(c.Get("Authorization"), s.cfg.UploadSecret) {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	size := int64(-1)
	if n, err := strconv.ParseInt(c.Get("Content-Length"), 10, 64); err == nil && n >= 0 {
		size = n
	}
	n, err := s.st.Put(c.Context(), key, requestBodyReader(c), size, contentTypeFor(key))
	if err != nil {
		return s.storageError(c, key, err)
	}

	w := map[string]interface{}{
		"key":   key,
		"bytes": n,
		"path":  "/v1/" + key,
	}
	b, err := json.Marshal(w)
	if err != nil {
		return c.SendStatus(http.StatusInternalServerError)
	}
	c.Set("Content-Type", "application/json; charset=utf-8")
	return c.Status(http.StatusCreated).Send(b)
}

func (s *Server) deleteObject(c fiber.Ctx, key string) error {
	if s.cfg.UploadSecret == "" {
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "deletes are disabled (UPLOAD_SECRET not set)",
		})
	}
	if !constantTimeBearer(c.Get("Authorization"), s.cfg.UploadSecret) {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	if err := s.st.Delete(c.Context(), key); err != nil {
		return s.storageError(c, key, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func requestBodyReader(c fiber.Ctx) io.Reader {
	if r := c.RequestCtx().RequestBodyStream(); r != nil {
		return r
	}
	return bytes.NewReader(c.Body())
}

func constantTimeBearer(authHeader, secret string) bool {
	if secret == "" {
		return false
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return false
	}
	token := strings.TrimSpace(authHeader[len(prefix):])
	return subtle.ConstantTimeCompare([]byte(token), []byte(secret)) == 1
}

func (s *Server) ListenAndShutdown() error {
	addr := ":" + s.cfg.Port
	go func() {
		log.Printf("nebula: listening on %s (storage=%s)", addr, s.st.Name())
		if err := s.app.Listen(addr); err != nil {
			log.Printf("nebula: server stopped: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Printf("nebula: shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.app.ShutdownWithContext(ctx)
}
