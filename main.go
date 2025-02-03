package main

import (
	"fmt"
	"log"
	"os"

	"github.com/StrafeChat/nebula/src/config"
	"github.com/StrafeChat/nebula/src/database"
	"github.com/StrafeChat/nebula/src/handlers/v1/files"
	"github.com/StrafeChat/nebula/src/handlers/v1/users"
	"github.com/StrafeChat/nebula/src/middleware"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	// Initialize database connections
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Session.Close()

	// Load configuration
	cfg := config.LoadConfig()

	// Create uploads directory if it doesn't exist
	if err := os.MkdirAll("uploads", 0755); err != nil {
		log.Fatalf("Failed to create uploads directory: %v", err)
	}

	// Create Fiber app
	app := fiber.New(fiber.Config{
		BodyLimit: 10 * 1024 * 1024, // 10MB
	})

	// Middleware
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{cfg.Domain},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
	}))

	// API routes
	api := app.Group("/api")
	v1 := api.Group("/v1")

	// User routes
	userRoutes := v1.Group("/users")
	userRoutes.Use(middleware.Auth)
	userRoutes.Post("/avatar", users.UploadAvatar)
	userRoutes.Post("/banner", users.HandleBannerUpload)

	// File routes
	v1.Get("/files/*", files.ServeFile)

	// Static file serving
	app.Use("avatars", static.New("./uploads/avatars"))
	app.Use("banners", static.New("./uploads/banners"))
	app.Use("uploads", static.New("./uploads/uploads"))

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Server starting on %s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}