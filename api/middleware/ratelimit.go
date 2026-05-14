package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"freeradius-api/config"
)

// RateLimit — fiber bawaan limiter, key per API key name (atau IP kalau belum auth).
// Diterapkan setelah APIKey supaya key name sudah ada di Locals.
func RateLimit() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        config.RateLimitMax,
		Expiration: time.Duration(config.RateLimitWindowSeconds) * time.Second,
		KeyGenerator: func(c *fiber.Ctx) string {
			if n, ok := c.Locals("api_key_name").(string); ok && n != "" {
				return "key:" + n
			}
			return "ip:" + c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "rate limit exceeded",
			})
		},
	})
}
