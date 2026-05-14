package handlers

import (
	"github.com/gofiber/fiber/v2"

	"freeradius-api/database"
)

// Health godoc
// @Summary Healthcheck
// @Tags meta
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func Health(c *fiber.Ctx) error {
	sqlDB, err := database.DB.DB()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "detail": err.Error()})
	}
	if err := sqlDB.Ping(); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "detail": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}
