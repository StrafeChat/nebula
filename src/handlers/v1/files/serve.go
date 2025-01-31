package files

import (
	"bytes"
	"image"
	_ "image/gif"  // register GIF format
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// ServeFile serves the file with optional format conversion
func ServeFile(c fiber.Ctx) error {
	path := c.Params("*")
	format := strings.ToLower(c.Query("format", "png"))

	// Validate format
	if format != "png" && format != "jpeg" {
		return c.Status(400).SendString("Invalid format. Supported formats: png, jpeg")
	}

	// Get absolute path
	filePath := filepath.Join("uploads", path)
	
	// Check if file exists
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return c.Status(404).SendString("File not found")
		}
		return c.Status(500).SendString("Error opening file")
	}
	defer file.Close()

	// Decode image
	img, _, err := image.Decode(file)
	if err != nil {
		return c.Status(500).SendString("Error decoding image")
	}

	// Create buffer for the converted image
	buf := new(bytes.Buffer)

	// Convert to requested format
	switch format {
	case "png":
		if err := png.Encode(buf, img); err != nil {
			return c.Status(500).SendString("Error encoding to PNG")
		}
		c.Set("Content-Type", "image/png")
	case "jpeg":
		if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 85}); err != nil {
			return c.Status(500).SendString("Error encoding to JPEG")
		}
		c.Set("Content-Type", "image/jpeg")
	}

	return c.Send(buf.Bytes())
}
