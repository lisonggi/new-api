package controller

import (
	"errors"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/error_mapping"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// writeErrorMessageMappedRelayError is the single client-facing outlet for the
// final Relay error. It keeps the original error object untouched (logging,
// retry and channel health already ran), builds the legacy response on a
// shallow copy, and only replaces the client-visible message on a rule match.
// Inside the four supported entries it also refuses to append a JSON body to an
// already-committed SSE stream and sends a protocol error event instead.
func writeErrorMessageMappedRelayError(c *gin.Context, ws *websocket.Conn, newAPIError *types.NewAPIError, relayFormat types.RelayFormat, requestId string) {
	if relayFormat == types.RelayFormatOpenAIRealtime {
		newAPIError.SetMessage(common.MessageWithRequestId(newAPIError.Error(), requestId))
		helper.WssError(c, ws, newAPIError.ToOpenAIError())
		return
	}

	inScope := service.ErrorMessageMappingActive(c)
	// The legacy response applies the existing request-id decoration. It is
	// built on a copy so the original error (used for logs/retry/health) keeps
	// its original message.
	legacy := *newAPIError
	legacy.SetMessage(common.MessageWithRequestId(newAPIError.Error(), requestId))

	if relayFormat == types.RelayFormatClaude {
		claudeError := legacy.ToClaudeError()
		if replacement, matched := service.ApplyErrorMessageMapping(c, newAPIError.ToClaudeError().Message); matched {
			claudeError.Message = service.AppendErrorMessageMappingRequestID(replacement, requestId)
		}
		if inScope && service.ErrorMessageMappingCommittedSSE(c) {
			if !service.ErrorMessageMappingTerminalEmitted(c) {
				writeClaudeSSEError(c, claudeError)
			}
			return
		}
		if inScope && service.ResponseCommitted(c) {
			return
		}
		c.JSON(newAPIError.StatusCode, gin.H{
			"type":  "error",
			"error": claudeError,
		})
		return
	}

	openAIError := legacy.ToOpenAIError()
	if replacement, matched := service.ApplyErrorMessageMapping(c, newAPIError.ToOpenAIError().Message); matched {
		openAIError.Message = service.AppendErrorMessageMappingRequestID(replacement, requestId)
	}
	if inScope && service.ErrorMessageMappingCommittedSSE(c) {
		if !service.ErrorMessageMappingTerminalEmitted(c) {
			if service.ErrorMessageMappingClientName(c) == service.ErrorMessageMappingClientResponses {
				writeResponsesSSEError(c, openAIError)
			} else {
				writeOpenAISSEError(c, openAIError)
			}
		}
		return
	}
	if inScope && service.ResponseCommitted(c) {
		return
	}
	c.JSON(newAPIError.StatusCode, gin.H{
		"error": openAIError,
	})
}

// The three writers below emit the protocol's own terminal error event. They do
// not run the mapping again: the message is already final.
func writeOpenAISSEError(c *gin.Context, openAIError types.OpenAIError) {
	payload, err := common.Marshal(gin.H{"error": openAIError})
	if err != nil {
		return
	}
	c.Render(-1, common.CustomEvent{Data: "data: " + string(payload)})
	_ = helper.FlushWriter(c)
}

func writeClaudeSSEError(c *gin.Context, claudeError types.ClaudeError) {
	payload, err := common.Marshal(gin.H{"type": "error", "error": claudeError})
	if err != nil {
		return
	}
	c.Render(-1, common.CustomEvent{Data: "event: error\n"})
	c.Render(-1, common.CustomEvent{Data: "data: " + string(payload)})
	_ = helper.FlushWriter(c)
}

func writeResponsesSSEError(c *gin.Context, openAIError types.OpenAIError) {
	payload, err := common.Marshal(gin.H{
		"type":    "error",
		"code":    openAIError.Code,
		"message": openAIError.Message,
		"param":   openAIError.Param,
	})
	if err != nil {
		return
	}
	c.Render(-1, common.CustomEvent{Data: "event: error\n"})
	c.Render(-1, common.CustomEvent{Data: "data: " + string(payload)})
	_ = helper.FlushWriter(c)
}

// GetErrorMessageMapping returns the active, validated error-message-mapping
// config. It never returns an unvalidated database value.
func GetErrorMessageMapping(c *gin.Context) {
	snapshot := model.CurrentErrorMessageMapping()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    snapshot.Config,
	})
}

// UpdateErrorMessageMapping replaces the whole config and publishes it only
// after the database write commits.
func UpdateErrorMessageMapping(c *gin.Context) {
	body, ok := readErrorMessageMappingBody(c, error_mapping.MaxConfigBodyBytes)
	if !ok {
		return
	}
	cfg, err := error_mapping.ParseConfig(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": errorMessageMappingValidationMessage(c, err)})
		return
	}
	saved, err := model.SaveErrorMessageMapping(cfg)
	if err != nil {
		logger.LogError(c, "failed to save error message mapping: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": i18n.T(c, i18n.MsgErrorMappingSaveFailed)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": saved})
}

// PreviewErrorMessageMapping validates a draft config and matches one sample
// message with the same implementation used at runtime. It stores nothing.
func PreviewErrorMessageMapping(c *gin.Context) {
	body, ok := readErrorMessageMappingBody(c, error_mapping.MaxPreviewBodyBytes)
	if !ok {
		return
	}
	var request struct {
		Config  common.RawMessage `json:"config"`
		Message *string           `json:"message"`
	}
	if err := common.Unmarshal(body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": i18n.T(c, i18n.MsgErrorMappingInvalidRequest)})
		return
	}
	if request.Message == nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": i18n.T(c, i18n.MsgErrorMappingMessageRequired)})
		return
	}
	if len(*request.Message) > error_mapping.MaxPreviewMessageBytes {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": i18n.T(c, i18n.MsgErrorMappingMessageTooLong)})
		return
	}
	if len(request.Config) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": i18n.T(c, i18n.MsgErrorMappingConfigRequired)})
		return
	}
	cfg, err := error_mapping.ParseConfig(request.Config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": errorMessageMappingValidationMessage(c, err)})
		return
	}
	matcher, err := error_mapping.Compile(cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": errorMessageMappingValidationMessage(c, err)})
		return
	}
	result := matcher.Match(*request.Message)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{
		"matched": result.Matched,
		"rule_id": result.RuleID,
		"message": result.Message,
	}})
}

// errorMessageMappingValidationKeyPrefix namespaces the localized validation
// messages. Each error_mapping.ErrorCode is appended to build the message key.
const errorMessageMappingValidationKeyPrefix = "error_mapping.validation."

// errorMessageMappingValidationMessage renders a ParseConfig/Compile failure as
// a localized message. The setting package stays i18n-free: it reports a stable
// code and template parameters, and only the host maps them to a translation.
func errorMessageMappingValidationMessage(c *gin.Context, err error) string {
	var configErr *error_mapping.ConfigError
	if errors.As(err, &configErr) {
		key := errorMessageMappingValidationKeyPrefix + string(configErr.Code())
		if message := i18n.T(c, key, configErr.Params()); message != key {
			return message
		}
	}
	return err.Error()
}

func readErrorMessageMappingBody(c *gin.Context, limit int64) ([]byte, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"success": false, "message": i18n.T(c, i18n.MsgErrorMappingBodyTooLarge)})
			return nil, false
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": i18n.T(c, i18n.MsgErrorMappingBodyReadFailed)})
		return nil, false
	}
	return body, true
}
