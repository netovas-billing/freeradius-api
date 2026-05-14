package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"freeradius-api/apierr"
	"freeradius-api/config"
)

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
			return apierr.TooMany(c, "Rate limit exceeded")
		},
	})
}
