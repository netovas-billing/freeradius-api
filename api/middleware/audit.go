package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"freeradius-api/database"
	"freeradius-api/models"
)

// Audit — log setelah handler. Tulis async supaya tidak memperlambat response.
// Hanya dipasang di group /api (lihat main.go).
func Audit(c *fiber.Ctx) error {
	start := time.Now()
	err := c.Next()

	dur := time.Since(start)
	name, _ := c.Locals("api_key_name").(string)

	entry := models.ApiAuditLog{
		Timestamp:  start,
		APIKeyName: name,
		Method:     c.Method(),
		Path:       c.Path(),
		Status:     c.Response().StatusCode(),
		IP:         c.IP(),
		UserAgent:  truncate(string(c.Request().Header.UserAgent()), 256),
		DurationMs: int(dur.Milliseconds()),
	}

	go func() {
		database.DB.Create(&entry)
	}()
	return err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
