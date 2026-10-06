package service

import (
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// MaxChannelErrorRetryFieldBytes bounds each textual field projected into the
// matcher view. The original error and the client output are never truncated.
const MaxChannelErrorRetryFieldBytes = 16 * 1024

const channelErrorRetryHTTPContextKey = "channel_error_retry_http"

// BeginChannelErrorRetryHTTP marks a request that may evaluate a channel error
// retry policy: POST on one of the four supported HTTP entries and not a
// WebSocket upgrade. It is independent of the message mapping switch and is not
// scheduling state. Realtime is excluded by the path check.
func BeginChannelErrorRetryHTTP(c *gin.Context) {
	if c == nil || c.Request == nil {
		return
	}
	if c.Request.Method != http.MethodPost {
		return
	}
	if _, ok := errorMessageMappingPathClient(c.Request.URL.Path); !ok {
		return
	}
	if strings.EqualFold(c.Request.Header.Get("Upgrade"), "websocket") {
		return
	}
	c.Set(channelErrorRetryHTTPContextKey, true)
}

// ChannelErrorRetryHTTPActive reports whether the entry scope marker is set.
func ChannelErrorRetryHTTPActive(c *gin.Context) bool {
	if c == nil {
		return false
	}
	return c.GetBool(channelErrorRetryHTTPContextKey)
}

// ChannelErrorRetryView is the read-only projection of the existing error object
// used for matching. It reuses the existing normalization and masking; it never
// re-parses the upstream body.
type ChannelErrorRetryView struct {
	StatusCode       int
	Message          string
	MessageTruncated bool
	Code             string
	CodeTruncated    bool
	Type             string
	TypeTruncated    bool
}

func buildChannelErrorRetryView(err *types.NewAPIError) ChannelErrorRetryView {
	message, messageTruncated := truncateChannelErrorRetryField(err.MaskSensitiveError())
	code, codeTruncated := truncateChannelErrorRetryField(string(err.GetErrorCode()))
	errorType, typeTruncated := truncateChannelErrorRetryField(channelErrorRetryErrorType(err))
	return ChannelErrorRetryView{
		StatusCode:       err.StatusCode,
		Message:          message,
		MessageTruncated: messageTruncated,
		Code:             code,
		CodeTruncated:    codeTruncated,
		Type:             errorType,
		TypeTruncated:    typeTruncated,
	}
}

// channelErrorRetryErrorType returns the existing standardized protocol error
// type the administrator configures against, independent of the client entry
// format so the same upstream error matches the same way everywhere. It is
// deliberately not GetErrorType (the wrapper classification). ToOpenAIError is
// the single normalization view: ToClaudeError would reduce an OpenAI-typed
// error to its raw Code (often "<nil>").
func channelErrorRetryErrorType(err *types.NewAPIError) string {
	return err.ToOpenAIError().Type
}

func truncateChannelErrorRetryField(value string) (string, bool) {
	if len(value) <= MaxChannelErrorRetryFieldBytes {
		return value, false
	}
	prefix := value[:MaxChannelErrorRetryFieldBytes]
	for len(prefix) > 0 && !utf8.ValidString(prefix) {
		prefix = prefix[:len(prefix)-1]
	}
	return prefix, true
}

// ChannelErrorRetryMatchStatus is the evidence state of one policy evaluation.
type ChannelErrorRetryMatchStatus string

const (
	ChannelErrorRetryMatched         ChannelErrorRetryMatchStatus = "matched"
	ChannelErrorRetryMiss            ChannelErrorRetryMatchStatus = "miss"
	ChannelErrorRetryInputIncomplete ChannelErrorRetryMatchStatus = "input_incomplete"
)

// ChannelErrorRetryMatchResult is the matcher output. RuleID/Action are set only
// for a matched rule and hold no shared pointers.
type ChannelErrorRetryMatchResult struct {
	Status ChannelErrorRetryMatchStatus
	RuleID string
	Action string
}

// MatchChannelErrorRetry evaluates the ordered rules with first-match-wins. A
// truncated field can still decide contains/not_contains when its prefix already
// proves the outcome, and always decides equals, because a validated target is
// far shorter than the projection cap and therefore can never equal the full
// field; otherwise the rule is uncertain and the whole policy inherits instead
// of skipping to a later rule. It is pure and never mutates the policy or the
// view.
func MatchChannelErrorRetry(policy *kitdto.ChannelErrorRetryPolicy, view ChannelErrorRetryView) ChannelErrorRetryMatchResult {
	if policy == nil || !policy.Enabled {
		return ChannelErrorRetryMatchResult{Status: ChannelErrorRetryMiss}
	}
	for _, rule := range policy.Rules {
		if !rule.Enabled {
			continue
		}
		if len(rule.StatusCodes) > 0 && !slices.Contains(rule.StatusCodes, view.StatusCode) {
			continue
		}
		ruleFalse := false
		incomplete := false
		for _, condition := range rule.Conditions {
			fieldValue, truncated := channelErrorRetryConditionField(view, condition.Field)
			switch evaluateChannelErrorRetryCondition(condition, fieldValue, truncated) {
			case channelErrorRetryConditionFalse:
				ruleFalse = true
			case channelErrorRetryConditionUnknown:
				incomplete = true
			}
		}
		if ruleFalse {
			continue
		}
		if incomplete {
			return ChannelErrorRetryMatchResult{Status: ChannelErrorRetryInputIncomplete}
		}
		return ChannelErrorRetryMatchResult{Status: ChannelErrorRetryMatched, RuleID: rule.ID, Action: rule.Action}
	}
	return ChannelErrorRetryMatchResult{Status: ChannelErrorRetryMiss}
}

type channelErrorRetryConditionOutcome int

const (
	channelErrorRetryConditionFalse channelErrorRetryConditionOutcome = iota
	channelErrorRetryConditionTrue
	channelErrorRetryConditionUnknown
)

func channelErrorRetryConditionField(view ChannelErrorRetryView, field string) (string, bool) {
	switch field {
	case kitdto.ChannelErrorRetryFieldMessage:
		return view.Message, view.MessageTruncated
	case kitdto.ChannelErrorRetryFieldCode:
		return view.Code, view.CodeTruncated
	case kitdto.ChannelErrorRetryFieldType:
		return view.Type, view.TypeTruncated
	default:
		return "", false
	}
}

// evaluateChannelErrorRetryCondition returns false for an empty field, matching
// the contract that an empty field can never match, including not_contains.
func evaluateChannelErrorRetryCondition(condition kitdto.ChannelErrorRetryCondition, fieldValue string, truncated bool) channelErrorRetryConditionOutcome {
	if fieldValue == "" {
		return channelErrorRetryConditionFalse
	}
	target := condition.Value
	if !condition.CaseSensitive {
		fieldValue = strings.ToLower(fieldValue)
		target = strings.ToLower(target)
	}
	switch condition.Operator {
	case kitdto.ChannelErrorRetryOperatorEquals:
		if truncated {
			// The field is longer than the projection cap while a validated
			// target stays within MaxChannelErrorRetryValueRunes, so the two can
			// never be equal: equality is decidable here. Reporting unknown would
			// hand an oversized upstream error back to the legacy decision and
			// silently drop the configured stop rule.
			return channelErrorRetryConditionFalse
		}
		if fieldValue == target {
			return channelErrorRetryConditionTrue
		}
		return channelErrorRetryConditionFalse
	case kitdto.ChannelErrorRetryOperatorContains:
		if strings.Contains(fieldValue, target) {
			return channelErrorRetryConditionTrue
		}
		if truncated {
			return channelErrorRetryConditionUnknown
		}
		return channelErrorRetryConditionFalse
	case kitdto.ChannelErrorRetryOperatorNotContains:
		if strings.Contains(fieldValue, target) {
			return channelErrorRetryConditionFalse
		}
		if truncated {
			return channelErrorRetryConditionUnknown
		}
		return channelErrorRetryConditionTrue
	default:
		return channelErrorRetryConditionFalse
	}
}

// decideChannelErrorRetry resolves the current channel policy and evaluates it.
// handled=false means the caller must run the legacy decision block unchanged;
// that covers absent/disabled policy and out-of-scope requests. When the policy
// is active, the legacy decision is still used for inherit cases but the
// bounded audit is attached from this same evaluation.
func decideChannelErrorRetry(c *gin.Context, err *types.NewAPIError, retryTimes int) (PolicyDecision, bool) {
	if err == nil || !ChannelErrorRetryHTTPActive(c) {
		return PolicyDecision{}, false
	}
	upstream := err.GetUpstreamHTTPError()
	if upstream == nil || upstream.OriginalStatusCode < 400 || upstream.OriginalStatusCode > 599 {
		// Only a real upstream 4xx/5xx participates; redirects and other
		// non-error responses keep the legacy decision.
		return PolicyDecision{}, false
	}
	setting, ok := common.GetContextKeyType[kitdto.ChannelSettings](c, constant.ContextKeyChannelSetting)
	if !ok {
		return PolicyDecision{}, false
	}
	if setting.ErrorRetryPolicyDiagnostic != "" {
		decision := decideRelayRetryLegacy(c, err, retryTimes)
		decision.Audit = &PolicyDecisionAudit{Status: "invalid", Diagnostic: setting.ErrorRetryPolicyDiagnostic}
		return decision, true
	}
	policy := setting.ErrorRetryPolicy
	if policy == nil || !policy.Enabled {
		return PolicyDecision{}, false
	}

	view := buildChannelErrorRetryView(err)
	result := MatchChannelErrorRetry(policy, view)
	switch result.Status {
	case ChannelErrorRetryInputIncomplete:
		decision := decideRelayRetryLegacy(c, err, retryTimes)
		audit := channelErrorRetryAudit("input_incomplete", err, view)
		decision.Audit = &audit
		return decision, true
	case ChannelErrorRetryMatched:
		return decideMatchedChannelErrorRetry(c, err, retryTimes, result, view), true
	default:
		decision := decideRelayRetryLegacy(c, err, retryTimes)
		audit := channelErrorRetryAudit("miss", err, view)
		decision.Audit = &audit
		return decision, true
	}
}

// decideMatchedChannelErrorRetry applies the original hard limits, in their
// original order, before honoring the matched rule. A hard limit keeps its own
// source/reason and only records the unmatched candidate rule in the audit.
func decideMatchedChannelErrorRetry(c *gin.Context, err *types.NewAPIError, retryTimes int, result ChannelErrorRetryMatchResult, view ChannelErrorRetryView) PolicyDecision {
	hardLimit := func(reason, source string) PolicyDecision {
		audit := channelErrorRetryAudit("match_limited", err, view)
		audit.CandidateRuleID = result.RuleID
		return PolicyDecision{Action: "stop", Reason: reason, Source: source, Audit: &audit}
	}

	if ShouldSkipRetryAfterChannelAffinityFailure(c) {
		source := RequestPolicy(c).SessionModeSource
		if source == "" {
			source = "session_rule"
		}
		return hardLimit("strict_session", source)
	}
	if GetChannelConstraints(c).SuppressesRetry() {
		return hardLimit("pinned_channel", "channel_constraint")
	}
	if types.IsSkipRetryError(err) {
		return hardLimit("non_retryable_error", "system")
	}
	if channelErrorRetryCanceledOrCommitted(c) {
		return hardLimit("response_committed", "system")
	}
	if retryTimes <= 0 {
		return hardLimit("attempt_budget_exhausted", "global")
	}
	if code := err.StatusCode; code >= 200 && code < 300 {
		return hardLimit("system_retry_exclusion", "system")
	}
	if operation_setting.IsAlwaysSkipRetryCode(err.GetErrorCode()) || operation_setting.IsAlwaysSkipRetryStatusCode(err.StatusCode) {
		return hardLimit("system_retry_exclusion", "system")
	}

	audit := channelErrorRetryAudit("matched", err, view)
	return PolicyDecision{
		Action: result.Action,
		Reason: "channel_error_rule_" + result.Action,
		Source: "channel_rule",
		RuleID: result.RuleID,
		Audit:  &audit,
	}
}

func channelErrorRetryCanceledOrCommitted(c *gin.Context) bool {
	if c == nil {
		return false
	}
	if c.Request != nil && c.Request.Context().Err() != nil {
		return true
	}
	return ResponseCommitted(c)
}

func channelErrorRetryAudit(status string, err *types.NewAPIError, view ChannelErrorRetryView) PolicyDecisionAudit {
	audit := PolicyDecisionAudit{Status: status, MatchedStatus: view.StatusCode}
	if upstream := err.GetUpstreamHTTPError(); upstream != nil {
		audit.UpstreamStatus = upstream.OriginalStatusCode
	}
	if !view.TypeTruncated {
		audit.RetryErrorType = view.Type
	}
	return audit
}
