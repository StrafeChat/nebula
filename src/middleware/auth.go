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
	// Get the token from the X-Session-Token header
	token := c.Get("X-Session-Token")
	if token == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authorization token",
		})
	}

	// Remove "Bearer " prefix if present
	token = strings.TrimPrefix(token, "Bearer ")

	// Query the session from the database
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

	return c.Next()
}
