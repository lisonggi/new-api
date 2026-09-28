package common

import (
	"context"
	"testing"

	hostcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func relayInfoForUpstream(upstream, origin string) *RelayInfo {
	return &RelayInfo{
		ChannelMeta:     &ChannelMeta{UpstreamModelName: upstream},
		OriginModelName: origin,
	}
}

func relayInfoForBackfill(upstream, origin string, backfill bool) *RelayInfo {
	return &RelayInfo{
		ChannelMeta: &ChannelMeta{
			UpstreamModelName: upstream,
			ChannelSetting:    dto.ChannelSettings{ReasoningContentBackfill: backfill},
		},
		OriginModelName: origin,
	}
}

func TestNormalizeUpstreamRequestChatBackfill(t *testing.T) {
	tests := []struct {
		name   string
		model  string
		input  string
		expect string
	}{
		{
			name:   "fills missing reasoning_content on assistant tool call",
			model:  "deepseek-v4.1-flash",
			input:  `{"model":"deepseek-v4.1-flash","messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]}]}`,
			expect: `{"model":"deepseek-v4.1-flash","messages":[{"role":"assistant","tool_calls":[{"id":"c1"}],"reasoning_content":""}]}`,
		},
		{
			name:   "preserves existing reasoning_content",
			model:  "deepseek-v4.1-flash",
			input:  `{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}],"reasoning_content":"real"}]}`,
			expect: `{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}],"reasoning_content":"real"}]}`,
		},
		{
			name:   "keeps an explicit empty reasoning_content untouched",
			model:  "deepseek-v4.1-flash",
			input:  `{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}],"reasoning_content":""}]}`,
			expect: `{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}],"reasoning_content":""}]}`,
		},
		{
			name:   "replaces null reasoning_content",
			model:  "deepseek-v4-pro",
			input:  `{"messages":[{"role":"assistant","reasoning_content":null,"tool_calls":[{"id":"c1"}]}]}`,
			expect: `{"messages":[{"role":"assistant","reasoning_content":"","tool_calls":[{"id":"c1"}]}]}`,
		},
		{
			name:   "ignores assistant without tool_calls",
			model:  "deepseek-v4.1-flash",
			input:  `{"messages":[{"role":"assistant","content":"hi"}]}`,
			expect: `{"messages":[{"role":"assistant","content":"hi"}]}`,
		},
		{
			name:   "ignores empty tool_calls array",
			model:  "deepseek-v4.1-flash",
			input:  `{"messages":[{"role":"assistant","tool_calls":[]}]}`,
			expect: `{"messages":[{"role":"assistant","tool_calls":[]}]}`,
		},
		{
			name:   "ignores non-assistant tool_calls",
			model:  "deepseek-v4.1-flash",
			input:  `{"messages":[{"role":"tool","tool_calls":[{"id":"c1"}]}]}`,
			expect: `{"messages":[{"role":"tool","tool_calls":[{"id":"c1"}]}]}`,
		},
		{
			name:   "ignores other models",
			model:  "glm-5.3-flash",
			input:  `{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]}]}`,
			expect: `{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]}]}`,
		},
		{
			name:   "is case insensitive on the model name",
			model:  "DeepSeek-V4.1-Flash",
			input:  `{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]}]}`,
			expect: `{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}],"reasoning_content":""}]}`,
		},
		{
			name:   "touches only the matching message",
			model:  "deepseek-v4.1-flash",
			input:  `{"messages":[{"role":"user","content":"go"},{"role":"assistant","tool_calls":[{"id":"c1"}]},{"role":"tool","content":"1"}]}`,
			expect: `{"messages":[{"role":"user","content":"go"},{"role":"assistant","tool_calls":[{"id":"c1"}],"reasoning_content":""},{"role":"tool","content":"1"}]}`,
		},
		{
			name:   "fills every qualifying assistant message",
			model:  "deepseek-v4.1-flash",
			input:  `{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]},{"role":"tool","content":"1"},{"role":"assistant","tool_calls":[{"id":"c2"}]}]}`,
			expect: `{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}],"reasoning_content":""},{"role":"tool","content":"1"},{"role":"assistant","tool_calls":[{"id":"c2"}],"reasoning_content":""}]}`,
		},
		{
			name:   "leaves native responses input untouched",
			model:  "deepseek-v4.1-flash",
			input:  `{"model":"deepseek-v4.1-flash","input":[{"type":"function_call","call_id":"c1","name":"calc","arguments":"{}"}]}`,
			expect: `{"model":"deepseek-v4.1-flash","input":[{"type":"function_call","call_id":"c1","name":"calc","arguments":"{}"}]}`,
		},
		{
			name:   "leaves body without messages untouched",
			model:  "deepseek-v4.1-flash",
			input:  `{"model":"deepseek-v4.1-flash","prompt":"hi"}`,
			expect: `{"model":"deepseek-v4.1-flash","prompt":"hi"}`,
		},
		{
			name:   "leaves invalid json untouched",
			model:  "deepseek-v4.1-flash",
			input:  `{"messages":[`,
			expect: `{"messages":[`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := relayInfoForBackfill(tt.model, "", true)
			got := NormalizeUpstreamRequest([]byte(tt.input), info)
			assert.Equal(t, tt.expect, string(got))
		})
	}
}

func TestNormalizeUpstreamRequestBackfillIsOptIn(t *testing.T) {
	input := `{"model":"deepseek-v4.1-flash","messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]}]}`

	// Default (off): a channel that did not enable the backfill stays unchanged.
	assert.Equal(t, input, string(NormalizeUpstreamRequest([]byte(input), relayInfoForBackfill("deepseek-v4.1-flash", "", false))))

	// Existing relayInfoForUpstream builds a zero-value ChannelSettings, so it
	// must also leave the request unchanged (opt-in default off).
	assert.Equal(t, input, string(NormalizeUpstreamRequest([]byte(input), relayInfoForUpstream("deepseek-v4.1-flash", ""))))

	// Enabled: the backfill applies.
	assert.Contains(t, string(NormalizeUpstreamRequest([]byte(input), relayInfoForBackfill("deepseek-v4.1-flash", "", true))), `"reasoning_content":""`)

	// Non-DeepSeek stays unchanged even when the switch is on.
	other := `{"model":"glm-5.3-flash","messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]}]}`
	assert.Equal(t, other, string(NormalizeUpstreamRequest([]byte(other), relayInfoForBackfill("glm-5.3-flash", "", true))))
}

func TestNormalizeUpstreamRequestEmptyInputAndNilInfo(t *testing.T) {
	assert.Equal(t, "", string(NormalizeUpstreamRequest(nil, relayInfoForUpstream("deepseek-v4.1-flash", ""))))
	assert.Equal(t, "", string(NormalizeUpstreamRequest(nil, relayInfoForBackfill("deepseek-v4.1-flash", "", true))))
	assert.Equal(t, "", string(NormalizeUpstreamRequest(nil, nil)))

	input := []byte(`{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]}]}`)
	assert.Equal(t, string(input), string(NormalizeUpstreamRequest(input, nil)))
}

func TestNormalizeUpstreamRequestDoesNotInferUpstreamFromClientAlias(t *testing.T) {
	// The backfill is enabled so this test still guards the "never infer the
	// upstream model from the client alias" contract: even with the switch on,
	// an empty upstream model must not be treated as DeepSeek because the
	// client alias contains "deepseek".
	info := relayInfoForBackfill("", "deepseek-v4.1-flash", true)
	input := []byte(`{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]}]}`)
	assert.Equal(t, input, NormalizeUpstreamRequest(input, info))
}

func TestNormalizeUpstreamRequestNilChannelMetaDoesNotBackfill(t *testing.T) {
	// A DeepSeek body with a non-nil info but nil ChannelMeta must fall through
	// the nil guard without panicking and stay unchanged (opt-in default off).
	input := []byte(`{"model":"deepseek-v4.1-flash","messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]}]}`)
	info := &RelayInfo{OriginModelName: "deepseek-v4.1-flash"}
	assert.Equal(t, input, NormalizeUpstreamRequest(input, info))
}

func TestNormalizeUpstreamRequestUsesFinalModel(t *testing.T) {
	for _, tc := range []struct {
		name, upstream, final string
		wantFill              bool
	}{
		{"override away from DeepSeek", "deepseek-v4.1-flash", "other-model", false},
		{"override to DeepSeek", "other-model", "deepseek-v4.1-flash", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := []byte(`{"model":"` + tc.final + `","messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]}]}`)
			got := NormalizeUpstreamRequest(input, relayInfoForBackfill(tc.upstream, "", true))
			if tc.wantFill {
				assert.Contains(t, string(got), `"reasoning_content":""`)
			} else {
				assert.Equal(t, input, got)
			}
		})
	}
}

func TestNormalizeUpstreamRequestRejectsMalformedTrailingJSON(t *testing.T) {
	// Even with the backfill enabled, malformed trailing JSON must be returned
	// unchanged (never passed to the backfill that would fail on invalid input).
	input := []byte(`{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]}]} trailing`)
	assert.Equal(t, input, NormalizeUpstreamRequest(input, relayInfoForBackfill("deepseek-v4.1-flash", "", true)))
}

func TestNormalizeUpstreamRequestAfterClaudeToChatConversion(t *testing.T) {
	claudeRequest := dto.ClaudeRequest{
		Model:     "deepseek-v4.1-flash",
		MaxTokens: ptrUint(1024),
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "what is 2+2?"},
			{Role: "assistant", Content: []dto.ClaudeMediaMessage{
				{Type: "tool_use", Id: "toolu_1", Name: "calc", Input: map[string]any{"expr": "2+2"}},
			}},
			{Role: "user", Content: []dto.ClaudeMediaMessage{
				{Type: "tool_result", ToolUseId: "toolu_1", Content: "4"},
			}},
		},
	}

	result, err := relayconvert.ConvertRequest(context.Background(), &convmeta.Values{}, types.RelayFormatOpenAI, &claudeRequest)
	require.NoError(t, err)
	jsonData, err := hostcommon.Marshal(result.Value)
	require.NoError(t, err)

	normalized := NormalizeUpstreamRequest(jsonData, relayInfoForBackfill("deepseek-v4.1-flash", "deepseek-v4.1-flash", true))

	var chat dto.GeneralOpenAIRequest
	require.NoError(t, hostcommon.Unmarshal(normalized, &chat))
	assistant := findAssistantMessage(t, chat.Messages)
	require.NotEmpty(t, assistant.ParseToolCalls())
	require.NotNil(t, assistant.ReasoningContent)
	assert.Equal(t, "", *assistant.ReasoningContent)
}

func TestNormalizeUpstreamRequestAfterResponsesToChatConversion(t *testing.T) {
	responsesRequest := dto.OpenAIResponsesRequest{
		Model: "deepseek-v4.1-flash",
		Input: []byte(`[{"type":"function_call","call_id":"call_1","name":"calc","arguments":"{}"},{"type":"function_call_output","call_id":"call_1","output":"4"}]`),
	}

	result, err := relayconvert.ConvertRequest(context.Background(), &convmeta.Values{}, types.RelayFormatOpenAI, &responsesRequest)
	require.NoError(t, err)
	jsonData, err := hostcommon.Marshal(result.Value)
	require.NoError(t, err)

	normalized := NormalizeUpstreamRequest(jsonData, relayInfoForBackfill("deepseek-v4.1-flash", "deepseek-v4.1-flash", true))

	var chat dto.GeneralOpenAIRequest
	require.NoError(t, hostcommon.Unmarshal(normalized, &chat))
	assistant := findAssistantMessage(t, chat.Messages)
	require.NotEmpty(t, assistant.ParseToolCalls())
	require.NotNil(t, assistant.ReasoningContent)
	assert.Equal(t, "", *assistant.ReasoningContent)
}

func findAssistantMessage(t *testing.T, messages []dto.Message) *dto.Message {
	t.Helper()
	for i := range messages {
		if messages[i].Role == "assistant" {
			return &messages[i]
		}
	}
	t.Fatal("no assistant message found")
	return nil
}

func ptrUint(value uint) *uint { return &value }
