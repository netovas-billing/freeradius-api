package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"freeradius-api/apierr"
	"freeradius-api/database"
	"freeradius-api/models"
)

func RegisterAudit(r fiber.Router) {
	g := r.Group("/audit")
	g.Get("", listAudit)
	g.Delete("/before", purgeAudit)
}

// listAudit godoc
// @Summary List API audit log (admin)
// @Description Filter by api_key_name, method, status (exact / range via from/to=200..299),
// @Description path substring, time window (RFC3339). Sets X-Total-Count.
// @Tags audit
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param api_key_name query string false "Filter by key name"
// @Param method query string false "GET|POST|PUT|DELETE"
// @Param status query int false "HTTP status exact match"
// @Param status_min query int false "HTTP status >= (used with status_max)"
// @Param status_max query int false "HTTP status <="
// @Param path query string false "Path substring (LIKE %path%)"
// @Param from query string false "RFC3339"
// @Param to query string false "RFC3339"
// @Param limit query int false "Limit (max 1000)"
// @Param offset query int false "Offset"
// @Success 200 {array} models.ApiAuditLog
// @Header 200 {integer} X-Total-Count "total matching rows"
// @Router /api/audit [get]
func listAudit(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 100)
	if limit > 1000 {
		limit = 1000
	}
	offset := c.QueryInt("offset", 0)

	q := database.DB.Model(&models.ApiAuditLog{})

	if v := c.Query("api_key_name"); v != "" {
		q = q.Where("api_key_name = ?", v)
	}
	if v := c.Query("method"); v != "" {
		q = q.Where("method = ?", v)
	}
	if s := c.QueryInt("status", 0); s > 0 {
		q = q.Where("status = ?", s)
	}
	if smin := c.QueryInt("status_min", 0); smin > 0 {
		q = q.Where("status >= ?", smin)
	}
	if smax := c.QueryInt("status_max", 0); smax > 0 {
		q = q.Where("status <= ?", smax)
	}
	if v := c.Query("path"); v != "" {
		q = q.Where("path LIKE ?", "%"+v+"%")
	}
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q = q.Where("timestamp >= ?", t)
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q = q.Where("timestamp <= ?", t)
		}
	}

	var total int64
	q.Count(&total)
	c.Set("X-Total-Count", strconv.FormatInt(total, 10))

	var rows []models.ApiAuditLog
	q.Order("timestamp DESC").Offset(offset).Limit(limit).Find(&rows)
	return c.JSON(rows)
}

// purgeAudit godoc
// @Summary Purge audit log older than `before` (admin)
// @Tags audit
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param before query string true "RFC3339, hapus baris dengan timestamp < before"
// @Success 200 {object} map[string]int64
// @Router /api/audit/before [delete]
func purgeAudit(c *fiber.Ctx) error {
	before := c.Query("before")
	if before == "" {
		return apierr.BadRequest(c, "Query param `before` is required (RFC3339)")
	}
	t, err := time.Parse(time.RFC3339, before)
	if err != nil {
		return apierr.BadRequest(c, "Invalid RFC3339 timestamp")
	}
	res := database.DB.Where("timestamp < ?", t).Delete(&models.ApiAuditLog{})
	return c.JSON(fiber.Map{"deleted": res.RowsAffected})
}
