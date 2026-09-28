package controller

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/error_mapping"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const errorMappingTestConfig = `{"enabled":true,"rules":[{"id":"reasoning-format","name":"format","enabled":true,"keyword":"reasoning_content","case_sensitive":false,"replacement":"当前请求格式与模型不兼容，请调整后重试。"}]}`

// setupErrorMessageMappingTest provisions an isolated SQLite database and
// installs an enabled rule set. It restores the disabled default before the
// database is torn down.
func setupErrorMessageMappingTest(t *testing.T) {
	t.Helper()
	require.NoError(t, i18n.Init())
	_ = modelManagementDB(t, "sqlite", "")
	cfg, err := error_mapping.ParseConfig([]byte(errorMappingTestConfig))
	require.NoError(t, err)
	_, err = model.SaveErrorMessageMapping(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = model.SaveErrorMessageMapping(error_mapping.DefaultConfig())
	})
}

func newErrorMessageMappingRelayContext(t *testing.T, path string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	service.BeginErrorMessageMapping(c)
	return c, recorder
}

func TestErrorMessageMappingConfigStrictnessAndMatching(t *testing.T) {
	invalid := []string{
		``,
		`true`,
		`{"enabled":true}`,
		`{"rules":[]}`,
		`{"enabled":true,"rules":null}`,
		`{"enabled":null,"rules":[]}`,
		`{"enabled":true,"rules":{}}`,
		`{"enabled":true,"rules":[{"id":"a","name":"","enabled":true,"keyword":"k","case_sensitive":false,"replacement":"r","extra":1}]}`,
		`{"enabled":true,"rules":[{"id":"a","enabled":true,"keyword":"k","case_sensitive":false,"replacement":"r"},{"id":"a","enabled":true,"keyword":"k2","case_sensitive":false,"replacement":"r2"}]}`,
		`{"enabled":true,"rules":[{"id":"a","enabled":true,"keyword":"   ","case_sensitive":false,"replacement":"r"}]}`,
		`{"enabled":true,"rules":[{"id":"a","enabled":true,"keyword":"k","case_sensitive":false,"replacement":"   "}]}`,
		`{"enabled":true,"rules":[{"id":"bad id","enabled":true,"keyword":"k","case_sensitive":false,"replacement":"r"}]}`,
		`{"enabled":true,"rules":[]} trailing`,
	}
	for _, raw := range invalid {
		_, err := error_mapping.ParseConfig([]byte(raw))
		require.Error(t, err, "expected reject: %s", raw)
	}

	longKeyword := strings.Repeat("a", error_mapping.MaxKeywordLength+1)
	_, err := error_mapping.ParseConfig([]byte(`{"enabled":true,"rules":[{"id":"a","enabled":true,"keyword":"` + longKeyword + `","case_sensitive":false,"replacement":"r"}]}`))
	require.Error(t, err)
	longReplacement := strings.Repeat("b", error_mapping.MaxReplacementLength+1)
	_, err = error_mapping.ParseConfig([]byte(`{"enabled":true,"rules":[{"id":"a","enabled":true,"keyword":"k","case_sensitive":false,"replacement":"` + longReplacement + `"}]}`))
	require.Error(t, err)

	cfg, err := error_mapping.ParseConfig([]byte(`{"enabled":true,"rules":[` +
		`{"id":"first","name":"First","enabled":true,"keyword":"ALPHA","case_sensitive":false,"replacement":"替换甲"},` +
		`{"id":"third","name":"Third","enabled":true,"keyword":"中文关键词","case_sensitive":false,"replacement":"中文替换"}]}`))
	require.NoError(t, err)
	matcher, err := error_mapping.Compile(cfg)
	require.NoError(t, err)

	require.Equal(t, error_mapping.Result{Message: "替换甲", Matched: true, RuleID: "first"}, matcher.Match("see ALPHA here"))
	require.Equal(t, error_mapping.Result{Message: "替换甲", Matched: true, RuleID: "first"}, matcher.Match("see alpha here"))
	require.Equal(t, error_mapping.Result{Message: "中文替换", Matched: true, RuleID: "third"}, matcher.Match("出现中文关键词了"))
	require.False(t, matcher.Match("no keyword here").Matched)
	require.Equal(t, "no keyword here", matcher.Match("no keyword here").Message)

	// Case-sensitive rules compare the raw bytes.
	caseSensitive, err := error_mapping.Compile(error_mapping.Config{Enabled: true, Rules: []error_mapping.Rule{
		{ID: "cs", Enabled: true, Keyword: "Alpha", CaseSensitive: true, Replacement: "CS"},
	}})
	require.NoError(t, err)
	require.False(t, caseSensitive.Match("alpha").Matched)
	require.True(t, caseSensitive.Match("Alpha").Matched)

	// The global switch and per-rule switch both stop matching.
	disabled, err := error_mapping.Compile(error_mapping.Config{Enabled: false, Rules: cfg.Rules})
	require.NoError(t, err)
	require.False(t, disabled.Match("see ALPHA here").Matched)
	ruleOff, err := error_mapping.Compile(error_mapping.Config{Enabled: true, Rules: []error_mapping.Rule{
		{ID: "off", Enabled: false, Keyword: "alpha", Replacement: "X"},
	}})
	require.NoError(t, err)
	require.False(t, ruleOff.Match("alpha").Matched)

	// The first match wins and the replacement is not matched again.
	chain, err := error_mapping.Compile(error_mapping.Config{Enabled: true, Rules: []error_mapping.Rule{
		{ID: "a", Enabled: true, Keyword: "alpha", Replacement: "beta"},
		{ID: "b", Enabled: true, Keyword: "beta", Replacement: "gamma"},
	}})
	require.NoError(t, err)
	require.Equal(t, "beta", chain.Match("alpha").Message)
}

func TestErrorMessageMappingSettingsEndpoints(t *testing.T) {
	setupErrorMessageMappingTest(t)

	getRecorder := httptest.NewRecorder()
	getCtx, _ := gin.CreateTestContext(getRecorder)
	GetErrorMessageMapping(getCtx)
	require.Equal(t, http.StatusOK, getRecorder.Code)
	var getBody struct {
		Success bool                 `json:"success"`
		Data    error_mapping.Config `json:"data"`
	}
	require.NoError(t, common.Unmarshal(getRecorder.Body.Bytes(), &getBody))
	require.True(t, getBody.Success)
	require.True(t, getBody.Data.Enabled)
	require.Len(t, getBody.Data.Rules, 1)

	putInvalidRecorder := httptest.NewRecorder()
	putInvalidCtx, _ := gin.CreateTestContext(putInvalidRecorder)
	putInvalidCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option/error_message_mapping", strings.NewReader(`{"enabled":true,"rules":[],"extra":1}`))
	UpdateErrorMessageMapping(putInvalidCtx)
	require.Equal(t, http.StatusBadRequest, putInvalidRecorder.Code)

	putRecorder := httptest.NewRecorder()
	putCtx, _ := gin.CreateTestContext(putRecorder)
	putCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option/error_message_mapping", strings.NewReader(`{"enabled":false,"rules":[]}`))
	UpdateErrorMessageMapping(putCtx)
	require.Equal(t, http.StatusOK, putRecorder.Code)
	require.JSONEq(t, `{"success":true,"message":"","data":{"enabled":false,"rules":[]}}`, putRecorder.Body.String(), "saving an empty ruleset must return an array, not null")
	require.False(t, model.CurrentErrorMessageMapping().Config.Enabled)

	oversizedRecorder := httptest.NewRecorder()
	oversizedCtx, _ := gin.CreateTestContext(oversizedRecorder)
	oversizedCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option/error_message_mapping", strings.NewReader(`{"enabled":true,"rules":[],"pad":"`+strings.Repeat("a", error_mapping.MaxConfigBodyBytes)+`"}`))
	UpdateErrorMessageMapping(oversizedCtx)
	require.Equal(t, http.StatusRequestEntityTooLarge, oversizedRecorder.Code)
}

func TestErrorMessageMappingPreviewEndpoint(t *testing.T) {
	require.NoError(t, i18n.Init())
	matchedRecorder := httptest.NewRecorder()
	matchedCtx, _ := gin.CreateTestContext(matchedRecorder)
	matchedCtx.Request = httptest.NewRequest(http.MethodPost, "/api/option/error_message_mapping/preview", strings.NewReader(`{"config":`+errorMappingTestConfig+`,"message":"saw reasoning_content here"}`))
	PreviewErrorMessageMapping(matchedCtx)
	require.Equal(t, http.StatusOK, matchedRecorder.Code)
	var matchedBody struct {
		Success bool `json:"success"`
		Data    struct {
			Matched bool   `json:"matched"`
			RuleID  string `json:"rule_id"`
			Message string `json:"message"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(matchedRecorder.Body.Bytes(), &matchedBody))
	require.True(t, matchedBody.Success)
	require.True(t, matchedBody.Data.Matched)
	require.Equal(t, "reasoning-format", matchedBody.Data.RuleID)
	require.Equal(t, "当前请求格式与模型不兼容，请调整后重试。", matchedBody.Data.Message)

	disabledRecorder := httptest.NewRecorder()
	disabledCtx, _ := gin.CreateTestContext(disabledRecorder)
	disabledCtx.Request = httptest.NewRequest(http.MethodPost, "/api/option/error_message_mapping/preview", strings.NewReader(`{"config":{"enabled":false,"rules":[{"id":"r1","enabled":true,"keyword":"reasoning_content","case_sensitive":false,"replacement":"X"}]},"message":"reasoning_content"}`))
	PreviewErrorMessageMapping(disabledCtx)
	require.Equal(t, http.StatusOK, disabledRecorder.Code)
	var disabledBody struct {
		Data struct {
			Matched bool   `json:"matched"`
			Message string `json:"message"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(disabledRecorder.Body.Bytes(), &disabledBody))
	require.False(t, disabledBody.Data.Matched)
	require.Equal(t, "reasoning_content", disabledBody.Data.Message)

	missingRecorder := httptest.NewRecorder()
	missingCtx, _ := gin.CreateTestContext(missingRecorder)
	missingCtx.Request = httptest.NewRequest(http.MethodPost, "/api/option/error_message_mapping/preview", strings.NewReader(`{"config":`+errorMappingTestConfig+`}`))
	PreviewErrorMessageMapping(missingCtx)
	require.Equal(t, http.StatusBadRequest, missingRecorder.Code)

	emptyRecorder := httptest.NewRecorder()
	emptyCtx, _ := gin.CreateTestContext(emptyRecorder)
	emptyCtx.Request = httptest.NewRequest(http.MethodPost, "/api/option/error_message_mapping/preview", strings.NewReader(`{"config":`+errorMappingTestConfig+`,"message":""}`))
	PreviewErrorMessageMapping(emptyCtx)
	require.Equal(t, http.StatusOK, emptyRecorder.Code)

	invalidRecorder := httptest.NewRecorder()
	invalidCtx, _ := gin.CreateTestContext(invalidRecorder)
	invalidCtx.Request = httptest.NewRequest(http.MethodPost, "/api/option/error_message_mapping/preview", strings.NewReader(`{"config":{"enabled":true},"message":"x"}`))
	PreviewErrorMessageMapping(invalidCtx)
	require.Equal(t, http.StatusBadRequest, invalidRecorder.Code)
}

func TestWriteErrorMessageMappedRelayErrorHTTP(t *testing.T) {
	setupErrorMessageMappingTest(t)

	c, recorder := newErrorMessageMappingRelayContext(t, "/v1/chat/completions")
	c.Set(common.RequestIdKey, "req-abc")
	newAPIError := types.NewOpenAIError(errors.New("reasoning_content not supported"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)
	writeErrorMessageMappedRelayError(c, nil, newAPIError, types.RelayFormatOpenAI, "req-abc")

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"message":"当前请求格式与模型不兼容，请调整后重试。 (request id: req-abc)"`)
	require.Equal(t, "reasoning_content not supported", newAPIError.ToOpenAIError().Message, "the original error keeps its message")

	missCtx, missRecorder := newErrorMessageMappingRelayContext(t, "/v1/chat/completions")
	missError := types.NewOpenAIError(errors.New("unrelated upstream failure"), types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError)
	writeErrorMessageMappedRelayError(missCtx, nil, missError, types.RelayFormatOpenAI, "")
	require.Contains(t, missRecorder.Body.String(), `"message":"unrelated upstream failure"`)
	require.NotContains(t, missRecorder.Body.String(), "request id")
}

func TestWriteErrorMessageMappedRelayErrorCrossFormat(t *testing.T) {
	setupErrorMessageMappingTest(t)

	c, recorder := newErrorMessageMappingRelayContext(t, "/v1/messages")
	c.Set(common.RequestIdKey, "req-claude")
	newAPIError := types.NewOpenAIError(errors.New("reasoning_content not supported"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)
	writeErrorMessageMappedRelayError(c, nil, newAPIError, types.RelayFormatClaude, "req-claude")

	require.Contains(t, recorder.Body.String(), `"type":"error"`)
	require.Contains(t, recorder.Body.String(), `"message":"当前请求格式与模型不兼容，请调整后重试。 (request id: req-claude)"`)
}

func TestWriteErrorMessageMappedRelayErrorCommittedSSE(t *testing.T) {
	setupErrorMessageMappingTest(t)

	chatCtx, chatRecorder := newErrorMessageMappingRelayContext(t, "/v1/chat/completions")
	chatCtx.Writer.Header().Set("Content-Type", "text/event-stream")
	chatCtx.Writer.WriteHeaderNow()
	chatError := types.NewOpenAIError(errors.New("reasoning_content not supported"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)
	writeErrorMessageMappedRelayError(chatCtx, nil, chatError, types.RelayFormatOpenAI, "req-1")
	chatBody := chatRecorder.Body.String()
	require.True(t, strings.HasPrefix(chatBody, "data: "), "a committed SSE stream must not receive a plain JSON body")
	require.Contains(t, chatBody, "当前请求格式与模型不兼容，请调整后重试。")

	claudeCtx, claudeRecorder := newErrorMessageMappingRelayContext(t, "/v1/messages")
	claudeCtx.Writer.Header().Set("Content-Type", "text/event-stream")
	claudeCtx.Writer.WriteHeaderNow()
	writeErrorMessageMappedRelayError(claudeCtx, nil, types.NewOpenAIError(errors.New("reasoning_content"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest), types.RelayFormatClaude, "req-2")
	claudeBody := claudeRecorder.Body.String()
	require.Contains(t, claudeBody, "event: error\n")
	require.Contains(t, claudeBody, `"type":"error"`)
	require.Contains(t, claudeBody, "当前请求格式与模型不兼容，请调整后重试。")

	responsesCtx, responsesRecorder := newErrorMessageMappingRelayContext(t, "/v1/responses")
	responsesCtx.Writer.Header().Set("Content-Type", "text/event-stream")
	responsesCtx.Writer.WriteHeaderNow()
	writeErrorMessageMappedRelayError(responsesCtx, nil, types.NewOpenAIError(errors.New("reasoning_content"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest), types.RelayFormatOpenAIResponses, "req-3")
	responsesBody := responsesRecorder.Body.String()
	require.Contains(t, responsesBody, "event: error\n")
	require.Contains(t, responsesBody, `"type":"error"`)
	require.Contains(t, responsesBody, "当前请求格式与模型不兼容，请调整后重试。")
}

func TestWriteErrorMessageMappedRelayErrorTerminalAlreadyEmitted(t *testing.T) {
	setupErrorMessageMappingTest(t)

	c, recorder := newErrorMessageMappingRelayContext(t, "/v1/chat/completions")
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.WriteHeaderNow()
	service.MarkErrorMessageMappingTerminal(c)
	writeErrorMessageMappedRelayError(c, nil, types.NewOpenAIError(errors.New("reasoning_content"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest), types.RelayFormatOpenAI, "req-1")
	require.Empty(t, recorder.Body.String(), "no trailing error after a terminal event")
}

func TestWriteErrorMessageMappedRelayErrorOutOfScope(t *testing.T) {
	setupErrorMessageMappingTest(t)

	c, recorder := newErrorMessageMappingRelayContext(t, "/v1/embeddings")
	newAPIError := types.NewOpenAIError(errors.New("reasoning_content not supported"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)
	writeErrorMessageMappedRelayError(c, nil, newAPIError, types.RelayFormatOpenAI, "req-1")
	require.Contains(t, recorder.Body.String(), `"message":"reasoning_content not supported"`)
	require.NotContains(t, recorder.Body.String(), "当前请求格式与模型不兼容")
}

func TestHelperSSEWritesUseErrorMessageMapping(t *testing.T) {
	setupErrorMessageMappingTest(t)

	claudeCtx, claudeRecorder := newErrorMessageMappingRelayContext(t, "/v1/messages")
	require.NoError(t, helper.ClaudeData(claudeCtx, dto.ClaudeResponse{
		Type:  "error",
		Error: map[string]any{"type": "invalid_request_error", "message": "bad reasoning_content"},
	}))
	require.Contains(t, claudeRecorder.Body.String(), `"message":"当前请求格式与模型不兼容，请调整后重试。"`)

	chatCtx, chatRecorder := newErrorMessageMappingRelayContext(t, "/v1/chat/completions")
	require.NoError(t, helper.StringData(chatCtx, `{"error":{"message":"bad reasoning_content"}}`))
	require.Contains(t, chatRecorder.Body.String(), `"message":"当前请求格式与模型不兼容，请调整后重试。"`)
	require.True(t, service.ErrorMessageMappingTerminalEmitted(chatCtx), "an error frame is terminal")

	chatDoneCtx, _ := newErrorMessageMappingRelayContext(t, "/v1/chat/completions")
	helper.Done(chatDoneCtx)
	require.True(t, service.ErrorMessageMappingTerminalEmitted(chatDoneCtx))

	responsesCtx, responsesRecorder := newErrorMessageMappingRelayContext(t, "/v1/responses")
	require.NoError(t, helper.ResponseChunkData(responsesCtx, dto.ResponsesStreamResponse{Type: "response.failed"}, `{"type":"response.failed","response":{"error":{"message":"bad reasoning_content"}}}`))
	require.Contains(t, responsesRecorder.Body.String(), `"message":"当前请求格式与模型不兼容，请调整后重试。"`)
}

// A frame that never reaches the client must not be remembered as a terminal
// event, otherwise a later real error is silently suppressed.
func TestErrorMessageMappingTerminalRequiresSuccessfulWrite(t *testing.T) {
	setupErrorMessageMappingTest(t)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	service.BeginErrorMessageMapping(c)
	ctx, cancel := context.WithCancel(c.Request.Context())
	cancel()
	c.Request = c.Request.WithContext(ctx)

	err := helper.StringData(c, `{"error":{"message":"bad reasoning_content"}}`)
	require.Error(t, err)
	require.False(t, service.ErrorMessageMappingTerminalEmitted(c), "a failed write is not terminal")
}

// Flush is deliberately successful and the request remains live. This models
// a transport Write failure that CustomEvent must not hide from terminal state.
type errorMappingFailedWriter struct {
	*httptest.ResponseRecorder
}

func (w *errorMappingFailedWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("transport write failed")
}

// The helper write boundary is not recursive and preserves unrelated numeric
// bytes: the replacement is never matched again and the targeted patch leaves
// exact integers intact.
func TestHelperSSEMappingIsNotRecursiveAndPreservesNumbers(t *testing.T) {
	setupErrorMessageMappingTest(t)
	cfg, err := error_mapping.ParseConfig([]byte(`{"enabled":true,"rules":[{"id":"a","enabled":true,"keyword":"alpha","case_sensitive":false,"replacement":"beta"},{"id":"b","enabled":true,"keyword":"beta","case_sensitive":false,"replacement":"gamma"}]}`))
	require.NoError(t, err)
	_, err = model.SaveErrorMessageMapping(cfg)
	require.NoError(t, err)

	c, recorder := newErrorMessageMappingRelayContext(t, "/v1/chat/completions")
	require.NoError(t, helper.StringData(c, `{"error":{"message":"alpha"},"n":1234567890123456789,"f":0.1234567890123456789}`))
	body := recorder.Body.String()
	assert.Contains(t, body, `"message":"beta"`)
	assert.NotContains(t, body, `"message":"gamma"`)
	assert.Contains(t, body, `"n":1234567890123456789`)
	assert.Contains(t, body, `"f":0.1234567890123456789`)
}

// Validation failures are translated from the stable error code, so the same
// body returns a localized message for every supported language.
func TestErrorMessageMappingValidationMessagesLocalized(t *testing.T) {
	require.NoError(t, i18n.Init())
	cases := []struct {
		name     string
		language string
		body     string
		want     string
	}{
		{"english", "en", `{"enabled":true,"rules":null}`, "rules must be an array"},
		{"simplified", "zh-CN", `{"enabled":true,"rules":null}`, "rules 必须为数组"},
		{"traditional", "zh-TW", `{"enabled":true,"rules":null}`, "rules 必須為陣列"},
		{"template data", "zh-CN", `{"enabled":true,"rules":[{"id":"bad id","enabled":true,"keyword":"k","case_sensitive":false,"replacement":"r"}]}`, "第 0 条规则"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPut, "/api/option/error_message_mapping", strings.NewReader(tc.body))
			c.Request.Header.Set("Accept-Language", tc.language)
			UpdateErrorMessageMapping(c)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Contains(t, recorder.Body.String(), tc.want)
		})
	}
}

func TestErrorMessageMappingTransportWriteFailure(t *testing.T) {
	cases := []struct {
		name  string
		path  string
		write func(*gin.Context)
	}{
		{"chat", "/v1/chat/completions", func(c *gin.Context) {
			_ = helper.StringData(c, `{"error":{"message":"upstream failure"}}`)
		}},
		{"claude", "/v1/messages", func(c *gin.Context) {
			_ = helper.ClaudeData(c, dto.ClaudeResponse{Type: "error", Error: map[string]any{"message": "upstream failure"}})
		}},
		{"claude chunk", "/v1/messages", func(c *gin.Context) {
			helper.ClaudeChunkData(c, dto.ClaudeResponse{Type: "error"}, `{"type":"error","error":{"message":"upstream failure"}}`)
		}},
		{"responses", "/v1/responses", func(c *gin.Context) {
			_ = helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: "error"}, `{"type":"error","message":"upstream failure"}`)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writer := &errorMappingFailedWriter{ResponseRecorder: httptest.NewRecorder()}
			c, _ := gin.CreateTestContext(writer)
			c.Request = httptest.NewRequest(http.MethodPost, tc.path, nil)
			service.BeginErrorMessageMapping(c)
			tc.write(c)
			require.NoError(t, c.Request.Context().Err())
			require.Empty(t, writer.Body.String())
			assert.False(t, service.ErrorMessageMappingTerminalEmitted(c), "failed transport writes must not count as emitted terminal events")
		})
	}
}

type errorMappingShortWriter struct {
	*httptest.ResponseRecorder
}

func (w *errorMappingShortWriter) Write(p []byte) (int, error) {
	return w.ResponseRecorder.Write(p[:len(p)/2])
}

func TestErrorMessageMappingShortWriteIsNotTerminal(t *testing.T) {
	writer := &errorMappingShortWriter{ResponseRecorder: httptest.NewRecorder()}
	c, _ := gin.CreateTestContext(writer)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	service.BeginErrorMessageMapping(c)
	err := helper.StringData(c, `{"error":{"message":"failed"}}`)
	require.Error(t, err)
	assert.False(t, service.ErrorMessageMappingTerminalEmitted(c))
}

func TestErrorMessageMappingRequestsKeepTheirSnapshot(t *testing.T) {
	setupErrorMessageMappingTest(t)
	oldRequest, oldOutput := newErrorMessageMappingRelayContext(t, "/v1/chat/completions")
	cfg := model.CurrentErrorMessageMapping().Config
	cfg.Rules[0].Replacement = "NEW RULE"
	_, err := model.SaveErrorMessageMapping(cfg)
	require.NoError(t, err)
	newRequest, newOutput := newErrorMessageMappingRelayContext(t, "/v1/chat/completions")
	require.NoError(t, helper.StringData(oldRequest, `{"error":{"message":"reasoning_content"}}`))
	require.NoError(t, helper.StringData(newRequest, `{"error":{"message":"reasoning_content"}}`))
	assert.Contains(t, oldOutput.Body.String(), `"message":"当前请求格式与模型不兼容，请调整后重试。"`)
	assert.Contains(t, newOutput.Body.String(), `"message":"NEW RULE"`)
}

func TestErrorMessageMappingNativeResponsesHandler(t *testing.T) {
	setupErrorMessageMappingTest(t)
	const body = `{"id":"resp_failed","status":"failed","error":{"message":"reasoning_content","code":"invalid"},"usage":{"input_tokens":3,"output_tokens":0,"total_tokens":3},"output":[],"extra":9007199254740993}`
	c, recorder := newErrorMessageMappingRelayContext(t, "/v1/responses")
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	_, apiErr := openai.OaiResponsesHandler(c, &relaycommon.RelayInfo{OriginModelName: "test-model"}, response)
	require.Nil(t, apiErr)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, strings.Replace(body, "reasoning_content", "当前请求格式与模型不兼容，请调整后重试。", 1), recorder.Body.String())
}
