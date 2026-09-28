package middleware

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
)

func SecurityHeaders() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		return c.Next()
	}
}

// CORS mirrors equinox: nil/empty origins allows any Origin (useful for local dev).
//
// Vary: Origin matters here: the same asset URL is loaded both as a plain <img> (no CORS
// headers on the response) and via fetch() (needs them - e.g. to decrypt an E2EE
// attachment). Without Vary the browser reuses the cached no-CORS response for the
// fetch and rejects it.
func CORS(origins []string) fiber.Handler {
	return func(c fiber.Ctx) error {
		origin := c.Get("Origin")
		c.Set("Vary", "Origin")
		if len(origins) == 0 && origin != "" {
			c.Set("Access-Control-Allow-Origin", origin)
		} else if len(origins) > 0 {
			for _, o := range origins {
				if o == origin {
					c.Set("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}
		c.Set("Access-Control-Allow-Methods", "GET, HEAD, PUT, DELETE, OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
		c.Set("Access-Control-Allow-Credentials", "true")

		if c.Method() == http.MethodOptions {
			return c.SendStatus(http.StatusNoContent)
		}
		return c.Next()
	}
}
