package api

import (
	"os"

	"github.com/gofiber/fiber/v2"
)

func APIKeyAuth(apiKey string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if apiKey == "" {
			return c.Next()
		}

		key := c.Get("X-API-Key")
		if key == "" {
			return c.Status(401).JSON(fiber.Map{"error": "missing X-API-Key header"})
		}
		if key != apiKey {
			return c.Status(401).JSON(fiber.Map{"error": "invalid API key"})
		}

		return c.Next()
	}
}

func GetAPIKey() string {
	key := os.Getenv("API_KEY")
	if key == "" {
		key = "dev-key-change-me"
	}
	return key
}
