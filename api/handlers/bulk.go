package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"freeradius-api/apierr"
	"freeradius-api/database"
	"freeradius-api/models"
	"freeradius-api/schemas"
)

func RegisterBulk(r fiber.Router) {
	r.Post("/users/bulk", bulkCreateUsers)
}

// bulkCreateUsers godoc
// @Summary Bulk create users
// @Description Per-user transaction; partial success diperbolehkan.
// @Description Status code 200 (sebagian gagal/sukses) atau 201 (semua sukses).
// @Tags users
// @Security ApiKeyAuth
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param body body []schemas.UserCreate true "Array of UserCreate"
// @Success 201 {object} schemas.BulkUserResponse
// @Success 200 {object} schemas.BulkUserResponse
// @Router /api/users/bulk [post]
func bulkCreateUsers(c *fiber.Ctx) error {
	var payload []schemas.UserCreate
	if err := c.BodyParser(&payload); err != nil {
		return apierr.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if len(payload) == 0 {
		return apierr.BadRequest(c, "Empty array")
	}
	if len(payload) > 500 {
		return apierr.BadRequest(c, "Max 500 users per call")
	}

	out := schemas.BulkUserResponse{
		Total:   len(payload),
		Results: make([]schemas.BulkUserResult, 0, len(payload)),
	}

	for i, u := range payload {
		r := schemas.BulkUserResult{Index: i, Username: u.Username}
		if err := createOneUser(u); err != nil {
			r.Status = "error"
			r.Error = err.Error()
			out.Failed++
		} else {
			r.Status = "created"
			out.Created++
		}
		out.Results = append(out.Results, r)
	}

	if out.Failed == 0 {
		return c.Status(201).JSON(out)
	}
	return c.Status(200).JSON(out)
}

func createOneUser(u schemas.UserCreate) error {
	if u.Username == "" || u.Password == "" {
		return errors.New("username and password required")
	}
	passAttr := u.PasswordAttr
	if passAttr == "" {
		passAttr = "Cleartext-Password"
	}

	var existing models.Radcheck
	err := database.DB.Where("username = ?", u.Username).First(&existing).Error
	if err == nil {
		return errors.New("user already exists")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&models.Radcheck{
			Username: u.Username, Attribute: passAttr, Op: ":=", Value: u.Password,
		}).Error; err != nil {
			return err
		}
		for _, a := range u.CheckAttributes {
			op := a.Op
			if op == "" {
				op = ":="
			}
			if err := tx.Create(&models.Radcheck{
				Username: u.Username, Attribute: a.Attribute, Op: op, Value: a.Value,
			}).Error; err != nil {
				return err
			}
		}
		for _, a := range u.ReplyAttributes {
			op := a.Op
			if op == "" {
				op = ":="
			}
			if err := tx.Create(&models.Radreply{
				Username: u.Username, Attribute: a.Attribute, Op: op, Value: a.Value,
			}).Error; err != nil {
				return err
			}
		}
		for i, gname := range u.Groups {
			if err := tx.Create(&models.Radusergroup{
				Username: u.Username, Groupname: gname, Priority: i + 1,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
