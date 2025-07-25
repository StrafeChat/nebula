package bots

import (
	"log"

	"github.com/StrafeChat/nebula/src/handlers/v1/events"
	"github.com/gofiber/fiber/v3"
)

type CreateDefaultAvatarRequest struct {
	BotUserID string `json:"bot_user_id" validate:"required"`
}

// CreateDefaultAvatar creates a default avatar for a bot
func CreateDefaultAvatar(c fiber.Ctx) error {
	var req CreateDefaultAvatarRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Create default avatar for the bot
	events.CreateDefaultBotAvatar(req.BotUserID)

	log.Printf("Created default avatar for bot %s", req.BotUserID)

	return c.Status(200).JSON(fiber.Map{
		"message": "Default avatar created successfully",
	})
}