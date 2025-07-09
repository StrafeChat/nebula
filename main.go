package main

import (
	"fmt"
	"log"
	"os"

	"github.com/StrafeChat/nebula/src/config"
	"github.com/StrafeChat/nebula/src/database"
	"github.com/StrafeChat/nebula/src/events"
	handlerevents "github.com/StrafeChat/nebula/src/handlers/v1/events"
	"github.com/StrafeChat/nebula/src/handlers/v1/files"
	"github.com/StrafeChat/nebula/src/handlers/v1/rooms"
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

	// Initialize Redis connection
	if err := database.InitRedis(); err != nil {
		log.Fatalf("Failed to initialize Redis: %v", err)
	}

	// Start avatar event listener in a goroutine
	go handlerevents.StartAvatarEventListener()

	// Start file event listener in a goroutine
	go events.StartFileEventListener()

	// Check all user avatars on startup
	handlerevents.CheckUserAvatars()

	// Ensure all users from database have avatar directories
	handlerevents.EnsureUserAvatars()

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
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization", "x-session-token"},
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

	// Room routes
	roomRoutes := v1.Group("/rooms")
	roomRoutes.Use(middleware.Auth)
	roomRoutes.Post("/:id/icon", rooms.UploadIcon)

	// File routes
	fileRoutes := v1.Group("/files")
	fileRoutes.Use(middleware.Auth)
	fileRoutes.Post("/", files.UploadFile)
	fileRoutes.Get("/:id", files.GetFile)

	// Static file serving
	app.Use("avatars", static.New("./uploads/avatars"))
	app.Use("banners", static.New("./uploads/banners"))
	app.Use("icons", static.New("./uploads/icons"))
	app.Use("attachments", static.New("./uploads/attachments"))

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Server starting on %s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
