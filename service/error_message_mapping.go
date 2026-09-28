package service

import (
	"bytes"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Client-facing protocol families that the error-message mapping understands.
// They are derived from the request path, not from the relay kit format, so the
// mapping targets what the client actually receives.
const (
	ErrorMessageMappingClientOpenAI    = "openai"
	ErrorMessageMappingClientClaude    = "claude"
	ErrorMessageMappingClientResponses = "responses"
)

const errorMessageMappingContextKey = "error_message_mapping_context"

var (
	errorMessageMappingErrorLiteral = []byte(`"error"`)
	errorMessageMappingDoneLiteral  = []byte("[DONE]")
	// errorMessageMappingClaudeStopLiteral is Claude's terminal non-error event.
	errorMessageMappingClaudeStopLiteral = []byte(`"message_stop"`)
	// errorMessageMappingResponsesTerminalLiterals are the Responses stream
	// terminal/error event type literals. The streaming hot path only parses
	// JSON when one of these is present; ordinary delta events skip the parse.
	errorMessageMappingResponsesTerminalLiterals = [][]byte{
		[]byte(`"response.completed"`),
		[]byte(`"response.done"`),
		[]byte(`"response.incomplete"`),
		[]byte(`"response.failed"`),
		[]byte(`"response.cancelled"`),
		[]byte(`"response.canceled"`),
		[]byte(`"response.error"`),
		[]byte(`"error"`),
	}
)

// errorMessageMappingRequest is the per-request state of the feature. The
// snapshot is captured once at Relay entry so a concurrent save cannot change
// the rules mid-request; terminal tracks whether a terminal event has already
// been emitted to the client.
type errorMessageMappingRequest struct {
	snapshot *model.ErrorMessageMappingSnapshot
	client   string
	terminal atomic.Bool
}

// BeginErrorMessageMapping records the immutable snapshot for this request when
// the request path is one of the four supported HTTP entries. It is a no-op for
// every other entry, which keeps the feature inside its frozen scope.
func BeginErrorMessageMapping(c *gin.Context) {
	if c == nil || c.Request == nil {
		return
	}
	client, ok := errorMessageMappingPathClient(c.Request.URL.Path)
	if !ok {
		return
	}
	c.Set(errorMessageMappingContextKey, &errorMessageMappingRequest{
		snapshot: model.CurrentErrorMessageMapping(),
		client:   client,
	})
}

func errorMessageMappingPathClient(path string) (string, bool) {
	switch path {
	case "/v1/chat/completions":
		return ErrorMessageMappingClientOpenAI, true
	case "/v1/messages":
		return ErrorMessageMappingClientClaude, true
	case "/v1/responses", "/v1/responses/compact":
		return ErrorMessageMappingClientResponses, true
	default:
		return "", false
	}
}

func errorMessageMapping(c *gin.Context) *errorMessageMappingRequest {
	if c == nil {
		return nil
	}
	value, ok := c.Get(errorMessageMappingContextKey)
	if !ok {
		return nil
	}
	request, _ := value.(*errorMessageMappingRequest)
	return request
}

// ErrorMessageMappingActive reports whether this request is inside the feature
// scope. Callers use it to limit commit-aware error output to those entries.
func ErrorMessageMappingActive(c *gin.Context) bool {
	return errorMessageMapping(c) != nil
}

// ErrorMessageMappingClientName returns the client protocol family for this
// request, or "" when the request is out of scope.
func ErrorMessageMappingClientName(c *gin.Context) string {
	if request := errorMessageMapping(c); request != nil {
		return request.client
	}
	return ""
}

// ApplyErrorMessageMapping matches an HTTP error message against the active
// snapshot. It strips a trailing request id that exactly equals this request's
// local id before matching, because the matching text must be the original
// message rather than the decorated one. It returns ("", false) on a miss or
// when the feature is out of scope or disabled.
func ApplyErrorMessageMapping(c *gin.Context, message string) (string, bool) {
	request := errorMessageMapping(c)
	if request == nil || request.snapshot == nil || request.snapshot.Matcher == nil {
		return "", false
	}
	message = stripErrorMessageMappingRequestID(message, c.GetString(common.RequestIdKey))
	result := request.snapshot.Matcher.Match(message)
	if !result.Matched {
		return "", false
	}
	return result.Message, true
}

// AppendErrorMessageMappingRequestID decorates a matched replacement with this
// request's id, once, when the id is known. A replacement that already carries
// this exact local suffix is left as is. A miss never calls this, so the
// existing output is preserved byte for byte.
func AppendErrorMessageMappingRequestID(message, requestId string) string {
	if requestId == "" {
		return message
	}
	if strings.HasSuffix(message, " (request id: "+requestId+")") {
		return message
	}
	return common.MessageWithRequestId(message, requestId)
}

func stripErrorMessageMappingRequestID(message, requestId string) string {
	if requestId == "" {
		return message
	}
	return strings.TrimSuffix(message, " (request id: "+requestId+")")
}

// MarkErrorMessageMappingTerminal records that a terminal event for the client
// has already been written. A later handler error must not append a second one.
func MarkErrorMessageMappingTerminal(c *gin.Context) {
	if request := errorMessageMapping(c); request != nil {
		request.terminal.Store(true)
	}
}

// ErrorMessageMappingTerminalEmitted reports whether a terminal event has
// already been written for this request.
func ErrorMessageMappingTerminalEmitted(c *gin.Context) bool {
	if request := errorMessageMapping(c); request != nil {
		return request.terminal.Load()
	}
	return false
}

// ErrorMessageMappingCommittedSSE reports whether this in-scope request has
// already committed an SSE response. When true, a trailing error must be sent
// as a protocol event instead of a plain HTTP JSON body.
func ErrorMessageMappingCommittedSSE(c *gin.Context) bool {
	if errorMessageMapping(c) == nil {
		return false
	}
	if c.Writer == nil || !c.Writer.Written() {
		return false
	}
	return strings.HasPrefix(c.Writer.Header().Get("Content-Type"), "text/event-stream")
}

// ResponseCommitted reports whether the response has been committed at all.
func ResponseCommitted(c *gin.Context) bool {
	return c != nil && c.Writer != nil && c.Writer.Written()
}

// PrepareErrorMessageMappingSSEFrame maps one serialized SSE event payload in
// place for the client protocol and reports whether the payload is a terminal
// event. Terminal classification is independent of the matching switch so the
// "do not append after a terminal event" guard still works when mapping is off.
func PrepareErrorMessageMappingSSEFrame(c *gin.Context, payload []byte) ([]byte, bool) {
	request := errorMessageMapping(c)
	if request == nil || len(payload) == 0 {
		return payload, false
	}
	switch request.client {
	case ErrorMessageMappingClientOpenAI:
		return prepareOpenAISSEMapping(request, payload)
	case ErrorMessageMappingClientClaude:
		return prepareClaudeSSEMapping(request, payload)
	case ErrorMessageMappingClientResponses:
		return prepareResponsesSSEMapping(request, payload)
	default:
		return payload, false
	}
}

func prepareOpenAISSEMapping(request *errorMessageMappingRequest, payload []byte) ([]byte, bool) {
	if bytes.Equal(payload, errorMessageMappingDoneLiteral) {
		return payload, true
	}
	// Fast path: a normal chunk never carries an error object, so checking the
	// literal first lets the streaming hot path skip a full JSON parse per
	// chunk. Only a complete JSON document is a protocol error frame; a
	// truncated or malformed payload is passed through and never terminal.
	if !bytes.Contains(payload, errorMessageMappingErrorLiteral) {
		return payload, false
	}
	if !gjson.ValidBytes(payload) {
		return payload, false
	}
	errorValue := gjson.GetBytes(payload, "error")
	if !errorValue.IsObject() {
		return payload, false
	}
	if choices := gjson.GetBytes(payload, "choices"); choices.IsArray() && len(choices.Array()) > 0 {
		return payload, false
	}
	message := errorValue.Get("message")
	if message.Type != gjson.String {
		return payload, true
	}
	if mapped, ok := mapErrorMessageMappingText(request, message.String()); ok {
		return setErrorMessageMappingField(payload, "error.message", mapped), true
	}
	return payload, true
}

func prepareClaudeSSEMapping(request *errorMessageMappingRequest, payload []byte) ([]byte, bool) {
	// Fast path: only message_stop (terminal) or error (terminal + mapping)
	// events need a JSON parse; ordinary content events skip it.
	if !bytes.Contains(payload, errorMessageMappingClaudeStopLiteral) &&
		!bytes.Contains(payload, errorMessageMappingErrorLiteral) {
		return payload, false
	}
	if !gjson.ValidBytes(payload) {
		return payload, false
	}
	switch gjson.GetBytes(payload, "type").String() {
	case "message_stop":
		return payload, true
	case "error":
		message := gjson.GetBytes(payload, "error.message")
		if message.Type != gjson.String {
			return payload, true
		}
		if mapped, ok := mapErrorMessageMappingText(request, message.String()); ok {
			return setErrorMessageMappingField(payload, "error.message", mapped), true
		}
		return payload, true
	default:
		return payload, false
	}
}

func prepareResponsesSSEMapping(request *errorMessageMappingRequest, payload []byte) ([]byte, bool) {
	// Fast path: only terminal completion events and error events need a JSON
	// parse; ordinary delta events skip it on the streaming hot path.
	if !slices.ContainsFunc(errorMessageMappingResponsesTerminalLiterals, func(literal []byte) bool {
		return bytes.Contains(payload, literal)
	}) {
		return payload, false
	}
	if !gjson.ValidBytes(payload) {
		return payload, false
	}
	switch gjson.GetBytes(payload, "type").String() {
	case "response.completed", "response.done", "response.incomplete",
		"response.failed", "response.cancelled", "response.canceled",
		"error", "response.error":
	default:
		return payload, false
	}

	switch gjson.GetBytes(payload, "type").String() {
	case "error", "response.error":
		// When an error object is present, its message is the contract leaf.
		// A missing or non-string leaf stays untouched and the top-level
		// message is never used as a fallback.
		if errorValue := gjson.GetBytes(payload, "error"); errorValue.IsObject() {
			if message := errorValue.Get("message"); message.Type == gjson.String {
				if mapped, ok := mapErrorMessageMappingText(request, message.String()); ok {
					return setErrorMessageMappingField(payload, "error.message", mapped), true
				}
			}
			return payload, true
		}
		if message := gjson.GetBytes(payload, "message"); message.Type == gjson.String {
			if mapped, ok := mapErrorMessageMappingText(request, message.String()); ok {
				return setErrorMessageMappingField(payload, "message", mapped), true
			}
		}
		return payload, true
	case "response.failed":
		message := gjson.GetBytes(payload, "response.error.message")
		if message.Type == gjson.String {
			if mapped, ok := mapErrorMessageMappingText(request, message.String()); ok {
				return setErrorMessageMappingField(payload, "response.error.message", mapped), true
			}
		}
		return payload, true
	default:
		return payload, true
	}
}

// MapResponsesFailedBody rewrites the message of a native Responses HTTP 200
// body whose status is "failed". The original object has already been parsed
// for observation; only the serialized client bytes are patched.
func MapResponsesFailedBody(c *gin.Context, payload []byte) []byte {
	request := errorMessageMapping(c)
	if request == nil || len(payload) == 0 {
		return payload
	}
	if gjson.GetBytes(payload, "status").String() != "failed" {
		return payload
	}
	message := gjson.GetBytes(payload, "error.message")
	if message.Type != gjson.String {
		return payload
	}
	if mapped, ok := mapErrorMessageMappingText(request, message.String()); ok {
		return setErrorMessageMappingField(payload, "error.message", mapped)
	}
	return payload
}

// mapErrorMessageMappingText masks the candidate leaf with the existing relay
// masking helper before matching, so a rule keyword cannot be defeated by
// redaction and the mapping never sees credentials. A miss keeps the original
// text, so the passthrough output is unchanged.
func mapErrorMessageMappingText(request *errorMessageMappingRequest, message string) (string, bool) {
	if request == nil || request.snapshot == nil || request.snapshot.Matcher == nil {
		return "", false
	}
	result := request.snapshot.Matcher.Match(kitutil.MaskSensitiveInfo(message))
	if !result.Matched {
		return "", false
	}
	return result.Message, true
}

func setErrorMessageMappingField(payload []byte, path string, value string) []byte {
	working := make([]byte, len(payload))
	copy(working, payload)
	updated, err := sjson.SetBytes(working, path, value)
	if err != nil {
		return payload
	}
	return updated
}
