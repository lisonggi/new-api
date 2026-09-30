package common

import (
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// NormalizeUpstreamRequest applies upstream-provider compatibility fixes to an
// already-converted request body right before it is sent. It is the last
// request transformation, so a param override cannot reintroduce an
// incompatible shape that a handler-side fix would have missed.
//
// Each protocol shape is gated by its own opt-in channel switch:
//   - ReasoningContentBackfill targets a Chat Completions messages array.
//   - ResponsesReasoningContentBackfill targets a Responses input array.
//
// Thinking-mode upstreams reject a replayed assistant turn whose message or
// input item carries no reasoning_content. The fix fills a missing or null
// reasoning_content with an empty string; a value supplied by the converter or
// client is never overwritten. Bodies that match neither shape are returned
// unchanged.
func NormalizeUpstreamRequest(jsonData []byte, info *RelayInfo) []byte {
	// The backfills are opt-in per channel. A nil info or channel meta, or a
	// channel that enabled neither switch, is the common case (default off), so
	// return before parsing the body at all.
	if info == nil || info.ChannelMeta == nil {
		return jsonData
	}
	backfillChat := info.ChannelSetting.ReasoningContentBackfill
	backfillResponses := info.ChannelSetting.ResponsesReasoningContentBackfill
	if !backfillChat && !backfillResponses {
		return jsonData
	}
	if len(jsonData) == 0 || !gjson.ValidBytes(jsonData) {
		return jsonData
	}
	if backfillChat {
		jsonData = backfillChatToolCallReasoningContent(jsonData)
	}
	if backfillResponses {
		jsonData = backfillResponsesReasoningContent(jsonData)
	}
	return jsonData
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

// backfillResponsesReasoningContent fills an empty reasoning_content on
// assistant items in a Responses input array that replay a turn without one.
// It walks input[] structurally and rewrites only the missing key, so every
// other field and byte ordering is preserved. A body without an input array
// yields an unmodified body.
func backfillResponsesReasoningContent(jsonData []byte) []byte {
	input := gjson.GetBytes(jsonData, "input")
	if !input.IsArray() {
		return jsonData
	}
	result := jsonData
	input.ForEach(func(index, item gjson.Result) bool {
		if item.Get("role").String() != "assistant" {
			return true
		}
		reasoning := item.Get("reasoning_content")
		if reasoning.Exists() && reasoning.Type != gjson.Null {
			return true
		}
		if updated, err := sjson.SetBytes(result, "input."+index.String()+".reasoning_content", ""); err == nil {
			result = updated
		}
		return true
	})
	return result
}
