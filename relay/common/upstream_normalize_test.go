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

func relayInfoForResponsesBackfill(backfill bool) *RelayInfo {
	return &RelayInfo{
		ChannelMeta: &ChannelMeta{
			ChannelSetting: dto.ChannelSettings{ResponsesReasoningContentBackfill: backfill},
		},
	}
}

func TestNormalizeUpstreamRequestChatBackfill(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "fills missing reasoning_content on assistant tool call",
			input:  `{"model":"deepseek-v4.1-flash","messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]}]}`,
			expect: `{"model":"deepseek-v4.1-flash","messages":[{"role":"assistant","tool_calls":[{"id":"c1"}],"reasoning_content":""}]}`,
		},
		{
			name:   "preserves existing reasoning_content",
			input:  `{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}],"reasoning_content":"real"}]}`,
			expect: `{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}],"reasoning_content":"real"}]}`,
		},
		{
			name:   "keeps an explicit empty reasoning_content untouched",
			input:  `{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}],"reasoning_content":""}]}`,
			expect: `{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}],"reasoning_content":""}]}`,
		},
		{
			name:   "replaces null reasoning_content",
			input:  `{"messages":[{"role":"assistant","reasoning_content":null,"tool_calls":[{"id":"c1"}]}]}`,
			expect: `{"messages":[{"role":"assistant","reasoning_content":"","tool_calls":[{"id":"c1"}]}]}`,
		},
		{
			name:   "ignores assistant without tool_calls",
			input:  `{"messages":[{"role":"assistant","content":"hi"}]}`,
			expect: `{"messages":[{"role":"assistant","content":"hi"}]}`,
		},
		{
			name:   "ignores empty tool_calls array",
			input:  `{"messages":[{"role":"assistant","tool_calls":[]}]}`,
			expect: `{"messages":[{"role":"assistant","tool_calls":[]}]}`,
		},
		{
			name:   "ignores non-assistant tool_calls",
			input:  `{"messages":[{"role":"tool","tool_calls":[{"id":"c1"}]}]}`,
			expect: `{"messages":[{"role":"tool","tool_calls":[{"id":"c1"}]}]}`,
		},
		{
			name:   "backfills any model when enabled (no model gating)",
			input:  `{"model":"glm-5.3-flash","messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]}]}`,
			expect: `{"model":"glm-5.3-flash","messages":[{"role":"assistant","tool_calls":[{"id":"c1"}],"reasoning_content":""}]}`,
		},
		{
			name:   "touches only the matching message",
			input:  `{"messages":[{"role":"user","content":"go"},{"role":"assistant","tool_calls":[{"id":"c1"}]},{"role":"tool","content":"1"}]}`,
			expect: `{"messages":[{"role":"user","content":"go"},{"role":"assistant","tool_calls":[{"id":"c1"}],"reasoning_content":""},{"role":"tool","content":"1"}]}`,
		},
		{
			name:   "fills every qualifying assistant message",
			input:  `{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]},{"role":"tool","content":"1"},{"role":"assistant","tool_calls":[{"id":"c2"}]}]}`,
			expect: `{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}],"reasoning_content":""},{"role":"tool","content":"1"},{"role":"assistant","tool_calls":[{"id":"c2"}],"reasoning_content":""}]}`,
		},
		{
			name:   "leaves a responses input array untouched when only chat toggle is on",
			input:  `{"input":[{"type":"message","role":"assistant","content":"hi"}]}`,
			expect: `{"input":[{"type":"message","role":"assistant","content":"hi"}]}`,
		},
		{
			name:   "leaves body without messages untouched",
			input:  `{"model":"deepseek-v4.1-flash","prompt":"hi"}`,
			expect: `{"model":"deepseek-v4.1-flash","prompt":"hi"}`,
		},
		{
			name:   "leaves invalid json untouched",
			input:  `{"messages":[`,
			expect: `{"messages":[`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := relayInfoForBackfill("", "", true)
			got := NormalizeUpstreamRequest([]byte(tt.input), info)
			assert.Equal(t, tt.expect, string(got))
		})
	}
}

func TestNormalizeUpstreamRequestResponsesBackfill(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "fills missing reasoning_content on assistant input item",
			input:  `{"input":[{"type":"message","role":"assistant","content":"hi"}]}`,
			expect: `{"input":[{"type":"message","role":"assistant","content":"hi","reasoning_content":""}]}`,
		},
		{
			name:   "preserves existing reasoning_content on assistant item",
			input:  `{"input":[{"type":"message","role":"assistant","content":"hi","reasoning_content":"real"}]}`,
			expect: `{"input":[{"type":"message","role":"assistant","content":"hi","reasoning_content":"real"}]}`,
		},
		{
			name:   "replaces null reasoning_content on assistant item",
			input:  `{"input":[{"type":"message","role":"assistant","reasoning_content":null}]}`,
			expect: `{"input":[{"type":"message","role":"assistant","reasoning_content":""}]}`,
		},
		{
			name:   "ignores user input items",
			input:  `{"input":[{"type":"message","role":"user","content":"hi"}]}`,
			expect: `{"input":[{"type":"message","role":"user","content":"hi"}]}`,
		},
		{
			name:   "ignores function_call and function_call_output items",
			input:  `{"input":[{"type":"function_call","call_id":"c1","name":"calc","arguments":"{}"},{"type":"function_call_output","call_id":"c1","output":"4"}]}`,
			expect: `{"input":[{"type":"function_call","call_id":"c1","name":"calc","arguments":"{}"},{"type":"function_call_output","call_id":"c1","output":"4"}]}`,
		},
		{
			name:   "fills every qualifying assistant item",
			input:  `{"input":[{"type":"message","role":"user","content":"go"},{"type":"message","role":"assistant","content":"a"},{"type":"message","role":"assistant","content":"b"}]}`,
			expect: `{"input":[{"type":"message","role":"user","content":"go"},{"type":"message","role":"assistant","content":"a","reasoning_content":""},{"type":"message","role":"assistant","content":"b","reasoning_content":""}]}`,
		},
		{
			name:   "leaves a messages array untouched when only responses toggle is on",
			input:  `{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]}]}`,
			expect: `{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]}]}`,
		},
		{
			name:   "leaves invalid json untouched",
			input:  `{"input":[`,
			expect: `{"input":[`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := relayInfoForResponsesBackfill(true)
			got := NormalizeUpstreamRequest([]byte(tt.input), info)
			assert.Equal(t, tt.expect, string(got))
		})
	}
}

func TestNormalizeUpstreamRequestBackfillIsOptIn(t *testing.T) {
	chatInput := `{"model":"deepseek-v4.1-flash","messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]}]}`
	responsesInput := `{"input":[{"type":"message","role":"assistant","content":"hi"}]}`

	// Default (off): a channel that did not enable either switch stays unchanged.
	assert.Equal(t, chatInput, string(NormalizeUpstreamRequest([]byte(chatInput), relayInfoForBackfill("", "", false))))
	assert.Equal(t, responsesInput, string(NormalizeUpstreamRequest([]byte(responsesInput), relayInfoForResponsesBackfill(false))))

	// Existing relayInfoForUpstream builds a zero-value ChannelSettings, so it
	// must also leave the request unchanged (opt-in default off).
	assert.Equal(t, chatInput, string(NormalizeUpstreamRequest([]byte(chatInput), relayInfoForUpstream("deepseek-v4.1-flash", ""))))

	// Enabled: each switch applies to its own protocol shape.
	assert.Contains(t, string(NormalizeUpstreamRequest([]byte(chatInput), relayInfoForBackfill("", "", true))), `"reasoning_content":""`)
	assert.Contains(t, string(NormalizeUpstreamRequest([]byte(responsesInput), relayInfoForResponsesBackfill(true))), `"reasoning_content":""`)
}

func TestNormalizeUpstreamRequestChatAndResponsesTogglesAreIndependent(t *testing.T) {
	chatInput := `{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]}]}`
	responsesInput := `{"input":[{"type":"message","role":"assistant","content":"hi"}]}`

	// Chat on, responses off: only the messages array is backfilled.
	info := relayInfoForBackfill("", "", true)
	assert.Contains(t, string(NormalizeUpstreamRequest([]byte(chatInput), info)), `"reasoning_content":""`)
	assert.Equal(t, responsesInput, string(NormalizeUpstreamRequest([]byte(responsesInput), info)))

	// Chat off, responses on: only the input array is backfilled.
	responsesInfo := relayInfoForResponsesBackfill(true)
	assert.Equal(t, chatInput, string(NormalizeUpstreamRequest([]byte(chatInput), responsesInfo)))
	assert.Contains(t, string(NormalizeUpstreamRequest([]byte(responsesInput), responsesInfo)), `"reasoning_content":""`)
}

func TestNormalizeUpstreamRequestEmptyInputAndNilInfo(t *testing.T) {
	assert.Equal(t, "", string(NormalizeUpstreamRequest(nil, relayInfoForUpstream("deepseek-v4.1-flash", ""))))
	assert.Equal(t, "", string(NormalizeUpstreamRequest(nil, relayInfoForBackfill("deepseek-v4.1-flash", "", true))))
	assert.Equal(t, "", string(NormalizeUpstreamRequest(nil, relayInfoForResponsesBackfill(true))))
	assert.Equal(t, "", string(NormalizeUpstreamRequest(nil, nil)))

	input := []byte(`{"messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]}]}`)
	assert.Equal(t, string(input), string(NormalizeUpstreamRequest(input, nil)))
}

func TestNormalizeUpstreamRequestNilChannelMetaDoesNotBackfill(t *testing.T) {
	// A body with a non-nil info but nil ChannelMeta must fall through the nil
	// guard without panicking and stay unchanged (opt-in default off).
	input := []byte(`{"model":"deepseek-v4.1-flash","messages":[{"role":"assistant","tool_calls":[{"id":"c1"}]}]}`)
	info := &RelayInfo{OriginModelName: "deepseek-v4.1-flash"}
	assert.Equal(t, input, NormalizeUpstreamRequest(input, info))
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
