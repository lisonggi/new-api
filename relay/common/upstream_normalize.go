package common

import (
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// NormalizeUpstreamRequest applies upstream-provider compatibility fixes to an
// already-converted request body right before it is sent. It is the last
// request transformation, so a param override cannot reintroduce an
// incompatible shape that a handler-side fix would have missed.
//
// Only DeepSeek thinking-mode upstreams are targeted: they reject a replayed
// assistant tool call whose message carries no reasoning_content. The fix fills
// a missing or null reasoning_content with an empty string; a value supplied by
// the converter or client is never overwritten. Requests for any other upstream
// model, and bodies without a Chat Completions messages array (for example a
// native Responses input array), are returned unchanged.
func NormalizeUpstreamRequest(jsonData []byte, info *RelayInfo) []byte {
	// The backfill is opt-in per channel. A nil info or channel meta, or a
	// channel that did not enable it, is the common case (default off), so
	// return before parsing the body at all.
	if info == nil || info.ChannelMeta == nil || !info.ChannelSetting.ReasoningContentBackfill {
		return jsonData
	}
	if len(jsonData) == 0 || !gjson.ValidBytes(jsonData) {
		return jsonData
	}
	// Parameter overrides may change the model without updating RelayInfo.
	// Prefer the model actually sent upstream; never infer it from a client alias.
	modelName := ""
	model := gjson.GetBytes(jsonData, "model")
	if model.Exists() {
		if model.Type != gjson.String {
			return jsonData
		}
		modelName = model.String()
	} else {
		modelName = info.GetUpstreamModelName()
	}
	if !strings.Contains(strings.ToLower(modelName), "deepseek") {
		return jsonData
	}
	return backfillChatToolCallReasoningContent(jsonData)
}

// backfillChatToolCallReasoningContent fills an empty reasoning_content on
// assistant messages that replay tool calls without one. It walks the Chat
// Completions messages array structurally and rewrites only the missing key, so
// every other field and byte ordering is preserved. Invalid or non-object JSON
// yields an unmodified body.
func backfillChatToolCallReasoningContent(jsonData []byte) []byte {
	messages := gjson.GetBytes(jsonData, "messages")
	if !messages.IsArray() {
		return jsonData
	}
	result := jsonData
	messages.ForEach(func(index, message gjson.Result) bool {
		if message.Get("role").String() != "assistant" {
			return true
		}
		toolCalls := message.Get("tool_calls")
		if !toolCalls.IsArray() || len(toolCalls.Array()) == 0 {
			return true
		}
		reasoning := message.Get("reasoning_content")
		if reasoning.Exists() && reasoning.Type != gjson.Null {
			return true
		}
		if updated, err := sjson.SetBytes(result, "messages."+index.String()+".reasoning_content", ""); err == nil {
			result = updated
		}
		return true
	})
	return result
}
