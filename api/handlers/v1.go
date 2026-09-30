package handlers

// Legacy /api/v1/* — kompatibilitas shape FastAPI lama (nasvpntest-api).
// Row-level CRUD, response field id di akhir, NAS expose `secret`.

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"layeh.com/radius"

	"freeradius-api/apierr"
	"freeradius-api/database"
	"freeradius-api/models"
	"freeradius-api/schemas"
)

func RegisterV1(r fiber.Router) {
	v := r.Group("/v1")

	// users (radcheck row level)
	v.Get("/users", v1ListUsers)
	v.Post("/users", v1CreateUser)
	v.Get("/users/:username", v1GetUser)
	v.Put("/users/:user_id", v1UpdateUser)
	v.Delete("/users/:user_id", v1DeleteUser)

	// radusergroup
	v.Get("/user-group/", v1ListUserGroup)
	v.Post("/user-group/", v1CreateUserGroup)
	v.Put("/user-group/:group_id", v1UpdateUserGroup)
	v.Delete("/user-group/:group_id", v1DeleteUserGroup)

	// radgroupreply ("group")
	v.Get("/group/", v1ListGroupReply)
	v.Post("/group/", v1CreateGroupReply)
	v.Put("/group/:group_id", v1UpdateGroupReply)
	v.Delete("/group/:group_id", v1DeleteGroupReply)

	// radgroupcheck ("group-check")
	v.Get("/group-check/", v1ListGroupCheck)
	v.Post("/group-check/", v1CreateGroupCheck)
	v.Put("/group-check/:group_id", v1UpdateGroupCheck)
	v.Delete("/group-check/:group_id", v1DeleteGroupCheck)

	// radreply
	v.Get("/reply/", v1ListReply)
	v.Post("/reply/", v1CreateReply)
	v.Put("/reply/:reply_id", v1UpdateReply)
	v.Delete("/reply/:reply_id", v1DeleteReply)

	// nas
	v.Get("/nas/", v1ListNas)
	v.Post("/nas/", v1CreateNas)
	v.Get("/nas/:nas_id", v1GetNas)
	v.Put("/nas/:nas_id", v1UpdateNas)
	v.Delete("/nas/:nas_id", v1DeleteNas)

	// radacct (read-only)
	v.Get("/radacct/", v1ListRadacct)
	v.Get("/radacct/:radacctid", v1GetRadacct)
	v.Get("/radacct/status/:username", v1UserStatus)

	// disconnect
	v.Post("/disconnect", v1Disconnect)
}

// helpers

func paging(c *fiber.Ctx) (int, int) {
	skip := c.QueryInt("skip", 0)
	limit := c.QueryInt("limit", 100)
	if limit > 1000 {
		limit = 1000
	}
	return skip, limit
}

func rcOut(r models.Radcheck) schemas.V1Radcheck {
	return schemas.V1Radcheck{Username: r.Username, Attribute: r.Attribute, Op: r.Op, Value: r.Value, ID: r.ID}
}
func rrOut(r models.Radreply) schemas.V1Radreply {
	return schemas.V1Radreply{Username: r.Username, Attribute: r.Attribute, Op: r.Op, Value: r.Value, ID: r.ID}
}
func rgcOut(r models.Radgroupcheck) schemas.V1Radgroupcheck {
	return schemas.V1Radgroupcheck{Groupname: r.Groupname, Attribute: r.Attribute, Op: r.Op, Value: r.Value, ID: r.ID}
}
func rgrOut(r models.Radgroupreply) schemas.V1Radgroupreply {
	return schemas.V1Radgroupreply{Groupname: r.Groupname, Attribute: r.Attribute, Op: r.Op, Value: r.Value, ID: r.ID}
}
func rugOut(r models.Radusergroup) schemas.V1Radusergroup {
	return schemas.V1Radusergroup{Username: r.Username, Groupname: r.Groupname, Priority: r.Priority, ID: r.ID}
}
func nasOut(n models.NAS) schemas.V1NAS {
	out := schemas.V1NAS{
		NASName: n.NASName, Secret: n.Secret, ID: n.ID,
	}
	if n.ShortName != nil {
		out.ShortName = *n.ShortName
	}
	if n.Type != nil {
		out.Type = *n.Type
	}
	if n.Ports != nil {
		out.Ports = *n.Ports
	}
	if n.Server != nil {
		out.Server = *n.Server
	}
	if n.Community != nil {
		out.Community = *n.Community
	}
	if n.Description != nil {
		out.Description = *n.Description
	}
	return out
}
func acctOut(r models.Radacct) schemas.V1Radacct {
	out := schemas.V1Radacct{
		AcctSessionID: r.AcctSessionID, AcctUniqueID: r.AcctUniqueID, Username: r.Username,
		NASIPAddress: r.NASIPAddress, CalledStationID: r.CalledStationID, CallingStationID: r.CallingStationID,
		AcctTerminateCause: r.AcctTerminateCause, FramedIPAddress: r.FramedIPAddress, RadAcctID: r.RadAcctID,
	}
	if r.Realm != nil {
		out.Realm = *r.Realm
	}
	if r.NASPortID != nil {
		out.NASPortID = *r.NASPortID
	}
	if r.NASPortType != nil {
		out.NASPortType = *r.NASPortType
	}
	if r.AcctStartTime != nil {
		s := r.AcctStartTime.UTC().Format(time.RFC3339Nano)
		out.AcctStartTime = &s
	}
	if r.AcctUpdateTime != nil {
		s := r.AcctUpdateTime.UTC().Format(time.RFC3339Nano)
		out.AcctUpdateTime = &s
	}
	if r.AcctStopTime != nil {
		s := r.AcctStopTime.UTC().Format(time.RFC3339Nano)
		out.AcctStopTime = &s
	}
	if r.AcctInterval != nil {
		out.AcctInterval = *r.AcctInterval
	}
	if r.AcctSessionTime != nil {
		out.AcctSessionTime = *r.AcctSessionTime
	}
	if r.AcctAuthentic != nil {
		out.AcctAuthentic = *r.AcctAuthentic
	}
	if r.ConnectInfoStart != nil {
		out.ConnectInfoStart = *r.ConnectInfoStart
	}
	if r.ConnectInfoStop != nil {
		out.ConnectInfoStop = *r.ConnectInfoStop
	}
	if r.AcctInputOctets != nil {
		out.AcctInputOctets = *r.AcctInputOctets
	}
	if r.AcctOutputOctets != nil {
		out.AcctOutputOctets = *r.AcctOutputOctets
	}
	if r.ServiceType != nil {
		out.ServiceType = *r.ServiceType
	}
	if r.FramedProtocol != nil {
		out.FramedProtocol = *r.FramedProtocol
	}
	return out
}

// ===== USERS (radcheck) =====

// v1ListUsers godoc
// @Summary Get Users (radcheck rows)
// @Tags v1-users
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param username query string false "Filter by username"
// @Param attribute query string false "Filter by attribute"
// @Param skip query int false "Skip"
// @Param limit query int false "Limit (max 1000)"
// @Success 200 {array} schemas.V1Radcheck
// @Router /api/v1/users [get]
func v1ListUsers(c *fiber.Ctx) error {
	skip, limit := paging(c)
	q := database.DB.Model(&models.Radcheck{})
	if v := c.Query("username"); v != "" {
		q = q.Where("username = ?", v)
	}
	if v := c.Query("attribute"); v != "" {
		q = q.Where("attribute = ?", v)
	}
	var total int64
	q.Count(&total)
	c.Set("X-Total-Count", strconv.FormatInt(total, 10))

	var rows []models.Radcheck
	q.Order("id").Offset(skip).Limit(limit).Find(&rows)
	out := make([]schemas.V1Radcheck, 0, len(rows))
	for _, r := range rows {
		out = append(out, rcOut(r))
	}
	return c.JSON(out)
}

// v1CreateUser godoc
// @Summary Create User (insert radcheck row)
// @Tags v1-users
// @Security ApiKeyAuth
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param body body schemas.V1RowIn true "Radcheck row"
// @Success 201 {object} schemas.V1Radcheck
// @Router /api/v1/users [post]
func v1CreateUser(c *fiber.Ctx) error {
	var p schemas.V1RowIn
	if err := c.BodyParser(&p); err != nil {
		return apierr.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if err := apierr.MissingFields(c, "username", p.Username, "attribute", p.Attribute); err != nil {
		return nil
	}
	if p.Op == "" {
		p.Op = ":="
	}
	row := models.Radcheck{Username: p.Username, Attribute: p.Attribute, Op: p.Op, Value: p.Value}
	if err := database.DB.Create(&row).Error; err != nil {
		return apierr.Internal(c, err.Error())
	}
	return c.Status(201).JSON(rcOut(row))
}

// v1GetUser godoc
// @Summary Get User (all radcheck rows for given username)
// @Tags v1-users
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param username path string true "Username"
// @Success 200 {array} schemas.V1Radcheck
// @Router /api/v1/users/{username} [get]
func v1GetUser(c *fiber.Ctx) error {
	username := c.Params("username")
	var rows []models.Radcheck
	database.DB.Where("username = ?", username).Order("id").Find(&rows)
	if len(rows) == 0 {
		return apierr.NotFound(c, "User not found")
	}
	out := make([]schemas.V1Radcheck, 0, len(rows))
	for _, r := range rows {
		out = append(out, rcOut(r))
	}
	return c.JSON(out)
}

// v1UpdateUser godoc
// @Summary Update User (radcheck row by id)
// @Tags v1-users
// @Security ApiKeyAuth
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param user_id path int true "Radcheck row id"
// @Param body body schemas.V1RowIn true "Fields to set"
// @Success 200 {object} schemas.V1Radcheck
// @Router /api/v1/users/{user_id} [put]
func v1UpdateUser(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("user_id")
	var row models.Radcheck
	if err := database.DB.First(&row, id).Error; err != nil {
		return apierr.NotFound(c, "Row not found")
	}
	var p schemas.V1RowIn
	if err := c.BodyParser(&p); err != nil {
		return apierr.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if p.Username != "" {
		row.Username = p.Username
	}
	if p.Attribute != "" {
		row.Attribute = p.Attribute
	}
	if p.Op != "" {
		row.Op = p.Op
	}
	if p.Value != "" {
		row.Value = p.Value
	}
	if err := database.DB.Save(&row).Error; err != nil {
		return apierr.Internal(c, err.Error())
	}
	return c.JSON(rcOut(row))
}

// v1DeleteUser godoc
// @Summary Delete User (radcheck row by id)
// @Tags v1-users
// @Security ApiKeyAuth
// @Security BasicAuth
// @Param user_id path int true "Radcheck row id"
// @Success 204
// @Router /api/v1/users/{user_id} [delete]
func v1DeleteUser(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("user_id")
	res := database.DB.Delete(&models.Radcheck{}, id)
	if res.RowsAffected == 0 {
		return apierr.NotFound(c, "Row not found")
	}
	return c.SendStatus(204)
}

// ===== RADUSERGROUP =====

// v1ListUserGroup godoc
// @Summary Get Radusergroup List
// @Tags v1-radusergroup
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param username query string false "Filter by username"
// @Param groupname query string false "Filter by group name"
// @Param skip query int false "Skip"
// @Param limit query int false "Limit (max 1000)"
// @Success 200 {array} schemas.V1Radusergroup
// @Router /api/v1/user-group/ [get]
func v1ListUserGroup(c *fiber.Ctx) error {
	skip, limit := paging(c)
	q := database.DB.Model(&models.Radusergroup{})
	if v := c.Query("username"); v != "" {
		q = q.Where("username = ?", v)
	}
	if v := c.Query("groupname"); v != "" {
		q = q.Where("groupname = ?", v)
	}
	var total int64
	q.Count(&total)
	c.Set("X-Total-Count", strconv.FormatInt(total, 10))

	var rows []models.Radusergroup
	q.Order("id").Offset(skip).Limit(limit).Find(&rows)
	out := make([]schemas.V1Radusergroup, 0, len(rows))
	for _, r := range rows {
		out = append(out, rugOut(r))
	}
	return c.JSON(out)
}

// v1CreateUserGroup godoc
// @Summary Create Radusergroup
// @Tags v1-radusergroup
// @Security ApiKeyAuth
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param body body schemas.V1RowIn true "Radusergroup row {username, groupname, priority}"
// @Success 201 {object} schemas.V1Radusergroup
// @Router /api/v1/user-group/ [post]
func v1CreateUserGroup(c *fiber.Ctx) error {
	var p schemas.V1RowIn
	if err := c.BodyParser(&p); err != nil {
		return apierr.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if err := apierr.MissingFields(c, "username", p.Username, "groupname", p.Groupname); err != nil {
		return nil
	}
	pri := 1
	if p.Priority != nil {
		pri = *p.Priority
	}
	row := models.Radusergroup{Username: p.Username, Groupname: p.Groupname, Priority: pri}
	if err := database.DB.Create(&row).Error; err != nil {
		return apierr.Internal(c, err.Error())
	}
	return c.Status(201).JSON(rugOut(row))
}

// v1UpdateUserGroup godoc
// @Summary Update Radusergroup
// @Tags v1-radusergroup
// @Security ApiKeyAuth
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param group_id path int true "Row id"
// @Param body body schemas.V1RowIn true "Fields"
// @Success 200 {object} schemas.V1Radusergroup
// @Router /api/v1/user-group/{group_id} [put]
func v1UpdateUserGroup(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("group_id")
	var row models.Radusergroup
	if err := database.DB.First(&row, id).Error; err != nil {
		return apierr.NotFound(c, "Row not found")
	}
	var p schemas.V1RowIn
	if err := c.BodyParser(&p); err != nil {
		return apierr.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if p.Username != "" {
		row.Username = p.Username
	}
	if p.Groupname != "" {
		row.Groupname = p.Groupname
	}
	if p.Priority != nil {
		row.Priority = *p.Priority
	}
	if err := database.DB.Save(&row).Error; err != nil {
		return apierr.Internal(c, err.Error())
	}
	return c.JSON(rugOut(row))
}

// v1DeleteUserGroup godoc
// @Summary Delete Radusergroup
// @Tags v1-radusergroup
// @Security ApiKeyAuth
// @Security BasicAuth
// @Param group_id path int true "Row id"
// @Success 204
// @Router /api/v1/user-group/{group_id} [delete]
func v1DeleteUserGroup(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("group_id")
	res := database.DB.Delete(&models.Radusergroup{}, id)
	if res.RowsAffected == 0 {
		return apierr.NotFound(c, "Row not found")
	}
	return c.SendStatus(204)
}

// ===== RADGROUPREPLY =====

// v1ListGroupReply godoc
// @Summary Get Radgroupreply List
// @Tags v1-radgroupreply
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param name query string false "Filter by group name"
// @Param skip query int false "Skip"
// @Param limit query int false "Limit (max 1000)"
// @Success 200 {array} schemas.V1Radgroupreply
// @Router /api/v1/group/ [get]
func v1ListGroupReply(c *fiber.Ctx) error {
	skip, limit := paging(c)
	q := database.DB.Model(&models.Radgroupreply{})
	if v := c.Query("name"); v != "" {
		q = q.Where("groupname = ?", v)
	}
	var total int64
	q.Count(&total)
	c.Set("X-Total-Count", strconv.FormatInt(total, 10))

	var rows []models.Radgroupreply
	q.Order("id").Offset(skip).Limit(limit).Find(&rows)
	out := make([]schemas.V1Radgroupreply, 0, len(rows))
	for _, r := range rows {
		out = append(out, rgrOut(r))
	}
	return c.JSON(out)
}

// v1CreateGroupReply godoc
// @Summary Create Radgroupreply
// @Tags v1-radgroupreply
// @Security ApiKeyAuth
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param body body schemas.V1RowIn true "Row {groupname, attribute, op, value}"
// @Success 201 {object} schemas.V1Radgroupreply
// @Router /api/v1/group/ [post]
func v1CreateGroupReply(c *fiber.Ctx) error {
	var p schemas.V1RowIn
	if err := c.BodyParser(&p); err != nil {
		return apierr.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if err := apierr.MissingFields(c, "groupname", p.Groupname, "attribute", p.Attribute); err != nil {
		return nil
	}
	if p.Op == "" {
		p.Op = ":="
	}
	row := models.Radgroupreply{Groupname: p.Groupname, Attribute: p.Attribute, Op: p.Op, Value: p.Value}
	if err := database.DB.Create(&row).Error; err != nil {
		return apierr.Internal(c, err.Error())
	}
	return c.Status(201).JSON(rgrOut(row))
}

// v1UpdateGroupReply godoc
// @Summary Update Radgroupreply
// @Tags v1-radgroupreply
// @Security ApiKeyAuth
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param group_id path int true "Row id"
// @Param body body schemas.V1RowIn true "Fields"
// @Success 200 {object} schemas.V1Radgroupreply
// @Router /api/v1/group/{group_id} [put]
func v1UpdateGroupReply(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("group_id")
	var row models.Radgroupreply
	if err := database.DB.First(&row, id).Error; err != nil {
		return apierr.NotFound(c, "Row not found")
	}
	var p schemas.V1RowIn
	if err := c.BodyParser(&p); err != nil {
		return apierr.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if p.Groupname != "" {
		row.Groupname = p.Groupname
	}
	if p.Attribute != "" {
		row.Attribute = p.Attribute
	}
	if p.Op != "" {
		row.Op = p.Op
	}
	if p.Value != "" {
		row.Value = p.Value
	}
	if err := database.DB.Save(&row).Error; err != nil {
		return apierr.Internal(c, err.Error())
	}
	return c.JSON(rgrOut(row))
}

// v1DeleteGroupReply godoc
// @Summary Delete Radgroupreply
// @Tags v1-radgroupreply
// @Security ApiKeyAuth
// @Security BasicAuth
// @Param group_id path int true "Row id"
// @Success 204
// @Router /api/v1/group/{group_id} [delete]
func v1DeleteGroupReply(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("group_id")
	res := database.DB.Delete(&models.Radgroupreply{}, id)
	if res.RowsAffected == 0 {
		return apierr.NotFound(c, "Row not found")
	}
	return c.SendStatus(204)
}

// ===== RADGROUPCHECK =====

// v1ListGroupCheck godoc
// @Summary Get Radgroupcheck List
// @Tags v1-radgroupcheck
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param name query string false "Filter by group name"
// @Param skip query int false "Skip"
// @Param limit query int false "Limit (max 1000)"
// @Success 200 {array} schemas.V1Radgroupcheck
// @Router /api/v1/group-check/ [get]
func v1ListGroupCheck(c *fiber.Ctx) error {
	skip, limit := paging(c)
	q := database.DB.Model(&models.Radgroupcheck{})
	if v := c.Query("name"); v != "" {
		q = q.Where("groupname = ?", v)
	}
	var total int64
	q.Count(&total)
	c.Set("X-Total-Count", strconv.FormatInt(total, 10))

	var rows []models.Radgroupcheck
	q.Order("id").Offset(skip).Limit(limit).Find(&rows)
	out := make([]schemas.V1Radgroupcheck, 0, len(rows))
	for _, r := range rows {
		out = append(out, rgcOut(r))
	}
	return c.JSON(out)
}

// v1CreateGroupCheck godoc
// @Summary Create Radgroupcheck
// @Tags v1-radgroupcheck
// @Security ApiKeyAuth
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param body body schemas.V1RowIn true "Row {groupname, attribute, op, value}"
// @Success 201 {object} schemas.V1Radgroupcheck
// @Router /api/v1/group-check/ [post]
func v1CreateGroupCheck(c *fiber.Ctx) error {
	var p schemas.V1RowIn
	if err := c.BodyParser(&p); err != nil {
		return apierr.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if err := apierr.MissingFields(c, "groupname", p.Groupname, "attribute", p.Attribute); err != nil {
		return nil
	}
	if p.Op == "" {
		p.Op = ":="
	}
	row := models.Radgroupcheck{Groupname: p.Groupname, Attribute: p.Attribute, Op: p.Op, Value: p.Value}
	if err := database.DB.Create(&row).Error; err != nil {
		return apierr.Internal(c, err.Error())
	}
	return c.Status(201).JSON(rgcOut(row))
}

// v1UpdateGroupCheck godoc
// @Summary Update Radgroupcheck
// @Tags v1-radgroupcheck
// @Security ApiKeyAuth
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param group_id path int true "Row id"
// @Param body body schemas.V1RowIn true "Fields"
// @Success 200 {object} schemas.V1Radgroupcheck
// @Router /api/v1/group-check/{group_id} [put]
func v1UpdateGroupCheck(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("group_id")
	var row models.Radgroupcheck
	if err := database.DB.First(&row, id).Error; err != nil {
		return apierr.NotFound(c, "Row not found")
	}
	var p schemas.V1RowIn
	if err := c.BodyParser(&p); err != nil {
		return apierr.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if p.Groupname != "" {
		row.Groupname = p.Groupname
	}
	if p.Attribute != "" {
		row.Attribute = p.Attribute
	}
	if p.Op != "" {
		row.Op = p.Op
	}
	if p.Value != "" {
		row.Value = p.Value
	}
	if err := database.DB.Save(&row).Error; err != nil {
		return apierr.Internal(c, err.Error())
	}
	return c.JSON(rgcOut(row))
}

// v1DeleteGroupCheck godoc
// @Summary Delete Radgroupcheck
// @Tags v1-radgroupcheck
// @Security ApiKeyAuth
// @Security BasicAuth
// @Param group_id path int true "Row id"
// @Success 204
// @Router /api/v1/group-check/{group_id} [delete]
func v1DeleteGroupCheck(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("group_id")
	res := database.DB.Delete(&models.Radgroupcheck{}, id)
	if res.RowsAffected == 0 {
		return apierr.NotFound(c, "Row not found")
	}
	return c.SendStatus(204)
}

// ===== RADREPLY =====

// v1ListReply godoc
// @Summary Get Radreply List
// @Tags v1-radreply
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param username query string false "Filter by username"
// @Param attribute query string false "Filter by attribute"
// @Param skip query int false "Skip"
// @Param limit query int false "Limit (max 1000)"
// @Success 200 {array} schemas.V1Radreply
// @Router /api/v1/reply/ [get]
func v1ListReply(c *fiber.Ctx) error {
	skip, limit := paging(c)
	q := database.DB.Model(&models.Radreply{})
	if v := c.Query("username"); v != "" {
		q = q.Where("username = ?", v)
	}
	if v := c.Query("attribute"); v != "" {
		q = q.Where("attribute = ?", v)
	}
	var total int64
	q.Count(&total)
	c.Set("X-Total-Count", strconv.FormatInt(total, 10))

	var rows []models.Radreply
	q.Order("id").Offset(skip).Limit(limit).Find(&rows)
	out := make([]schemas.V1Radreply, 0, len(rows))
	for _, r := range rows {
		out = append(out, rrOut(r))
	}
	return c.JSON(out)
}

// v1CreateReply godoc
// @Summary Create Radreply
// @Tags v1-radreply
// @Security ApiKeyAuth
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param body body schemas.V1RowIn true "Row {username, attribute, op, value}"
// @Success 201 {object} schemas.V1Radreply
// @Router /api/v1/reply/ [post]
func v1CreateReply(c *fiber.Ctx) error {
	var p schemas.V1RowIn
	if err := c.BodyParser(&p); err != nil {
		return apierr.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if err := apierr.MissingFields(c, "username", p.Username, "attribute", p.Attribute); err != nil {
		return nil
	}
	if p.Op == "" {
		p.Op = ":="
	}
	row := models.Radreply{Username: p.Username, Attribute: p.Attribute, Op: p.Op, Value: p.Value}
	if err := database.DB.Create(&row).Error; err != nil {
		return apierr.Internal(c, err.Error())
	}
	return c.Status(201).JSON(rrOut(row))
}

// v1UpdateReply godoc
// @Summary Update Radreply
// @Tags v1-radreply
// @Security ApiKeyAuth
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param reply_id path int true "Row id"
// @Param body body schemas.V1RowIn true "Fields"
// @Success 200 {object} schemas.V1Radreply
// @Router /api/v1/reply/{reply_id} [put]
func v1UpdateReply(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("reply_id")
	var row models.Radreply
	if err := database.DB.First(&row, id).Error; err != nil {
		return apierr.NotFound(c, "Row not found")
	}
	var p schemas.V1RowIn
	if err := c.BodyParser(&p); err != nil {
		return apierr.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if p.Username != "" {
		row.Username = p.Username
	}
	if p.Attribute != "" {
		row.Attribute = p.Attribute
	}
	if p.Op != "" {
		row.Op = p.Op
	}
	if p.Value != "" {
		row.Value = p.Value
	}
	if err := database.DB.Save(&row).Error; err != nil {
		return apierr.Internal(c, err.Error())
	}
	return c.JSON(rrOut(row))
}

// v1DeleteReply godoc
// @Summary Delete Radreply
// @Tags v1-radreply
// @Security ApiKeyAuth
// @Security BasicAuth
// @Param reply_id path int true "Row id"
// @Success 204
// @Router /api/v1/reply/{reply_id} [delete]
func v1DeleteReply(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("reply_id")
	res := database.DB.Delete(&models.Radreply{}, id)
	if res.RowsAffected == 0 {
		return apierr.NotFound(c, "Row not found")
	}
	return c.SendStatus(204)
}

// ===== NAS =====

// v1ListNas godoc
// @Summary Get Nas List
// @Tags v1-nas
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param nasname query string false "Filter by NAS name"
// @Param type query string false "Filter by NAS type"
// @Param skip query int false "Skip"
// @Param limit query int false "Limit (max 1000)"
// @Success 200 {array} schemas.V1NAS
// @Router /api/v1/nas/ [get]
func v1ListNas(c *fiber.Ctx) error {
	skip, limit := paging(c)
	q := database.DB.Model(&models.NAS{})
	if v := c.Query("nasname"); v != "" {
		q = q.Where("nasname LIKE ?", "%"+v+"%")
	}
	if v := c.Query("type"); v != "" {
		q = q.Where("type = ?", v)
	}
	var total int64
	q.Count(&total)
	c.Set("X-Total-Count", strconv.FormatInt(total, 10))

	var rows []models.NAS
	q.Order("id").Offset(skip).Limit(limit).Find(&rows)
	out := make([]schemas.V1NAS, 0, len(rows))
	for _, r := range rows {
		out = append(out, nasOut(r))
	}
	return c.JSON(out)
}

// v1CreateNas godoc
// @Summary Create Nas
// @Tags v1-nas
// @Security ApiKeyAuth
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param body body schemas.V1NASCreate true "NAS"
// @Success 201 {object} schemas.V1NAS
// @Router /api/v1/nas/ [post]
func v1CreateNas(c *fiber.Ctx) error {
	var p schemas.V1NASCreate
	if err := c.BodyParser(&p); err != nil {
		return apierr.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if err := apierr.MissingFields(c, "nasname", p.NASName, "secret", p.Secret); err != nil {
		return nil
	}
	n := models.NAS{NASName: p.NASName, Secret: p.Secret}
	t := p.Type
	if t == "" {
		t = "other"
	}
	n.Type = &t
	if p.ShortName != "" {
		s := p.ShortName
		n.ShortName = &s
	}
	if p.Ports != 0 {
		v := p.Ports
		n.Ports = &v
	}
	if p.Server != "" {
		s := p.Server
		n.Server = &s
	}
	if p.Community != "" {
		s := p.Community
		n.Community = &s
	}
	if p.Description != "" {
		s := p.Description
		n.Description = &s
	}
	if err := database.DB.Create(&n).Error; err != nil {
		return apierr.Internal(c, err.Error())
	}
	return c.Status(201).JSON(nasOut(n))
}

// v1GetNas godoc
// @Summary Get Nas
// @Tags v1-nas
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param nas_id path int true "NAS id"
// @Success 200 {object} schemas.V1NAS
// @Router /api/v1/nas/{nas_id} [get]
func v1GetNas(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("nas_id")
	var n models.NAS
	if err := database.DB.First(&n, id).Error; err != nil {
		return apierr.NotFound(c, "NAS not found")
	}
	return c.JSON(nasOut(n))
}

// v1UpdateNas godoc
// @Summary Update Nas
// @Tags v1-nas
// @Security ApiKeyAuth
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param nas_id path int true "NAS id"
// @Param body body schemas.V1NASCreate true "NAS"
// @Success 200 {object} schemas.V1NAS
// @Router /api/v1/nas/{nas_id} [put]
func v1UpdateNas(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("nas_id")
	var n models.NAS
	if err := database.DB.First(&n, id).Error; err != nil {
		return apierr.NotFound(c, "NAS not found")
	}
	var p schemas.V1NASCreate
	if err := c.BodyParser(&p); err != nil {
		return apierr.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if p.NASName != "" {
		n.NASName = p.NASName
	}
	if p.Secret != "" {
		n.Secret = p.Secret
	}
	if p.ShortName != "" {
		s := p.ShortName
		n.ShortName = &s
	}
	if p.Type != "" {
		s := p.Type
		n.Type = &s
	}
	if p.Ports != 0 {
		v := p.Ports
		n.Ports = &v
	}
	if p.Server != "" {
		s := p.Server
		n.Server = &s
	}
	if p.Community != "" {
		s := p.Community
		n.Community = &s
	}
	if p.Description != "" {
		s := p.Description
		n.Description = &s
	}
	if err := database.DB.Save(&n).Error; err != nil {
		return apierr.Internal(c, err.Error())
	}
	return c.JSON(nasOut(n))
}

// v1DeleteNas godoc
// @Summary Delete Nas
// @Tags v1-nas
// @Security ApiKeyAuth
// @Security BasicAuth
// @Param nas_id path int true "NAS id"
// @Success 204
// @Router /api/v1/nas/{nas_id} [delete]
func v1DeleteNas(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("nas_id")
	res := database.DB.Delete(&models.NAS{}, id)
	if res.RowsAffected == 0 {
		return apierr.NotFound(c, "NAS not found")
	}
	return c.SendStatus(204)
}

// ===== RADACCT =====

// v1ListRadacct godoc
// @Summary Get Radacct List
// @Tags v1-radacct
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param skip query int false "Skip"
// @Param limit query int false "Limit (max 1000)"
// @Param username query string false "Filter by username"
// @Param start_date query string false "RFC3339, lower bound on acctstarttime"
// @Param end_date query string false "RFC3339, upper bound on acctstarttime"
// @Success 200 {array} schemas.V1Radacct
// @Router /api/v1/radacct/ [get]
func v1ListRadacct(c *fiber.Ctx) error {
	skip, limit := paging(c)
	q := database.DB.Model(&models.Radacct{})
	if v := c.Query("username"); v != "" {
		q = q.Where("username = ?", v)
	}
	if v := c.Query("start_date"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q = q.Where("acctstarttime >= ?", t)
		}
	}
	if v := c.Query("end_date"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q = q.Where("acctstarttime <= ?", t)
		}
	}
	var total int64
	q.Count(&total)
	c.Set("X-Total-Count", strconv.FormatInt(total, 10))

	var rows []models.Radacct
	q.Order("radacctid DESC").Offset(skip).Limit(limit).Find(&rows)
	out := make([]schemas.V1Radacct, 0, len(rows))
	for _, r := range rows {
		out = append(out, acctOut(r))
	}
	return c.JSON(out)
}

// v1GetRadacct godoc
// @Summary Get Radacct
// @Tags v1-radacct
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param radacctid path int true "radacctid"
// @Success 200 {object} schemas.V1Radacct
// @Router /api/v1/radacct/{radacctid} [get]
func v1GetRadacct(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("radacctid")
	var r models.Radacct
	if err := database.DB.Where("radacctid = ?", id).First(&r).Error; err != nil {
		return apierr.NotFound(c, "Session not found")
	}
	return c.JSON(acctOut(r))
}

// v1UserStatus godoc
// @Summary Get User Status (online/offline + active session info)
// @Tags v1-radacct
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param username path string true "username"
// @Success 200 {object} schemas.V1UserStatus
// @Router /api/v1/radacct/status/{username} [get]
func v1UserStatus(c *fiber.Ctx) error {
	username := c.Params("username")
	var r models.Radacct
	err := database.DB.
		Where("username = ? AND acctstoptime IS NULL", username).
		Order("acctstarttime DESC").
		First(&r).Error
	if err != nil {
		return c.JSON(schemas.V1UserStatus{Username: username, Online: false, IsOnline: false})
	}
	s := acctOut(r)
	return c.JSON(schemas.V1UserStatus{Username: username, Online: true, IsOnline: true, Session: &s})
}

// ===== DISCONNECT =====

// v1Disconnect godoc
// @Summary Disconnect a user from NAS (RFC 5176 Disconnect-Request)
// @Tags v1-disconnect
// @Security ApiKeyAuth
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param body body schemas.V1DisconnectRequest true "Disconnect payload"
// @Success 200 {object} schemas.V1DisconnectResponse
// @Failure 404 {object} schemas.ErrorResponse "NAS belum terdaftar di tabel `nas`"
// @Failure 504 {object} schemas.ErrorResponse
// @Router /api/v1/disconnect [post]
func v1Disconnect(c *fiber.Ctx) error {
	var p schemas.V1DisconnectRequest
	if err := c.BodyParser(&p); err != nil {
		return apierr.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if err := apierr.MissingFields(c, "username", p.Username); err != nil {
		return nil
	}

	// `nas_ip` dan `nas_ip_address` SAMA artinya. Yang pertama didokumentasikan
	// aplikasi Python dan dipakai konsumen lain; backend ERP mengirim keduanya.
	nasIPMinta := strings.TrimSpace(p.NASIPAddress)
	if nasIPMinta == "" {
		nasIPMinta = strings.TrimSpace(p.NasIP)
	}
	secretMinta := strings.TrimSpace(p.RadiusSecret)

	// Sesi dicari untuk melengkapi atribut (Acct-Session-Id, Framed-IP) dan
	// untuk menebak NAS bila pemanggil tak menyebutkannya. Ketiadaannya BUKAN
	// penghalang — lihat di bawah.
	q := database.DB.Where("username = ? AND acctstoptime IS NULL", p.Username)
	if nasIPMinta != "" {
		q = q.Where("nasipaddress = ?", nasIPMinta)
	}
	var sess models.Radacct
	adaSesi := true
	if err := q.Order("acctstarttime DESC").First(&sess).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return apierr.Internal(c, err.Error())
		}
		adaSesi = false
	}

	nasIP := nasIPMinta
	if nasIP == "" && adaSesi {
		nasIP = sess.NASIPAddress
	}

	// TANPA sesi aktif, paket TETAP dikirim selama tujuannya diketahui.
	//
	// Aplikasi Python tidak pernah melihat radacct: ia menjalankan
	// `radclient <nas_ip>:<port> disconnect <secret>` dengan User-Name saja.
	// Jadi pelanggan yang SEDANG ONLINE tapi baris radacct-nya sudah tertutup
	// (dibersihkan autoclearzombie, atau accounting-nya tak sampai) TETAP
	// tertendang di Python. Menolak lebih awal di sini membuat isolir diam-diam
	// berhenti menggigit justru pada kasus yang paling perlu.
	//
	// 200, BUKAN 404, saat tujuannya memang tak bisa ditebak: pemanggil
	// membedakan "server tak terjangkau" (error) dari "tidak ditendang"
	// (200 + tidak-diterima). Dengan non-2xx, backend membacanya sebagai server
	// mati, jatuh ke jalur cadangan yang percuma, dan mencatat kegagalan ke
	// PEMUTUS-ARUS-nya — tiga pelanggan offline berurutan pada satu NAS sudah
	// cukup membukanya, dan sesudahnya tendangan yang NYATA pun dilewati.
	if nasIP == "" {
		return c.JSON(schemas.V1DisconnectResponse{
			Status:   "rejected",
			Code:     "no-active-session",
			Username: p.Username,
			Success:  false,
			Output: "No active session for user " + p.Username +
				" and no nas_ip supplied — no Disconnect-Request sent",
		})
	}

	// Secret dari PEMANGGIL lebih dulu.
	//
	// Aplikasi Python memakainya langsung, sehingga NAS tak perlu terdaftar di
	// tabel `nas` instance ini. Backend ERP mengirimnya di jalur isolir utama
	// (customer/service.go: RadiusSecret: n.Secret). Mengabaikannya berarti
	// setiap NAS yang ada di ERP tapi belum terdaftar di sini berhenti bisa
	// diisolir — padahal sebelumnya jalan.
	secret := secretMinta
	if secret == "" {
		var nas models.NAS
		if err := database.DB.Where("nasname = ?", nasIP).First(&nas).Error; err != nil {
			return apierr.NotFound(c,
				"NAS "+nasIP+" tidak terdaftar di tabel `nas` dan radius_secret tidak dikirim — "+
					"daftarkan NAS-nya, atau sertakan radius_secret pada permintaan")
		}
		secret = nas.Secret
	}

	port := p.Port
	if port == 0 {
		port = 3799
	}
	timeout := time.Duration(p.TimeoutMs) * time.Millisecond
	if timeout == 0 {
		timeout = 3 * time.Second
	}
	ip := net.ParseIP(nasIP)
	if ip == nil {
		return apierr.BadRequest(c, "Invalid NAS IP address: "+nasIP)
	}

	// Perakitan paket ada di SATU tempat, bangunPaketDisconnect (msgauth.go),
	// beserta alasan kenapa atributnya seminimal itu.
	acctSID := ""
	if adaSesi {
		acctSID = sess.AcctSessionID
	}
	pkt, perr := bangunPaketDisconnect(secret, p.Username, acctSID)
	if perr != nil {
		return apierr.Internal(c, "build disconnect packet: "+perr.Error())
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	addr := net.JoinHostPort(nasIP, strconv.Itoa(port))
	resp, err := (&radius.Client{}).Exchange(ctx, pkt, addr)
	if err != nil {
		return apierr.GatewayTimeout(c, "disconnect failed: "+err.Error())
	}

	status := "unknown"
	switch resp.Code {
	case radius.CodeDisconnectACK:
		status = "acknowledged"
	case radius.CodeDisconnectNAK:
		status = "rejected"
	}
	return c.JSON(schemas.V1DisconnectResponse{
		Status: status, Code: resp.Code.String(),
		Username: p.Username, NASIPAddress: nasIP, Port: port,
		AcctSessionID: sess.AcctSessionID,
		// Bentuk lama. `output` mengikuti format keluaran radclient karena ada
		// alat yang memutuskan "router menjawab" dari substring
		// "Received Disconnect" di dalamnya.
		Success: status == "acknowledged",
		Output: "Received " + resp.Code.String() + " Id " +
			strconv.Itoa(int(resp.Identifier)) + " from " + addr,
		NasIP: nasIP,
	})
}
