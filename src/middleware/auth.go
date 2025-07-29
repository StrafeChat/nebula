package middleware

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/StrafeChat/nebula/src/database"
	"github.com/StrafeChat/nebula/src/database/models"
	"github.com/gocql/gocql"
	"github.com/gofiber/fiber/v3"
	"github.com/scylladb/gocqlx/v3/qb"
)

func Auth(c fiber.Ctx) error {
	// Check for session token first, then bot token
	token := c.Get("X-Session-Token")
	isBot := false
	if token == "" {
		token = c.Get("X-Bot-Token")
		isBot = true
	}
	
	if token == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authorization token",
		})
	}

	// Remove "Bearer " prefix if present
	token = strings.TrimPrefix(token, "Bearer ")

	if isBot {
		// Handle bot authentication
		botByToken := models.BotByTokenTable.SelectBuilder().
			Columns("user_id").
			Where(qb.Eq("bot_token")).
			Limit(1)

		var bot models.BotByToken
		botByTokenQuery := botByToken.Query(*database.Session).
			BindStruct(models.BotByToken{
				Token: token,
			})

		if err := botByTokenQuery.GetRelease(&bot); err != nil {
			if errors.Is(err, gocql.ErrNotFound) {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Invalid bot token",
				})
			}
			fmt.Printf("Error getting bot: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Internal server error",
			})
		}

		// Get the user associated with the bot
		userById := models.UserTable.SelectBuilder().
			Columns("id", "username", "email", "discriminator", "display_name", "about_me", "bio", "bot", "created_at", "updated_at", "avatar", "banner", "accent_color", "locale", "verified_email", "flags").
			Where(qb.Eq("id")).
			Limit(1)

		var user models.User
		userByIdQuery := userById.Query(*database.Session).
			BindStruct(models.User{
				ID: bot.UserID,
			})

		if err := userByIdQuery.GetRelease(&user); err != nil {
			if errors.Is(err, gocql.ErrNotFound) {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Bot user not found",
				})
			}
			fmt.Printf("Error getting bot user: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Internal server error",
			})
		}

		c.Locals("user", user)
		c.Locals("bot", bot)
		c.Locals("user_id", user.ID)
	} else {
		// Handle session authentication
		sessionByToken := models.SessionTable.SelectBuilder().
			Columns("user_id", "expires_at").
			Where(qb.Eq("session_token")).
			Limit(1)

		sessionByTokenQuery := sessionByToken.Query(*database.Session).
			BindStruct(models.Session{
				Token: token,
			})

		var session models.Session
		if err := sessionByTokenQuery.GetRelease(&session); err != nil {
			if errors.Is(err, gocql.ErrNotFound) {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Invalid session token",
				})
			}
			fmt.Printf("Error getting session: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Internal server error",
			})
		}

		// Check if session has expired
		if session.ExpiresAt.Before(time.Now()) {
			// Delete expired session
			deleteSession := models.SessionTable.DeleteBuilder().
				Where(qb.Eq("session_token"))

			if err := deleteSession.Query(*database.Session).
				BindStruct(models.Session{Token: token}).
				ExecRelease(); err != nil {
				fmt.Printf("Error deleting expired session: %v", err)
			}

			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Session expired",
			})
		}

		// Set the user ID in the context for use in handlers
		c.Locals("user_id", session.UserId)
	}

	return c.Next()
}
