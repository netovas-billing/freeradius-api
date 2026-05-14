package handlers

import (
	"github.com/gofiber/fiber/v2"

	"freeradius-api/database"
	"freeradius-api/models"
	"freeradius-api/schemas"
)

func RegisterNAS(r fiber.Router) {
	g := r.Group("/nas")
	g.Get("", listNAS)
	g.Post("", createNAS)
	g.Get("/:id", getNAS)
	g.Put("/:id", updateNAS)
	g.Delete("/:id", deleteNAS)
}

// listNAS godoc
// @Summary List NAS
// @Tags nas
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {array} models.NAS
// @Router /api/nas [get]
func listNAS(c *fiber.Ctx) error {
	var nas []models.NAS
	database.DB.Order("id").Find(&nas)
	return c.JSON(nas)
}

// createNAS godoc
// @Summary Create NAS entry
// @Tags nas
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param body body schemas.NASCreate true "NAS payload"
// @Success 201 {object} models.NAS
// @Router /api/nas [post]
func createNAS(c *fiber.Ctx) error {
	var payload schemas.NASCreate
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if payload.NASName == "" || payload.Secret == "" {
		return c.Status(400).JSON(fiber.Map{"error": "nasname and secret required"})
	}
	n := models.NAS{
		NASName: payload.NASName,
		Secret:  payload.Secret,
		Ports:   payload.Ports,
	}
	t := payload.Type
	if t == "" {
		t = "other"
	}
	n.Type = &t
	if payload.ShortName != "" {
		s := payload.ShortName
		n.ShortName = &s
	}
	if payload.Server != "" {
		s := payload.Server
		n.Server = &s
	}
	if payload.Community != "" {
		s := payload.Community
		n.Community = &s
	}
	if payload.Description != "" {
		s := payload.Description
		n.Description = &s
	}
	if err := database.DB.Create(&n).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(201).JSON(n)
}

// getNAS godoc
// @Summary Get NAS detail
// @Tags nas
// @Security ApiKeyAuth
// @Produce json
// @Param id path int true "NAS ID"
// @Success 200 {object} models.NAS
// @Failure 404 {object} schemas.ErrorResponse
// @Router /api/nas/{id} [get]
func getNAS(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("id")
	var n models.NAS
	if err := database.DB.First(&n, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "NAS not found"})
	}
	return c.JSON(n)
}

// updateNAS godoc
// @Summary Update NAS (partial)
// @Tags nas
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path int true "NAS ID"
// @Param body body schemas.NASUpdate true "Fields to update"
// @Success 200 {object} models.NAS
// @Failure 404 {object} schemas.ErrorResponse
// @Router /api/nas/{id} [put]
func updateNAS(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("id")
	var n models.NAS
	if err := database.DB.First(&n, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "NAS not found"})
	}
	var patch schemas.NASUpdate
	if err := c.BodyParser(&patch); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if patch.NASName != nil {
		n.NASName = *patch.NASName
	}
	if patch.ShortName != nil {
		n.ShortName = patch.ShortName
	}
	if patch.Type != nil {
		n.Type = patch.Type
	}
	if patch.Ports != nil {
		n.Ports = patch.Ports
	}
	if patch.Secret != nil {
		n.Secret = *patch.Secret
	}
	if patch.Server != nil {
		n.Server = patch.Server
	}
	if patch.Community != nil {
		n.Community = patch.Community
	}
	if patch.Description != nil {
		n.Description = patch.Description
	}
	if err := database.DB.Save(&n).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(n)
}

// deleteNAS godoc
// @Summary Delete NAS
// @Tags nas
// @Security ApiKeyAuth
// @Param id path int true "NAS ID"
// @Success 204
// @Failure 404 {object} schemas.ErrorResponse
// @Router /api/nas/{id} [delete]
func deleteNAS(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("id")
	res := database.DB.Delete(&models.NAS{}, id)
	if res.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "NAS not found"})
	}
	return c.SendStatus(204)
}
