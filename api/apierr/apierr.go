// Package apierr — format error konsisten di SEMUA endpoint (modern /api/* dan
// legacy /api/v1/*). Schema:
//
//	{
//	  "error":  "<human-readable message>",
//	  "code":   "<machine-readable code, snake_case>",
//	  "fields": [ {"name": "...", "code": "..."} ]   // hanya untuk validation_error
//	}
//
// Caller pattern untuk validation:
//
//	if err := apierr.MissingFields(c, "username", p.Username); err != nil {
//	    return nil   // response sudah ditulis, return nil = Fiber kirim apa yang ter-set
//	}
package apierr

import (
	"errors"

	"github.com/gofiber/fiber/v2"
)

type Body struct {
	Error  string  `json:"error"`
	Code   string  `json:"code"`
	Fields []Field `json:"fields,omitempty"`
}

type Field struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

// ErrWritten — sentinel non-nil error. Helper validation (MissingFields, Validation)
// pakai ini supaya caller bisa cek `if err != nil { return nil }` tanpa men-trigger
// Fiber error handler yang akan menimpa response.
var ErrWritten = errors.New("apierr: response already written")

func send(c *fiber.Ctx, status int, msg, code string, fields []Field) error {
	return c.Status(status).JSON(Body{Error: msg, Code: code, Fields: fields})
}

// ---- helpers per HTTP status ----

func BadRequest(c *fiber.Ctx, msg string) error {
	return send(c, fiber.StatusBadRequest, msg, "bad_request", nil)
}

func Unauthorized(c *fiber.Ctx, msg string) error {
	return send(c, fiber.StatusUnauthorized, msg, "unauthorized", nil)
}

func Forbidden(c *fiber.Ctx, msg, code string) error {
	if code == "" {
		code = "forbidden"
	}
	return send(c, fiber.StatusForbidden, msg, code, nil)
}

func NotFound(c *fiber.Ctx, msg string) error {
	return send(c, fiber.StatusNotFound, msg, "not_found", nil)
}

func Conflict(c *fiber.Ctx, msg string) error {
	return send(c, fiber.StatusConflict, msg, "conflict", nil)
}

func TooMany(c *fiber.Ctx, msg string) error {
	return send(c, fiber.StatusTooManyRequests, msg, "rate_limited", nil)
}

func Internal(c *fiber.Ctx, msg string) error {
	return send(c, fiber.StatusInternalServerError, msg, "internal_error", nil)
}

func BadGateway(c *fiber.Ctx, msg string) error {
	return send(c, fiber.StatusBadGateway, msg, "bad_gateway", nil)
}

func GatewayTimeout(c *fiber.Ctx, msg string) error {
	return send(c, fiber.StatusGatewayTimeout, msg, "gateway_timeout", nil)
}

// ---- validation (422) ----

// MissingFields — varargs (fieldName, value). Untuk tiap value kosong, tambah ke fields.
// Kalau ada minimal satu yang missing → tulis 422 + return ErrWritten (sentinel non-nil).
// Kalau semua OK → return nil.
func MissingFields(c *fiber.Ctx, pairs ...string) error {
	if len(pairs)%2 != 0 {
		return nil
	}
	var fields []Field
	for i := 0; i < len(pairs); i += 2 {
		if pairs[i+1] == "" {
			fields = append(fields, Field{Name: pairs[i], Code: "required"})
		}
	}
	if len(fields) == 0 {
		return nil
	}
	_ = send(c, fiber.StatusUnprocessableEntity, "Request validation failed", "validation_error", fields)
	return ErrWritten
}

// Validation — return 422 dengan list field issues kustom + sentinel ErrWritten.
func Validation(c *fiber.Ctx, fields []Field) error {
	_ = send(c, fiber.StatusUnprocessableEntity, "Request validation failed", "validation_error", fields)
	return ErrWritten
}
