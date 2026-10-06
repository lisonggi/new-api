package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/error_mapping"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResetStatusCode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		statusCode       int
		statusCodeConfig string
		expectedCode     int
	}{
		{
			name:             "map string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"503"}`,
			expectedCode:     503,
		},
		{
			name:             "map int value",
			statusCode:       429,
			statusCodeConfig: `{"429":503}`,
			expectedCode:     503,
		},
		{
			name:             "skip invalid string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"bad-code"}`,
			expectedCode:     429,
		},
		{
			name:             "skip status code 200",
			statusCode:       200,
			statusCodeConfig: `{"200":503}`,
			expectedCode:     200,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			newAPIError := &types.NewAPIError{
				StatusCode: tc.statusCode,
			}
			ResetStatusCode(newAPIError, tc.statusCodeConfig)
			require.Equal(t, tc.expectedCode, newAPIError.StatusCode)
		})
	}
}

func TestRelayErrorHandlerTruncatesInvalidJSONBodyInLog(t *testing.T) {
	withDebugEnabled(t, false)

	body := strings.Repeat("b", common.LocalLogContentLimit+256)
	var logBuffer bytes.Buffer

	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, "bad response status code 500", newAPIError.Error())
	require.Contains(t, logBuffer.String(), "[truncated")
	require.Contains(t, logBuffer.String(), fmt.Sprintf("original_length=%d", len(body)))
	require.NotContains(t, logBuffer.String(), strings.Repeat("b", common.LocalLogContentLimit+1))
}

func TestRelayErrorHandlerKeepsStructuredErrorMessage(t *testing.T) {
	message := strings.Repeat("c", common.LocalLogContentLimit+256)
	body := `{"message":"` + message + `"}`
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, message, newAPIError.Error())
}

func TestRelayErrorHandlerKeepsOpenAIErrorMessage(t *testing.T) {
	message := strings.Repeat("d", common.LocalLogContentLimit+256)
	body := `{"error":{"message":"` + message + `","type":"server_error","code":"server_error"}}`
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, message, newAPIError.Error())
}

func TestRelayErrorHandlerMarksUpstreamHTTPErrorPreservingWireStatus(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"openai error object", `{"error":{"message":"boom","type":"server_error","code":"server_error"}}`},
		{"message only", `{"message":"boom"}`},
		{"invalid json", `not-json`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(strings.NewReader(tc.body)),
			}
			newAPIError := RelayErrorHandler(context.Background(), resp, false)
			require.NotNil(t, newAPIError)

			info := newAPIError.GetUpstreamHTTPError()
			require.NotNil(t, info)
			require.Equal(t, http.StatusTooManyRequests, info.OriginalStatusCode)

			// Local status mapping must not rewrite the original wire status.
			ResetStatusCode(newAPIError, `{"429":503}`)
			require.Equal(t, http.StatusServiceUnavailable, newAPIError.StatusCode)
			require.Equal(t, http.StatusTooManyRequests, newAPIError.GetUpstreamHTTPError().OriginalStatusCode)
		})
	}
}

func TestLocallySynthesizedErrorsHaveNoUpstreamHTTPMarker(t *testing.T) {
	require.Nil(t, types.NewError(errors.New("dial failed"), types.ErrorCodeDoRequestFailed).GetUpstreamHTTPError())
	require.Nil(t, (*types.NewAPIError)(nil).GetUpstreamHTTPError())
}

func TestRelayErrorHandlerKeepsInvalidJSONBodyInDebugLog(t *testing.T) {
	withDebugEnabled(t, true)

	body := strings.Repeat("e", common.LocalLogContentLimit+256)
	var logBuffer bytes.Buffer

	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.NotContains(t, logBuffer.String(), "[truncated")
	require.Contains(t, logBuffer.String(), body)
}

func withDebugEnabled(t *testing.T, enabled bool) {
	t.Helper()

	oldDebug := common.DebugEnabled
	common.DebugEnabled = enabled
	t.Cleanup(func() {
		common.DebugEnabled = oldDebug
	})
}

const errorMessageMappingReasoningConfig = `{"enabled":true,"rules":[{"id":"r1","enabled":true,"keyword":"reasoning_content","case_sensitive":false,"replacement":"REPLACED"}]}`

func newErrorMessageMappingTestContext(t *testing.T, client, configJSON string) *gin.Context {
	t.Helper()
	cfg, err := error_mapping.ParseConfig([]byte(configJSON))
	require.NoError(t, err)
	matcher, err := error_mapping.Compile(cfg)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set(errorMessageMappingContextKey, &errorMessageMappingRequest{
		snapshot: &model.ErrorMessageMappingSnapshot{Config: cfg, JSON: "", Matcher: matcher},
		client:   client,
	})
	return c
}

func TestBeginErrorMessageMappingPathScope(t *testing.T) {
	for path, want := range map[string]bool{
		"/v1/chat/completions":   true,
		"/v1/messages":           true,
		"/v1/responses":          true,
		"/v1/responses/compact":  true,
		"/v1/embeddings":         false,
		"/v1/images/generations": false,
		"/v1/realtime":           false,
	} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, path, nil)
		BeginErrorMessageMapping(c)
		require.Equal(t, want, ErrorMessageMappingActive(c), path)
	}
}

func TestPrepareErrorMessageMappingSSEFrame(t *testing.T) {
	cases := []struct {
		name     string
		client   string
		payload  string
		want     string
		terminal bool
	}{
		{"chat done", ErrorMessageMappingClientOpenAI, `[DONE]`, `[DONE]`, true},
		{
			"chat error frame",
			ErrorMessageMappingClientOpenAI,
			`{"error":{"message":"bad reasoning_content","type":"invalid_request_error","code":"x"},"id":"req1","created":1}`,
			`{"error":{"message":"REPLACED","type":"invalid_request_error","code":"x"},"id":"req1","created":1}`,
			true,
		},
		{
			"chat content chunk is untouched",
			ErrorMessageMappingClientOpenAI,
			`{"choices":[{"delta":{"content":"reasoning_content mentioned"}}]}`,
			`{"choices":[{"delta":{"content":"reasoning_content mentioned"}}]}`,
			false,
		},
		{
			"chat error with choices is a success payload",
			ErrorMessageMappingClientOpenAI,
			`{"choices":[{"delta":{"content":"x"}}],"error":{"message":"reasoning_content"}}`,
			`{"choices":[{"delta":{"content":"x"}}],"error":{"message":"reasoning_content"}}`,
			false,
		},
		{"chat non-string message", ErrorMessageMappingClientOpenAI, `{"error":{"message":123}}`, `{"error":{"message":123}}`, true},
		{"claude message_stop", ErrorMessageMappingClientClaude, `{"type":"message_stop"}`, `{"type":"message_stop"}`, true},
		{
			"claude error",
			ErrorMessageMappingClientClaude,
			`{"type":"error","error":{"type":"invalid_request_error","message":"reasoning_content"}}`,
			`{"type":"error","error":{"type":"invalid_request_error","message":"REPLACED"}}`,
			true,
		},
		{"claude delta is untouched", ErrorMessageMappingClientClaude, `{"type":"content_block_delta","delta":{"text":"reasoning_content"}}`, `{"type":"content_block_delta","delta":{"text":"reasoning_content"}}`, false},
		{"responses completed", ErrorMessageMappingClientResponses, `{"type":"response.completed"}`, `{"type":"response.completed"}`, true},
		{
			"responses failed",
			ErrorMessageMappingClientResponses,
			`{"type":"response.failed","response":{"error":{"message":"reasoning_content","code":"x"},"id":"resp_1"}}`,
			`{"type":"response.failed","response":{"error":{"message":"REPLACED","code":"x"},"id":"resp_1"}}`,
			true,
		},
		{"responses error top-level message", ErrorMessageMappingClientResponses, `{"type":"error","message":"reasoning_content"}`, `{"type":"error","message":"REPLACED"}`, true},
		{"responses error object", ErrorMessageMappingClientResponses, `{"type":"error","error":{"message":"reasoning_content"}}`, `{"type":"error","error":{"message":"REPLACED"}}`, true},
		{"responses response.error top-level message", ErrorMessageMappingClientResponses, `{"type":"response.error","message":"reasoning_content"}`, `{"type":"response.error","message":"REPLACED"}`, true},
		{"responses response.error object", ErrorMessageMappingClientResponses, `{"type":"response.error","error":{"message":"reasoning_content"}}`, `{"type":"response.error","error":{"message":"REPLACED"}}`, true},
		{"responses created is untouched", ErrorMessageMappingClientResponses, `{"type":"response.created"}`, `{"type":"response.created"}`, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newErrorMessageMappingTestContext(t, tc.client, errorMessageMappingReasoningConfig)
			mapped, terminal := PrepareErrorMessageMappingSSEFrame(c, []byte(tc.payload))
			require.Equal(t, tc.terminal, terminal)
			if tc.want == "[DONE]" {
				require.Equal(t, tc.want, string(mapped))
				return
			}
			require.JSONEq(t, tc.want, string(mapped))
		})
	}
}

func TestPrepareErrorMessageMappingSSEFrameDisabledStillTracksTerminal(t *testing.T) {
	c := newErrorMessageMappingTestContext(t, ErrorMessageMappingClientOpenAI, `{"enabled":false,"rules":[{"id":"r1","enabled":true,"keyword":"reasoning_content","case_sensitive":false,"replacement":"REPLACED"}]}`)
	payload := `{"error":{"message":"bad reasoning_content"}}`
	mapped, terminal := PrepareErrorMessageMappingSSEFrame(c, []byte(payload))
	require.True(t, terminal)
	require.Equal(t, payload, string(mapped))
}

func TestPrepareErrorMessageMappingSSEFrameIsNotRecursive(t *testing.T) {
	const cfg = `{"enabled":true,"rules":[{"id":"a","enabled":true,"keyword":"alpha","case_sensitive":false,"replacement":"beta"},{"id":"b","enabled":true,"keyword":"beta","case_sensitive":false,"replacement":"gamma"}]}`
	c := newErrorMessageMappingTestContext(t, ErrorMessageMappingClientOpenAI, cfg)
	mapped, terminal := PrepareErrorMessageMappingSSEFrame(c, []byte(`{"error":{"message":"alpha"}}`))
	require.True(t, terminal)
	require.JSONEq(t, `{"error":{"message":"beta"}}`, string(mapped))
}

func TestPrepareErrorMessageMappingSSEFrameMasksBeforeMatching(t *testing.T) {
	const cfg = `{"enabled":true,"rules":[{"id":"domain","enabled":true,"keyword":"example.com","case_sensitive":false,"replacement":"REPLACED"}]}`
	c := newErrorMessageMappingTestContext(t, ErrorMessageMappingClientOpenAI, cfg)
	payload := `{"error":{"message":"failed at https://api.example.com/v1"}}`
	mapped, terminal := PrepareErrorMessageMappingSSEFrame(c, []byte(payload))
	require.True(t, terminal)
	require.Equal(t, payload, string(mapped), "the masked candidate no longer contains the raw domain keyword")
}

func TestPrepareErrorMessageMappingSSEFrameOutOfScopeIsUntouched(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	payload := []byte(`{"error":{"message":"reasoning_content"}}`)
	mapped, terminal := PrepareErrorMessageMappingSSEFrame(c, payload)
	require.False(t, terminal)
	require.Equal(t, string(payload), string(mapped))
}

func TestApplyErrorMessageMappingHTTP(t *testing.T) {
	c := newErrorMessageMappingTestContext(t, ErrorMessageMappingClientOpenAI, errorMessageMappingReasoningConfig)
	c.Set(common.RequestIdKey, "req-123")

	replacement, matched := ApplyErrorMessageMapping(c, "reasoning_content (request id: req-123)")
	require.True(t, matched)
	require.Equal(t, "REPLACED", replacement)

	_, matched = ApplyErrorMessageMapping(c, "unrelated error")
	require.False(t, matched)

	require.Equal(t, "REPLACED (request id: req-123)", AppendErrorMessageMappingRequestID("REPLACED", "req-123"))
	require.Equal(t, "REPLACED (request id: req-123)", AppendErrorMessageMappingRequestID("REPLACED (request id: req-123)", "req-123"))
	require.Equal(t, "REPLACED", AppendErrorMessageMappingRequestID("REPLACED", ""))

	plain, _ := gin.CreateTestContext(httptest.NewRecorder())
	plain.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	_, matched = ApplyErrorMessageMapping(plain, "reasoning_content")
	require.False(t, matched, "an uninitialized context never maps")
}

func TestErrorMessageMappingTerminalAndCommitState(t *testing.T) {
	c := newErrorMessageMappingTestContext(t, ErrorMessageMappingClientOpenAI, errorMessageMappingReasoningConfig)
	require.False(t, ErrorMessageMappingTerminalEmitted(c))
	MarkErrorMessageMappingTerminal(c)
	require.True(t, ErrorMessageMappingTerminalEmitted(c))

	require.False(t, ErrorMessageMappingCommittedSSE(c))
	require.False(t, ResponseCommitted(c))
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.WriteHeaderNow()
	require.True(t, ResponseCommitted(c))
	require.True(t, ErrorMessageMappingCommittedSSE(c))
}

func TestMapResponsesFailedBody(t *testing.T) {
	c := newErrorMessageMappingTestContext(t, ErrorMessageMappingClientResponses, errorMessageMappingReasoningConfig)
	body := `{"id":"resp_1","status":"failed","error":{"message":"reasoning_content","code":"x"},"usage":{"input_tokens":3},"output":[]}`
	mapped := MapResponsesFailedBody(c, []byte(body))
	require.JSONEq(t, `{"id":"resp_1","status":"failed","error":{"message":"REPLACED","code":"x"},"usage":{"input_tokens":3},"output":[]}`, string(mapped))

	// The patch is targeted: unrelated raw values, including integers beyond
	// IEEE-754 precision, stay byte-identical.
	wide := `{"status":"failed","error":{"message":"reasoning_content"},"n":1234567890123456789,"nested":{"big":9007199254740993},"output":[]}`
	wideMapped := MapResponsesFailedBody(c, []byte(wide))
	require.Contains(t, string(wideMapped), `"message":"REPLACED"`)
	require.Contains(t, string(wideMapped), `"n":1234567890123456789`)
	require.Contains(t, string(wideMapped), `"big":9007199254740993`)

	completed := `{"status":"completed","error":{"message":"reasoning_content"}}`
	require.Equal(t, completed, string(MapResponsesFailedBody(c, []byte(completed))))

	outOfScope, _ := gin.CreateTestContext(httptest.NewRecorder())
	outOfScope.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	require.Equal(t, body, string(MapResponsesFailedBody(outOfScope, []byte(body))))
}

// Phase D regressions: only valid protocol error objects may be rewritten.
func TestErrorMessageMappingReviewFrameBoundaries(t *testing.T) {
	cases := []struct {
		name     string
		client   string
		payload  string
		terminal bool
	}{
		{"truncated chat JSON", "openai", `{"error":{"message":"reasoning_content"}`, false},
		{"trailing Claude JSON", "claude", `{"type":"error","error":{"message":"reasoning_content"}} garbage`, false},
		{"Responses non-string error leaf", "responses", `{"type":"error","error":{"message":null},"message":"reasoning_content"}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newErrorMessageMappingTestContext(t, tc.client, errorMessageMappingReasoningConfig)
			got, terminal := PrepareErrorMessageMappingSSEFrame(c, []byte(tc.payload))
			require.Equal(t, tc.payload, string(got), "out-of-contract payloads must remain byte-identical")
			require.Equal(t, tc.terminal, terminal)
		})
	}
}
