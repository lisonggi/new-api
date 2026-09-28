package helper

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func FlushWriter(c *gin.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("flush panic recovered: %v", r)
		}
	}()

	if c == nil || c.Writer == nil {
		return nil
	}

	if requestContextDone(c) {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return errors.New("streaming error: flusher not found")
	}

	flusher.Flush()
	return nil
}

func requestContextDone(c *gin.Context) bool {
	return c != nil && c.Request != nil && c.Request.Context().Err() != nil
}

// errorMessageMappingWriteRecorder wraps the response writer to record whether
// the underlying writer accepted every byte. gin's ResponseWriter exposes
// WriteString (uppercase), so common.CustomEvent never takes the lowercase
// writeString fast path and all of its bytes go through Write; this wrapper
// therefore observes every SSE byte without changing the encoder.
type errorMessageMappingWriteRecorder struct {
	gin.ResponseWriter
	err error
}

func (w *errorMessageMappingWriteRecorder) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	if err == nil && n < len(p) {
		err = io.ErrShortWrite
	}
	if err != nil && w.err == nil {
		w.err = err
	}
	return n, err
}

// renderErrorMessageMappingFrame renders SSE data parts with the same
// common.CustomEvent encoder as before. When terminal is false it uses the
// original render-and-flush path, so non-terminal and out-of-scope writes keep
// their exact bytes and cost. When terminal is true it wraps the writer to
// record the real local write result: a nil error means the writer accepted
// every byte and the flush succeeded, so the caller may record a terminal
// event; a failed or short write leaves it unset.
func renderErrorMessageMappingFrame(c *gin.Context, terminal bool, parts ...string) error {
	if !terminal {
		for _, part := range parts {
			c.Render(-1, common.CustomEvent{Data: part})
		}
		return FlushWriter(c)
	}

	recorder := &errorMessageMappingWriteRecorder{ResponseWriter: c.Writer}
	original := c.Writer
	c.Writer = recorder
	defer func() { c.Writer = original }()

	for _, part := range parts {
		c.Render(-1, common.CustomEvent{Data: part})
		if recorder.err != nil {
			return recorder.err
		}
	}
	return FlushWriter(c)
}

func SetEventStreamHeaders(c *gin.Context) {
	// 检查是否已经设置过头部
	if _, exists := c.Get("event_stream_headers_set"); exists {
		return
	}

	// 设置标志，表示头部已经设置过
	c.Set("event_stream_headers_set", true)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
}

func ClaudeData(c *gin.Context, resp dto.ClaudeResponse) error {
	if requestContextDone(c) {
		return nil
	}

	jsonData, err := common.Marshal(resp)
	if err != nil {
		common.SysError("error marshalling stream response: " + err.Error())
	} else {
		mapped, terminal := service.PrepareErrorMessageMappingSSEFrame(c, jsonData)
		writeErr := renderErrorMessageMappingFrame(
			c,
			terminal,
			fmt.Sprintf("event: %s\n", resp.Type),
			"data: "+string(mapped),
		)
		// Terminal is only recorded when the frame actually reached the
		// client; a failed write must not suppress a later error event.
		if writeErr == nil && terminal {
			service.MarkErrorMessageMappingTerminal(c)
		}
	}
	return nil
}

func ClaudeChunkData(c *gin.Context, resp dto.ClaudeResponse, data string) {
	if requestContextDone(c) {
		return
	}

	mapped, terminal := service.PrepareErrorMessageMappingSSEFrame(c, []byte(data))
	writeErr := renderErrorMessageMappingFrame(
		c,
		terminal,
		fmt.Sprintf("event: %s\n", resp.Type),
		fmt.Sprintf("data: %s\n", string(mapped)),
	)
	if writeErr == nil && terminal {
		service.MarkErrorMessageMappingTerminal(c)
	}
}

func ResponseChunkData(c *gin.Context, resp dto.ResponsesStreamResponse, data string) error {
	if requestContextDone(c) {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}

	mapped, terminal := service.PrepareErrorMessageMappingSSEFrame(c, []byte(data))
	if err := renderErrorMessageMappingFrame(
		c,
		terminal,
		fmt.Sprintf("event: %s\n", resp.Type),
		fmt.Sprintf("data: %s", string(mapped)),
	); err != nil {
		return err
	}
	if terminal {
		service.MarkErrorMessageMappingTerminal(c)
	}
	return nil
}

func StringData(c *gin.Context, str string) error {
	if c == nil || c.Writer == nil {
		return errors.New("context or writer is nil")
	}

	if requestContextDone(c) {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}

	mapped, terminal := service.PrepareErrorMessageMappingSSEFrame(c, []byte(str))
	err := renderErrorMessageMappingFrame(c, terminal, "data: "+string(mapped))
	if err == nil && terminal {
		service.MarkErrorMessageMappingTerminal(c)
	}
	return err
}

func PingData(c *gin.Context) error {
	if c == nil || c.Writer == nil {
		return errors.New("context or writer is nil")
	}

	if requestContextDone(c) {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}

	if _, err := c.Writer.Write([]byte(": PING\n\n")); err != nil {
		return fmt.Errorf("write ping data failed: %w", err)
	}
	return FlushWriter(c)
}

func ObjectData(c *gin.Context, object any) error {
	if object == nil {
		return errors.New("object is nil")
	}
	jsonData, err := common.Marshal(object)
	if err != nil {
		return fmt.Errorf("error marshalling object: %w", err)
	}
	return StringData(c, string(jsonData))
}

func Done(c *gin.Context) {
	_ = StringData(c, "[DONE]")
}

func WssString(c *gin.Context, ws *websocket.Conn, str string) error {
	if ws == nil {
		logger.LogError(c, "websocket connection is nil")
		return errors.New("websocket connection is nil")
	}
	//common.LogInfo(c, fmt.Sprintf("sending message: %s", str))
	return ws.WriteMessage(1, []byte(str))
}

func WssObject(c *gin.Context, ws *websocket.Conn, object any) error {
	jsonData, err := common.Marshal(object)
	if err != nil {
		return fmt.Errorf("error marshalling object: %w", err)
	}
	if ws == nil {
		logger.LogError(c, "websocket connection is nil")
		return errors.New("websocket connection is nil")
	}
	//common.LogInfo(c, fmt.Sprintf("sending message: %s", jsonData))
	return ws.WriteMessage(1, jsonData)
}

func WssError(c *gin.Context, ws *websocket.Conn, openaiError types.OpenAIError) {
	if ws == nil {
		return
	}
	errorObj := &dto.RealtimeEvent{
		Type:    "error",
		EventId: GetLocalRealtimeID(c),
		Error:   &openaiError,
	}
	_ = WssObject(c, ws, errorObj)
}

func GetResponseID(c *gin.Context) string {
	logID := c.GetString(common.RequestIdKey)
	return fmt.Sprintf("chatcmpl-%s", logID)
}

func GetLocalRealtimeID(c *gin.Context) string {
	logID := c.GetString(common.RequestIdKey)
	return fmt.Sprintf("evt_%s", logID)
}

func GenerateStartEmptyResponse(id string, createAt int64, model string, systemFingerprint *string) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:                id,
		Object:            "chat.completion.chunk",
		Created:           createAt,
		Model:             model,
		SystemFingerprint: systemFingerprint,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role:    "assistant",
					Content: common.GetPointer(""),
				},
			},
		},
	}
}

func GenerateStopResponse(id string, createAt int64, model string, finishReason string) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:                id,
		Object:            "chat.completion.chunk",
		Created:           createAt,
		Model:             model,
		SystemFingerprint: nil,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				FinishReason: &finishReason,
			},
		},
	}
}

func GenerateFinalUsageResponse(id string, createAt int64, model string, usage dto.Usage) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:                id,
		Object:            "chat.completion.chunk",
		Created:           createAt,
		Model:             model,
		SystemFingerprint: nil,
		Choices:           make([]dto.ChatCompletionsStreamResponseChoice, 0),
		Usage:             &usage,
	}
}
