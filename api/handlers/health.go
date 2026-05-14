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

// Root godoc
// @Summary Root info
// @Tags meta
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router / [get]
func Root(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"name":    "freeradius-api",
		"version": "0.3.0",
		"docs":    "/docs/index.html",
		"apis": fiber.Map{
			"v1":      "/api/v1/*  (legacy, nasvpntest-api compatible)",
			"current": "/api/*     (modern, composite responses, multi-key, scope, audit)",
		},
	})
}
