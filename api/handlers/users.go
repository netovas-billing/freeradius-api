package handlers

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"freeradius-api/database"
	"freeradius-api/models"
	"freeradius-api/schemas"
)

var passwordAttrs = []string{
	"Cleartext-Password", "Password.Cleartext",
	"NT-Password", "Password.NT",
	"MD5-Password", "Password.MD5",
	"SHA-Password", "Password.SHA1",
	"SSHA-Password", "Password.SSHA",
	"SSHA2-256-Password", "SSHA2-512-Password",
	"Crypt-Password", "Password.Crypt",
}

func RegisterUsers(r fiber.Router) {
	g := r.Group("/users")
	g.Get("", listUsers)
	g.Post("", createUser)
	g.Get("/:username", getUser)
	g.Put("/:username/password", updatePassword)
	g.Delete("/:username", deleteUser)
	g.Post("/:username/check", addCheckAttr)
	g.Post("/:username/reply", addReplyAttr)
	g.Put("/:username/check/:id", updateCheckAttr)
	g.Put("/:username/reply/:id", updateReplyAttr)
	g.Delete("/:username/check/:id", deleteCheckAttr)
	g.Delete("/:username/reply/:id", deleteReplyAttr)
}

func buildUserOut(username string) (*schemas.UserOut, error) {
	var check []models.Radcheck
	if err := database.DB.Where("username = ?", username).Find(&check).Error; err != nil {
		return nil, err
	}
	if len(check) == 0 {
		return nil, nil
	}
	var reply []models.Radreply
	database.DB.Where("username = ?", username).Find(&reply)

	var grp []models.Radusergroup
	database.DB.Where("username = ?", username).Order("priority").Find(&grp)

	out := &schemas.UserOut{
		Username:        username,
		CheckAttributes: []schemas.AttributeOut{},
		ReplyAttributes: []schemas.AttributeOut{},
		Groups:          []string{},
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
	for _, g := range grp {
		out.Groups = append(out.Groups, g.Groupname)
	}
	return out, nil
}

// listUsers godoc
// @Summary List users with filter
// @Description Filter via q (substring match on username), group (members of a group),
// @Description attribute (only users that have this check attribute, e.g. Expiration).
// @Description Response sets X-Total-Count header.
// @Tags users
// @Security ApiKeyAuth
// @Produce json
// @Param q query string false "Substring on username (LIKE %q%)"
// @Param group query string false "Only users in this group"
// @Param attribute query string false "Only users that have this check attribute"
// @Param skip query int false "Skip"
// @Param limit query int false "Limit (max 1000)"
// @Success 200 {array} string
// @Header 200 {integer} X-Total-Count "total matching users"
// @Router /api/users [get]
func listUsers(c *fiber.Ctx) error {
	skip := c.QueryInt("skip", 0)
	limit := c.QueryInt("limit", 100)
	if limit > 1000 {
		limit = 1000
	}
	q := c.Query("q")
	group := c.Query("group")
	attr := c.Query("attribute")

	tx := database.DB.Model(&models.Radcheck{}).Distinct("username")
	if q != "" {
		tx = tx.Where("username LIKE ?", "%"+q+"%")
	}
	if attr != "" {
		tx = tx.Where("attribute = ?", attr)
	}
	if group != "" {
		tx = tx.Where("username IN (?)",
			database.DB.Model(&models.Radusergroup{}).Select("username").Where("groupname = ?", group))
	}

	var total int64
	tx.Count(&total)
	c.Set("X-Total-Count", strconv.FormatInt(total, 10))

	var names []string
	tx.Order("username").Offset(skip).Limit(limit).Pluck("username", &names)
	return c.JSON(names)
}

// createUser godoc
// @Summary Create user
// @Tags users
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param body body schemas.UserCreate true "User payload"
// @Success 201 {object} schemas.UserOut
// @Failure 409 {object} schemas.ErrorResponse
// @Router /api/users [post]
func createUser(c *fiber.Ctx) error {
	var payload schemas.UserCreate
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if payload.Username == "" || payload.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "username and password are required"})
	}
	if payload.PasswordAttr == "" {
		payload.PasswordAttr = "Cleartext-Password"
	}

	var existing models.Radcheck
	if err := database.DB.Where("username = ?", payload.Username).First(&existing).Error; err == nil {
		return c.Status(409).JSON(fiber.Map{"error": "User already exists"})
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&models.Radcheck{
			Username: payload.Username, Attribute: payload.PasswordAttr, Op: ":=", Value: payload.Password,
		}).Error; err != nil {
			return err
		}
		for _, a := range payload.CheckAttributes {
			op := a.Op
			if op == "" {
				op = ":="
			}
			if err := tx.Create(&models.Radcheck{
				Username: payload.Username, Attribute: a.Attribute, Op: op, Value: a.Value,
			}).Error; err != nil {
				return err
			}
		}
		for _, a := range payload.ReplyAttributes {
			op := a.Op
			if op == "" {
				op = ":="
			}
			if err := tx.Create(&models.Radreply{
				Username: payload.Username, Attribute: a.Attribute, Op: op, Value: a.Value,
			}).Error; err != nil {
				return err
			}
		}
		for i, gname := range payload.Groups {
			if err := tx.Create(&models.Radusergroup{
				Username: payload.Username, Groupname: gname, Priority: i + 1,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	out, _ := buildUserOut(payload.Username)
	return c.Status(201).JSON(out)
}

// getUser godoc
// @Summary Get user detail
// @Tags users
// @Security ApiKeyAuth
// @Produce json
// @Param username path string true "Username"
// @Success 200 {object} schemas.UserOut
// @Failure 404 {object} schemas.ErrorResponse
// @Router /api/users/{username} [get]
func getUser(c *fiber.Ctx) error {
	username := c.Params("username")
	out, err := buildUserOut(username)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if out == nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}
	return c.JSON(out)
}

// updatePassword godoc
// @Summary Update user password
// @Tags users
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Param body body schemas.UserPasswordUpdate true "Password payload"
// @Success 200 {object} schemas.UserOut
// @Failure 404 {object} schemas.ErrorResponse
// @Router /api/users/{username}/password [put]
func updatePassword(c *fiber.Ctx) error {
	username := c.Params("username")
	var payload schemas.UserPasswordUpdate
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if payload.PasswordAttr == "" {
		payload.PasswordAttr = "Cleartext-Password"
	}

	var row models.Radcheck
	err := database.DB.Where("username = ? AND attribute IN ?", username, passwordAttrs).First(&row).Error
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User or password attribute not found"})
	}
	row.Attribute = payload.PasswordAttr
	row.Value = payload.Password
	if err := database.DB.Save(&row).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	out, _ := buildUserOut(username)
	return c.JSON(out)
}

// deleteUser godoc
// @Summary Delete user
// @Tags users
// @Security ApiKeyAuth
// @Param username path string true "Username"
// @Success 204
// @Failure 404 {object} schemas.ErrorResponse
// @Router /api/users/{username} [delete]
func deleteUser(c *fiber.Ctx) error {
	username := c.Params("username")
	res := database.DB.Where("username = ?", username).Delete(&models.Radcheck{})
	database.DB.Where("username = ?", username).Delete(&models.Radreply{})
	database.DB.Where("username = ?", username).Delete(&models.Radusergroup{})
	if res.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}
	return c.SendStatus(204)
}

// addCheckAttr godoc
// @Summary Add check attribute
// @Tags users
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Param body body schemas.AttributeIn true "Attribute"
// @Success 201 {object} schemas.AttributeOut
// @Router /api/users/{username}/check [post]
func addCheckAttr(c *fiber.Ctx) error {
	username := c.Params("username")
	var attr schemas.AttributeIn
	if err := c.BodyParser(&attr); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if attr.Op == "" {
		attr.Op = ":="
	}
	row := models.Radcheck{Username: username, Attribute: attr.Attribute, Op: attr.Op, Value: attr.Value}
	if err := database.DB.Create(&row).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(201).JSON(schemas.AttributeOut{ID: row.ID, Attribute: row.Attribute, Op: row.Op, Value: row.Value})
}

// addReplyAttr godoc
// @Summary Add reply attribute
// @Tags users
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Param body body schemas.AttributeIn true "Attribute"
// @Success 201 {object} schemas.AttributeOut
// @Router /api/users/{username}/reply [post]
func addReplyAttr(c *fiber.Ctx) error {
	username := c.Params("username")
	var attr schemas.AttributeIn
	if err := c.BodyParser(&attr); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if attr.Op == "" {
		attr.Op = ":="
	}
	row := models.Radreply{Username: username, Attribute: attr.Attribute, Op: attr.Op, Value: attr.Value}
	if err := database.DB.Create(&row).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(201).JSON(schemas.AttributeOut{ID: row.ID, Attribute: row.Attribute, Op: row.Op, Value: row.Value})
}

// updateCheckAttr godoc
// @Summary Update check attribute (partial)
// @Tags users
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Param id path int true "Attribute ID"
// @Param body body schemas.AttributeUpdate true "Fields to update"
// @Success 200 {object} schemas.AttributeOut
// @Failure 404 {object} schemas.ErrorResponse
// @Router /api/users/{username}/check/{id} [put]
func updateCheckAttr(c *fiber.Ctx) error {
	username := c.Params("username")
	id, _ := c.ParamsInt("id")
	var patch schemas.AttributeUpdate
	if err := c.BodyParser(&patch); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	var row models.Radcheck
	if err := database.DB.Where("username = ? AND id = ?", username, id).First(&row).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Attribute not found"})
	}
	if patch.Attribute != nil {
		row.Attribute = *patch.Attribute
	}
	if patch.Op != nil {
		row.Op = *patch.Op
	}
	if patch.Value != nil {
		row.Value = *patch.Value
	}
	if err := database.DB.Save(&row).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(schemas.AttributeOut{ID: row.ID, Attribute: row.Attribute, Op: row.Op, Value: row.Value})
}

// updateReplyAttr godoc
// @Summary Update reply attribute (partial)
// @Tags users
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param username path string true "Username"
// @Param id path int true "Attribute ID"
// @Param body body schemas.AttributeUpdate true "Fields to update"
// @Success 200 {object} schemas.AttributeOut
// @Failure 404 {object} schemas.ErrorResponse
// @Router /api/users/{username}/reply/{id} [put]
func updateReplyAttr(c *fiber.Ctx) error {
	username := c.Params("username")
	id, _ := c.ParamsInt("id")
	var patch schemas.AttributeUpdate
	if err := c.BodyParser(&patch); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	var row models.Radreply
	if err := database.DB.Where("username = ? AND id = ?", username, id).First(&row).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Attribute not found"})
	}
	if patch.Attribute != nil {
		row.Attribute = *patch.Attribute
	}
	if patch.Op != nil {
		row.Op = *patch.Op
	}
	if patch.Value != nil {
		row.Value = *patch.Value
	}
	if err := database.DB.Save(&row).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(schemas.AttributeOut{ID: row.ID, Attribute: row.Attribute, Op: row.Op, Value: row.Value})
}

// deleteCheckAttr godoc
// @Summary Delete check attribute
// @Tags users
// @Security ApiKeyAuth
// @Param username path string true "Username"
// @Param id path int true "Attribute ID"
// @Success 204
// @Failure 404 {object} schemas.ErrorResponse
// @Router /api/users/{username}/check/{id} [delete]
func deleteCheckAttr(c *fiber.Ctx) error {
	username := c.Params("username")
	id, _ := c.ParamsInt("id")
	res := database.DB.Where("username = ? AND id = ?", username, id).Delete(&models.Radcheck{})
	if res.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Attribute not found"})
	}
	return c.SendStatus(204)
}

// deleteReplyAttr godoc
// @Summary Delete reply attribute
// @Tags users
// @Security ApiKeyAuth
// @Param username path string true "Username"
// @Param id path int true "Attribute ID"
// @Success 204
// @Failure 404 {object} schemas.ErrorResponse
// @Router /api/users/{username}/reply/{id} [delete]
func deleteReplyAttr(c *fiber.Ctx) error {
	username := c.Params("username")
	id, _ := c.ParamsInt("id")
	res := database.DB.Where("username = ? AND id = ?", username, id).Delete(&models.Radreply{})
	if res.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Attribute not found"})
	}
	return c.SendStatus(204)
}
