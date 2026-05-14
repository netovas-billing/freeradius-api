package handlers

import (
	"errors"
	"sort"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"freeradius-api/database"
	"freeradius-api/models"
	"freeradius-api/schemas"
)

func RegisterGroups(r fiber.Router) {
	g := r.Group("/groups")
	g.Get("", listGroups)
	g.Post("", createGroup)
	g.Get("/:groupname", getGroup)
	g.Delete("/:groupname", deleteGroup)
	g.Post("/:groupname/users/:username", addUserToGroup)
	g.Delete("/:groupname/users/:username", removeUserFromGroup)
}

func buildGroupOut(groupname string) schemas.GroupOut {
	var check []models.Radgroupcheck
	var reply []models.Radgroupreply
	database.DB.Where("groupname = ?", groupname).Find(&check)
	database.DB.Where("groupname = ?", groupname).Find(&reply)

	out := schemas.GroupOut{
		Groupname:       groupname,
		CheckAttributes: []schemas.AttributeOut{},
		ReplyAttributes: []schemas.AttributeOut{},
	}
	for _, r := range check {
		out.CheckAttributes = append(out.CheckAttributes, schemas.AttributeOut{
			ID: r.ID, Attribute: r.Attribute, Op: r.Op, Value: r.Value,
		})
	}
	for _, r := range reply {
		out.ReplyAttributes = append(out.ReplyAttributes, schemas.AttributeOut{
			ID: r.ID, Attribute: r.Attribute, Op: r.Op, Value: r.Value,
		})
	}
	return out
}

// listGroups godoc
// @Summary List groups
// @Tags groups
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Success 200 {array} string
// @Router /api/groups [get]
func listGroups(c *fiber.Ctx) error {
	names := map[string]struct{}{}
	collect := func(model interface{}) {
		var rows []string
		database.DB.Model(model).Distinct("groupname").Pluck("groupname", &rows)
		for _, x := range rows {
			names[x] = struct{}{}
		}
	}
	collect(&models.Radgroupcheck{})
	collect(&models.Radgroupreply{})
	collect(&models.Radusergroup{})

	out := make([]string, 0, len(names))
	for k := range names {
		out = append(out, k)
	}
	sort.Strings(out)
	return c.JSON(out)
}

// createGroup godoc
// @Summary Create group (insert group attributes)
// @Tags groups
// @Security ApiKeyAuth
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param body body schemas.GroupCreate true "Group payload"
// @Success 201 {object} schemas.GroupOut
// @Router /api/groups [post]
func createGroup(c *fiber.Ctx) error {
	var payload schemas.GroupCreate
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if payload.Groupname == "" {
		return c.Status(400).JSON(fiber.Map{"error": "groupname required"})
	}
	for _, a := range payload.CheckAttributes {
		op := a.Op
		if op == "" {
			op = ":="
		}
		database.DB.Create(&models.Radgroupcheck{Groupname: payload.Groupname, Attribute: a.Attribute, Op: op, Value: a.Value})
	}
	for _, a := range payload.ReplyAttributes {
		op := a.Op
		if op == "" {
			op = ":="
		}
		database.DB.Create(&models.Radgroupreply{Groupname: payload.Groupname, Attribute: a.Attribute, Op: op, Value: a.Value})
	}
	return c.Status(201).JSON(buildGroupOut(payload.Groupname))
}

// getGroup godoc
// @Summary Get group detail
// @Tags groups
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param groupname path string true "Group name"
// @Success 200 {object} schemas.GroupOut
// @Router /api/groups/{groupname} [get]
func getGroup(c *fiber.Ctx) error {
	return c.JSON(buildGroupOut(c.Params("groupname")))
}

// deleteGroup godoc
// @Summary Delete group
// @Tags groups
// @Security ApiKeyAuth
// @Security BasicAuth
// @Param groupname path string true "Group name"
// @Success 204
// @Router /api/groups/{groupname} [delete]
func deleteGroup(c *fiber.Ctx) error {
	groupname := c.Params("groupname")
	database.DB.Where("groupname = ?", groupname).Delete(&models.Radgroupcheck{})
	database.DB.Where("groupname = ?", groupname).Delete(&models.Radgroupreply{})
	database.DB.Where("groupname = ?", groupname).Delete(&models.Radusergroup{})
	return c.SendStatus(204)
}

// addUserToGroup godoc
// @Summary Add user to group
// @Tags groups
// @Security ApiKeyAuth
// @Security BasicAuth
// @Param groupname path string true "Group name"
// @Param username path string true "Username"
// @Param priority query int false "Priority"
// @Success 201 {object} map[string]interface{}
// @Router /api/groups/{groupname}/users/{username} [post]
func addUserToGroup(c *fiber.Ctx) error {
	groupname := c.Params("groupname")
	username := c.Params("username")
	priority := c.QueryInt("priority", 1)

	var existing models.Radusergroup
	err := database.DB.Where("username = ? AND groupname = ?", username, groupname).First(&existing).Error
	if err == nil {
		existing.Priority = priority
		database.DB.Save(&existing)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		database.DB.Create(&models.Radusergroup{Username: username, Groupname: groupname, Priority: priority})
	} else {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(201).JSON(fiber.Map{"username": username, "groupname": groupname, "priority": priority})
}

// removeUserFromGroup godoc
// @Summary Remove user from group
// @Tags groups
// @Security ApiKeyAuth
// @Security BasicAuth
// @Param groupname path string true "Group name"
// @Param username path string true "Username"
// @Success 204
// @Failure 404 {object} schemas.ErrorResponse
// @Router /api/groups/{groupname}/users/{username} [delete]
func removeUserFromGroup(c *fiber.Ctx) error {
	groupname := c.Params("groupname")
	username := c.Params("username")
	res := database.DB.Where("username = ? AND groupname = ?", username, groupname).Delete(&models.Radusergroup{})
	if res.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Membership not found"})
	}
	return c.SendStatus(204)
}
