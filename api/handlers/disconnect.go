package handlers

import (
	"context"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2866"

	"freeradius-api/apierr"
	"freeradius-api/database"
	"freeradius-api/models"
	"freeradius-api/schemas"
)

func RegisterSessions(r fiber.Router) {
	g := r.Group("/sessions")
	g.Post("/:acctuniqueid/disconnect", disconnectSession)
}

const defaultCoAPort = 3799

// disconnectSession godoc
// @Summary Disconnect an active session (RFC 5176 Disconnect-Request)
// @Description Looks up session by acctuniqueid in radacct, finds NAS secret in the nas table
// @Description (matched by nasipaddress = nas.nasname), then sends a Disconnect-Request
// @Description to the NAS over UDP. Returns the NAS response (ACK / NAK / timeout).
// @Tags sessions
// @Security ApiKeyAuth
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param acctuniqueid path string true "radacct.acctuniqueid"
// @Param body body schemas.DisconnectRequest false "Override port / timeout"
// @Success 200 {object} schemas.DisconnectResponse
// @Failure 404 {object} schemas.ErrorResponse
// @Failure 504 {object} schemas.ErrorResponse
// @Router /api/sessions/{acctuniqueid}/disconnect [post]
func disconnectSession(c *fiber.Ctx) error {
	acctUniqueID := c.Params("acctuniqueid")

	var req schemas.DisconnectRequest
	_ = c.BodyParser(&req)
	port := req.Port
	if port == 0 {
		port = defaultCoAPort
	}
	timeout := time.Duration(req.Timeout) * time.Millisecond
	if timeout == 0 {
		timeout = 3 * time.Second
	}

	var sess models.Radacct
	if err := database.DB.Where("acctuniqueid = ?", acctUniqueID).First(&sess).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apierr.NotFound(c, "Session not found")
		}
		return apierr.Internal(c, err.Error())
	}
	if sess.AcctStopTime != nil {
		return apierr.Conflict(c, "Session already stopped")
	}
	if sess.NASIPAddress == "" {
		return apierr.BadRequest(c, "Session has empty nasipaddress")
	}

	var nas models.NAS
	if err := database.DB.Where("nasname = ?", sess.NASIPAddress).First(&nas).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "NAS secret not registered in `nas` table (clients.conf-only setups not supported). Add the NAS via POST /api/nas first.",
		})
	}

	nasIP := net.ParseIP(sess.NASIPAddress)
	if nasIP == nil {
		return apierr.BadRequest(c, "Invalid NAS IP address")
	}

	packet := radius.New(radius.CodeDisconnectRequest, []byte(nas.Secret))
	_ = rfc2865.UserName_SetString(packet, sess.Username)
	_ = rfc2866.AcctSessionID_SetString(packet, sess.AcctSessionID)
	_ = rfc2865.NASIPAddress_Set(packet, nasIP)
	if sess.FramedIPAddress != "" {
		if ip := net.ParseIP(sess.FramedIPAddress); ip != nil {
			_ = rfc2865.FramedIPAddress_Set(packet, ip)
		}
	}
	if sess.CallingStationID != "" {
		_ = rfc2865.CallingStationID_SetString(packet, sess.CallingStationID)
	}

	addr := net.JoinHostPort(sess.NASIPAddress, strconv.Itoa(port))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := radius.Client{}
	response, err := client.Exchange(ctx, packet, addr)
	if err != nil {
		return c.Status(504).JSON(fiber.Map{
			"error": "Disconnect-Request failed: " + err.Error(),
			"nasipaddress": sess.NASIPAddress,
			"port":         port,
		})
	}

	status := "unknown"
	switch response.Code {
	case radius.CodeDisconnectACK:
		status = "acknowledged"
	case radius.CodeDisconnectNAK:
		status = "rejected"
	}

	return c.JSON(schemas.DisconnectResponse{
		Status:        status,
		Code:          response.Code.String(),
		NASIPAddress:  sess.NASIPAddress,
		Port:          port,
		Username:      sess.Username,
		AcctSessionID: sess.AcctSessionID,
	})
}
