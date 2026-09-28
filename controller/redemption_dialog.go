package controller

import (
	"errors"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/redemption_dialog"

	"github.com/gin-gonic/gin"
)

// GetRedemptionDialog returns the active, validated redemption-success-dialog
// config. It never returns an unvalidated database value.
func GetRedemptionDialog(c *gin.Context) {
	snapshot := model.CurrentRedemptionDialog()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    snapshot.Config,
	})
}

// UpdateRedemptionDialog replaces the whole config and publishes it only after
// the database write commits.
func UpdateRedemptionDialog(c *gin.Context) {
	body, ok := readRedemptionDialogBody(c, redemption_dialog.MaxConfigBodyBytes)
	if !ok {
		return
	}
	cfg, err := redemption_dialog.ParseConfig(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": redemptionDialogValidationMessage(c, err)})
		return
	}
	saved, err := model.SaveRedemptionDialog(cfg)
	if err != nil {
		logger.LogError(c, "failed to save redemption success dialog: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": i18n.T(c, i18n.MsgRedemptionDialogSaveFailed)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": saved})
}

// redemptionDialogValidationKeyPrefix namespaces the localized validation
// messages. Each redemption_dialog.ErrorCode is appended to build the key.
const redemptionDialogValidationKeyPrefix = "redemption_dialog.validation."

// redemptionDialogValidationMessage renders a ParseConfig failure as a localized
// message. The setting package stays i18n-free: it reports a stable code and
// template parameters, and only the host maps them to a translation.
func redemptionDialogValidationMessage(c *gin.Context, err error) string {
	var configErr *redemption_dialog.ConfigError
	if errors.As(err, &configErr) {
		key := redemptionDialogValidationKeyPrefix + string(configErr.Code())
		if message := i18n.T(c, key, configErr.Params()); message != key {
			return message
		}
	}
	return err.Error()
}

func readRedemptionDialogBody(c *gin.Context, limit int64) ([]byte, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"success": false, "message": i18n.T(c, i18n.MsgRedemptionDialogBodyTooLarge)})
			return nil, false
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": i18n.T(c, i18n.MsgRedemptionDialogBodyReadFailed)})
		return nil, false
	}
	return body, true
}
