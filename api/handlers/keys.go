package handlers

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gofiber/fiber/v2"

	"freeradius-api/database"
	"freeradius-api/middleware"
	"freeradius-api/models"
	"freeradius-api/schemas"
)

func RegisterKeys(r fiber.Router) {
	g := r.Group("/keys")
	g.Get("", listKeys)
	g.Post("", createKey)
	g.Put("/:id", updateKey)
	g.Delete("/:id", deleteKey)
}

func validScope(s string) bool {
	return s == middleware.ScopeRead || s == middleware.ScopeWrite || s == middleware.ScopeAdmin
}

// listKeys godoc
// @Summary List API keys (admin)
// @Tags keys
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {array} models.ApiKey
// @Router /api/keys [get]
func listKeys(c *fiber.Ctx) error {
	var rows []models.ApiKey
	database.DB.Order("id").Find(&rows)
	return c.JSON(rows)
}

// createKey godoc
// @Summary Create new API key (admin)
// @Description Returns the raw key ONCE. Hash-nya yang disimpan; tidak bisa di-recover.
// @Tags keys
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param body body schemas.ApiKeyCreate true "Key payload"
// @Success 201 {object} schemas.ApiKeyCreatedResponse
// @Router /api/keys [post]
func createKey(c *fiber.Ctx) error {
	var payload schemas.ApiKeyCreate
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if payload.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name required"})
	}
	if !validScope(payload.Scope) {
		return c.Status(400).JSON(fiber.Map{"error": "scope must be one of: read, write, admin"})
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	rawHex := hex.EncodeToString(raw)
	hash := middleware.HashKey(rawHex)

	row := models.ApiKey{
		Name:    payload.Name,
		KeyHash: hash,
		Scope:   payload.Scope,
		Enabled: true,
		Notes:   payload.Notes,
	}
	if err := database.DB.Create(&row).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(201).JSON(schemas.ApiKeyCreatedResponse{
		ID:    row.ID,
		Name:  row.Name,
		Scope: row.Scope,
		Key:   rawHex,
	})
}

// updateKey godoc
// @Summary Update API key (admin)
// @Tags keys
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "Key ID"
// @Param body body schemas.ApiKeyUpdate true "Fields to update"
// @Success 200 {object} models.ApiKey
// @Router /api/keys/{id} [put]
func updateKey(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("id")
	var row models.ApiKey
	if err := database.DB.First(&row, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Key not found"})
	}
	var patch schemas.ApiKeyUpdate
	if err := c.BodyParser(&patch); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if patch.Scope != nil {
		if !validScope(*patch.Scope) {
			return c.Status(400).JSON(fiber.Map{"error": "invalid scope"})
		}
		row.Scope = *patch.Scope
	}
	if patch.Enabled != nil {
		row.Enabled = *patch.Enabled
	}
	if patch.Notes != nil {
		row.Notes = patch.Notes
	}
	if err := database.DB.Save(&row).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(row)
}

// deleteKey godoc
// @Summary Delete API key (admin)
// @Tags keys
// @Security ApiKeyAuth
// @Param id path int true "Key ID"
// @Success 204
// @Router /api/keys/{id} [delete]
func deleteKey(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("id")
	res := database.DB.Delete(&models.ApiKey{}, id)
	if res.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Key not found"})
	}
	return c.SendStatus(204)
}
