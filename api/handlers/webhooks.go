package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"freeradius-api/apierr"
	"freeradius-api/database"
	"freeradius-api/models"
	"freeradius-api/schemas"
)

func RegisterWebhooks(r fiber.Router) {
	g := r.Group("/webhooks")
	g.Get("", listWebhooks)
	g.Post("", createWebhook)
	g.Get("/:id", getWebhook)
	g.Put("/:id", updateWebhook)
	g.Delete("/:id", deleteWebhook)
	g.Get("/:id/deliveries", listDeliveries)
}

func validEvent(e string) bool {
	switch e {
	case "session_stop", "session_start", "auth_accept", "auth_reject":
		return true
	}
	return false
}

// listWebhooks godoc
// @Summary List webhook subscriptions (admin)
// @Tags webhooks
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Success 200 {array} models.Webhook
// @Router /api/webhooks [get]
func listWebhooks(c *fiber.Ctx) error {
	var rows []models.Webhook
	database.DB.Order("id").Find(&rows)
	return c.JSON(rows)
}

// createWebhook godoc
// @Summary Create webhook subscription (admin)
// @Description Event: session_stop | session_start | auth_accept | auth_reject.
// @Description Kalau secret kosong, di-generate random 32 byte hex; nilai dikembalikan SEKALI.
// @Description Outbound POST akan ditandatangani HMAC-SHA256 di header X-Signature.
// @Tags webhooks
// @Security ApiKeyAuth
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param body body schemas.WebhookCreate true "Subscription payload"
// @Success 201 {object} schemas.WebhookCreatedResponse
// @Router /api/webhooks [post]
func createWebhook(c *fiber.Ctx) error {
	var payload schemas.WebhookCreate
	if err := c.BodyParser(&payload); err != nil {
		return apierr.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if payload.Name == "" || payload.URL == "" {
		return apierr.BadRequest(c, "Name and url are required")
	}
	if !validEvent(payload.Event) {
		return apierr.BadRequest(c, "Event must be one of: session_stop, session_start, auth_accept, auth_reject")
	}

	secret := payload.Secret
	if secret == "" {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			return apierr.Internal(c, err.Error())
		}
		secret = hex.EncodeToString(buf)
	}

	cursor := computeCursor(payload.Event)

	row := models.Webhook{
		Name:    payload.Name,
		URL:     payload.URL,
		Event:   payload.Event,
		Secret:  secret,
		Enabled: true,
		Cursor:  cursor,
	}
	if err := database.DB.Create(&row).Error; err != nil {
		return apierr.Internal(c, err.Error())
	}
	return c.Status(201).JSON(schemas.WebhookCreatedResponse{
		ID:     row.ID,
		Name:   row.Name,
		Event:  row.Event,
		URL:    row.URL,
		Secret: secret,
	})
}

// computeCursor — initial cursor = current max ID for the event source table,
// supaya webhook hanya menerima event BARU setelah subscribe (bukan replay history).
func computeCursor(event string) uint64 {
	var max uint64
	switch event {
	case "session_stop", "session_start":
		database.DB.Model(&models.Radacct{}).Select("COALESCE(MAX(radacctid),0)").Scan(&max)
	case "auth_accept", "auth_reject":
		database.DB.Model(&models.Radpostauth{}).Select("COALESCE(MAX(id),0)").Scan(&max)
	}
	return max
}

// getWebhook godoc
// @Summary Get webhook detail (admin)
// @Tags webhooks
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param id path int true "Webhook ID"
// @Success 200 {object} models.Webhook
// @Router /api/webhooks/{id} [get]
func getWebhook(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("id")
	var row models.Webhook
	if err := database.DB.First(&row, id).Error; err != nil {
		return apierr.NotFound(c, "Webhook not found")
	}
	return c.JSON(row)
}

// updateWebhook godoc
// @Summary Update webhook (admin)
// @Tags webhooks
// @Security ApiKeyAuth
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param id path int true "Webhook ID"
// @Param body body schemas.WebhookUpdate true "Fields"
// @Success 200 {object} models.Webhook
// @Router /api/webhooks/{id} [put]
func updateWebhook(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("id")
	var row models.Webhook
	if err := database.DB.First(&row, id).Error; err != nil {
		return apierr.NotFound(c, "Webhook not found")
	}
	var patch schemas.WebhookUpdate
	if err := c.BodyParser(&patch); err != nil {
		return apierr.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if patch.URL != nil {
		row.URL = *patch.URL
	}
	if patch.Event != nil {
		if !validEvent(*patch.Event) {
			return apierr.BadRequest(c, "Invalid event")
		}
		row.Event = *patch.Event
		row.Cursor = computeCursor(row.Event)
	}
	if patch.Secret != nil {
		row.Secret = *patch.Secret
	}
	if patch.Enabled != nil {
		row.Enabled = *patch.Enabled
	}
	if err := database.DB.Save(&row).Error; err != nil {
		return apierr.Internal(c, err.Error())
	}
	return c.JSON(row)
}

// deleteWebhook godoc
// @Summary Delete webhook (admin)
// @Tags webhooks
// @Security ApiKeyAuth
// @Security BasicAuth
// @Param id path int true "Webhook ID"
// @Success 204
// @Router /api/webhooks/{id} [delete]
func deleteWebhook(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("id")
	res := database.DB.Delete(&models.Webhook{}, id)
	if res.RowsAffected == 0 {
		return apierr.NotFound(c, "Webhook not found")
	}
	database.DB.Where("webhook_id = ?", id).Delete(&models.WebhookDelivery{})
	return c.SendStatus(204)
}

// listDeliveries godoc
// @Summary List delivery history for a webhook (admin)
// @Tags webhooks
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param id path int true "Webhook ID"
// @Param limit query int false "Limit (max 1000)"
// @Param offset query int false "Offset"
// @Success 200 {array} models.WebhookDelivery
// @Header 200 {integer} X-Total-Count "total deliveries for this webhook"
// @Router /api/webhooks/{id}/deliveries [get]
func listDeliveries(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("id")
	limit := c.QueryInt("limit", 100)
	if limit > 1000 {
		limit = 1000
	}
	offset := c.QueryInt("offset", 0)

	q := database.DB.Model(&models.WebhookDelivery{}).Where("webhook_id = ?", id)
	var total int64
	q.Count(&total)
	c.Set("X-Total-Count", strconv.FormatInt(total, 10))

	var rows []models.WebhookDelivery
	q.Order("timestamp DESC").Offset(offset).Limit(limit).Find(&rows)
	return c.JSON(rows)
}
