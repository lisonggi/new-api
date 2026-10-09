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
//   - AssistantContentBackfill targets a Chat Completions messages array.
//
// Thinking-mode upstreams reject a replayed assistant turn whose message or
// input item carries no reasoning_content. The fix fills a missing or null
// reasoning_content with an empty string; a value supplied by the converter or
// client is never overwritten. Separately, an upstream that requires an
// assistant turn to carry content or a tool call rejects a turn with neither;
// AssistantContentBackfill fills that missing or null content with an empty
// string. Bodies that match no enabled shape are returned unchanged.
func NormalizeUpstreamRequest(jsonData []byte, info *RelayInfo) []byte {
	// The backfills are opt-in per channel. A nil info or channel meta, or a
	// channel that enabled none of the switches, is the common case (default
	// off), so return before parsing the body at all.
	if info == nil || info.ChannelMeta == nil {
		return jsonData
	}
	backfillChat := info.ChannelSetting.ReasoningContentBackfill
	backfillResponses := info.ChannelSetting.ResponsesReasoningContentBackfill
	backfillContent := info.ChannelSetting.AssistantContentBackfill
	if !backfillChat && !backfillResponses && !backfillContent {
		return jsonData
	}
	if len(jsonData) == 0 || !gjson.ValidBytes(jsonData) {
		return jsonData
	}
	if backfillChat {
		// Chat Completions only needs it on assistant turns that replay tool calls.
		jsonData = backfillAssistantReasoningContent(jsonData, "messages", true)
	}
	if backfillResponses {
		jsonData = backfillAssistantReasoningContent(jsonData, "input", false)
	}
	if backfillContent {
		// The complementary set: assistant turns that replay no tool call.
		jsonData = backfillAssistantContent(jsonData)
	}
	return jsonData
}

// backfillAssistantContent fills a missing or null content on assistant turns in
// a Chat Completions messages array. An upstream may reject a replayed
// assistant turn that carries neither content nor a tool call ("Invalid
// assistant message: content or tool_calls must be set"); an empty string
// satisfies that check while preserving the turn. It walks the array
// structurally and rewrites only the missing key, so every other field and byte
// ordering is preserved.
//
// A turn that replays a tool call (a non-empty tool_calls array or the legacy
// function_call) already satisfies the upstream check and keeps its null
// content. A turn whose content is already present is left untouched,
// including an explicit empty string and a multimodal parts array. A body
// without a messages array yields an unmodified body.
func backfillAssistantContent(jsonData []byte) []byte {
	messages := gjson.GetBytes(jsonData, "messages")
	if !messages.IsArray() {
		return jsonData
	}
	result := jsonData
	messages.ForEach(func(index, turn gjson.Result) bool {
		if turn.Get("role").String() != "assistant" {
			return true
		}
		if toolCalls := turn.Get("tool_calls"); toolCalls.IsArray() && len(toolCalls.Array()) > 0 {
			return true
		}
		if functionCall := turn.Get("function_call"); functionCall.Exists() && functionCall.Type != gjson.Null {
			return true
		}
		content := turn.Get("content")
		if content.Exists() && content.Type != gjson.Null {
			return true
		}
		if updated, err := sjson.SetBytes(result, "messages."+index.String()+".content", ""); err == nil {
			result = updated
		}
		return true
	})
	return result
}

// backfillAssistantReasoningContent fills a missing or null reasoning_content on
// assistant turns in the array at arrayPath. It walks the array structurally and
// rewrites only the missing key, so every other field and byte ordering is
// preserved. A body without that array yields an unmodified body.
//
// Chat Completions (requireToolCalls) restricts the rewrite to assistant turns
// that replay tool calls, because only those are rejected without reasoning.
// Responses input items type the tool call instead of the role, so that protocol
// matches on item type as well (see isAssistantTurn).
func backfillAssistantReasoningContent(jsonData []byte, arrayPath string, requireToolCalls bool) []byte {
	array := gjson.GetBytes(jsonData, arrayPath)
	if !array.IsArray() {
		return jsonData
	}
	result := jsonData
	array.ForEach(func(index, turn gjson.Result) bool {
		if !isAssistantTurn(turn, requireToolCalls) {
			return true
		}
		reasoning := turn.Get("reasoning_content")
		if reasoning.Exists() && reasoning.Type != gjson.Null {
			return true
		}
		if updated, err := sjson.SetBytes(result, arrayPath+"."+index.String()+".reasoning_content", ""); err == nil {
			result = updated
		}
		return true
	})
	return result
}

// isAssistantTurn reports whether a replayed turn is assistant-authored and
// therefore needs reasoning_content present for a thinking-mode upstream.
//
// Chat Completions marks assistant turns with role "assistant" and only rejects
// the ones that replay tool calls.
//
// Responses input items are typed rather than role-only: an assistant turn is a
// message with role assistant, or one of the tool-call item types the model emits
// (function_call, custom_tool_call, local_shell_call), which carry no role at all.
// Matching on role alone skips exactly the tool-call turns these upstreams reject,
// while function_call_output items — the tool result, not an assistant turn — must
// stay untouched.
func isAssistantTurn(turn gjson.Result, requireToolCalls bool) bool {
	if turn.Get("role").String() == "assistant" {
		if !requireToolCalls {
			return true
		}
		toolCalls := turn.Get("tool_calls")
		return toolCalls.IsArray() && len(toolCalls.Array()) > 0
	}
	if requireToolCalls {
		// A Chat Completions assistant turn always carries a role, so a role-less
		// object in that array is not an assistant turn.
		return false
	}
	switch turn.Get("type").String() {
	case "function_call", "custom_tool_call", "local_shell_call":
		return true
	default:
		return false
	}
}
