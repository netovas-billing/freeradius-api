package handlers

import (
	"github.com/gofiber/fiber/v2"

	"freeradius-api/database"
	"freeradius-api/models"
	"freeradius-api/schemas"
)

func RegisterAccounting(r fiber.Router) {
	g := r.Group("/accounting")
	g.Get("/sessions/active", activeSessions)
	g.Get("/sessions", listSessions)
	g.Get("/users/:username/usage", userUsage)
}

// activeSessions godoc
// @Summary List currently active sessions (no Acct-Stop yet)
// @Tags accounting
// @Security ApiKeyAuth
// @Produce json
// @Param limit query int false "Limit (max 1000)"
// @Success 200 {array} models.Radacct
// @Router /api/accounting/sessions/active [get]
func activeSessions(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 100)
	if limit > 1000 {
		limit = 1000
	}
	var rows []models.Radacct
	database.DB.
		Where("acctstoptime IS NULL").
		Order("acctstarttime DESC").
		Limit(limit).
		Find(&rows)
	return c.JSON(rows)
}

// listSessions godoc
// @Summary List session history
// @Tags accounting
// @Security ApiKeyAuth
// @Produce json
// @Param username query string false "Filter by username"
// @Param nasipaddress query string false "Filter by NAS IP"
// @Param limit query int false "Limit (max 1000)"
// @Param offset query int false "Offset"
// @Success 200 {array} models.Radacct
// @Router /api/accounting/sessions [get]
func listSessions(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 100)
	if limit > 1000 {
		limit = 1000
	}
	offset := c.QueryInt("offset", 0)
	username := c.Query("username")
	nasip := c.Query("nasipaddress")

	q := database.DB.Model(&models.Radacct{})
	if username != "" {
		q = q.Where("username = ?", username)
	}
	if nasip != "" {
		q = q.Where("nasipaddress = ?", nasip)
	}

	var rows []models.Radacct
	q.Order("acctstarttime DESC").Offset(offset).Limit(limit).Find(&rows)
	return c.JSON(rows)
}

// userUsage godoc
// @Summary Aggregated traffic & session count for a user
// @Tags accounting
// @Security ApiKeyAuth
// @Produce json
// @Param username path string true "Username"
// @Success 200 {object} schemas.UserUsage
// @Router /api/accounting/users/{username}/usage [get]
func userUsage(c *fiber.Ctx) error {
	username := c.Params("username")
	var result struct {
		SessionCount        int64 `gorm:"column:session_count"`
		TotalSessionSeconds int64 `gorm:"column:total_session_seconds"`
		InputBytes          int64 `gorm:"column:input_bytes"`
		OutputBytes         int64 `gorm:"column:output_bytes"`
	}
	database.DB.Model(&models.Radacct{}).Select(
		"COUNT(radacctid) AS session_count, " +
			"COALESCE(SUM(acctsessiontime),0) AS total_session_seconds, " +
			"COALESCE(SUM(acctinputoctets),0) AS input_bytes, " +
			"COALESCE(SUM(acctoutputoctets),0) AS output_bytes",
	).Where("username = ?", username).Scan(&result)

	return c.JSON(schemas.UserUsage{
		Username:            username,
		SessionCount:        result.SessionCount,
		TotalSessionSeconds: result.TotalSessionSeconds,
		InputBytes:          result.InputBytes,
		OutputBytes:         result.OutputBytes,
	})
}
