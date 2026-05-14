package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"freeradius-api/database"
	"freeradius-api/models"
)

func RegisterPostAuth(r fiber.Router) {
	g := r.Group("/postauth")
	g.Get("", listPostAuth)
}

// listPostAuth godoc
// @Summary List post-authentication log (radpostauth)
// @Description Filter by username (exact), reply (Access-Accept|Access-Reject), class,
// @Description and time window (from/to RFC3339). Sets X-Total-Count header.
// @Tags postauth
// @Security ApiKeyAuth
// @Produce json
// @Param username query string false "Filter by username"
// @Param reply query string false "Access-Accept or Access-Reject"
// @Param class query string false "Filter by class"
// @Param from query string false "RFC3339 (e.g. 2026-05-14T00:00:00Z)"
// @Param to query string false "RFC3339"
// @Param limit query int false "Limit (max 1000)"
// @Param offset query int false "Offset"
// @Success 200 {array} models.Radpostauth
// @Header 200 {integer} X-Total-Count "total matching rows"
// @Router /api/postauth [get]
func listPostAuth(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 100)
	if limit > 1000 {
		limit = 1000
	}
	offset := c.QueryInt("offset", 0)

	q := database.DB.Model(&models.Radpostauth{})
	q = applyPostAuthFilters(q, c)

	var total int64
	q.Count(&total)
	c.Set("X-Total-Count", strconv.FormatInt(total, 10))

	var rows []models.Radpostauth
	q.Order("authdate DESC").Offset(offset).Limit(limit).Find(&rows)
	return c.JSON(rows)
}

func applyPostAuthFilters(q *gorm.DB, c *fiber.Ctx) *gorm.DB {
	if v := c.Query("username"); v != "" {
		q = q.Where("username = ?", v)
	}
	if v := c.Query("reply"); v != "" {
		q = q.Where("reply = ?", v)
	}
	if v := c.Query("class"); v != "" {
		q = q.Where("class = ?", v)
	}
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q = q.Where("authdate >= ?", t)
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q = q.Where("authdate <= ?", t)
		}
	}
	return q
}
