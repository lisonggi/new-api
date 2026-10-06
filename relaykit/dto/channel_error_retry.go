package dto

import (
	"bytes"
	"strconv"
	"strings"
	"unicode/utf8"

	"encoding/json"

	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
)

// Error retry policy limits. The persisted policy must fit the existing
// Channel.Setting TEXT column on every supported database, so the per-field
// limits are paired with a whole-policy and whole-setting byte budget.
const (
	MaxChannelErrorRetryRules       = 32
	MaxChannelErrorRetryConditions  = 8
	MaxChannelErrorRetryStatusCodes = 500
	MaxChannelErrorRetryIDBytes     = 64
	MaxChannelErrorRetryNameRunes   = 64
	MaxChannelErrorRetryValueRunes  = 512
	MaxChannelErrorRetryPolicyBytes = 32 * 1024
	MaxChannelSettingBytes          = 65535
)

// Error retry actions and matcher enums.
const (
	ChannelErrorRetryActionRetry = "retry"
	ChannelErrorRetryActionStop  = "stop"

	ChannelErrorRetryFieldMessage = "message"
	ChannelErrorRetryFieldCode    = "code"
	ChannelErrorRetryFieldType    = "type"

	ChannelErrorRetryOperatorEquals      = "equals"
	ChannelErrorRetryOperatorContains    = "contains"
	ChannelErrorRetryOperatorNotContains = "not_contains"

	ChannelErrorRetryStatusMin = 100
	ChannelErrorRetryStatusMax = 599
)

// ChannelErrorRetryPolicy is the optional per-channel error retry classification.
// Retry and stop only express whether the current error may be retried by the
// existing retry mechanism; they never select a channel or change counters.
type ChannelErrorRetryPolicy struct {
	Enabled bool                    `json:"enabled"`
	Rules   []ChannelErrorRetryRule `json:"rules,omitempty"`
}

type ChannelErrorRetryRule struct {
	ID          string                       `json:"id"`
	Name        string                       `json:"name,omitempty"`
	Enabled     bool                         `json:"enabled"`
	Action      string                       `json:"action"`
	StatusCodes []int                        `json:"status_codes,omitempty"`
	Conditions  []ChannelErrorRetryCondition `json:"conditions,omitempty"`
}

type ChannelErrorRetryCondition struct {
	Field         string `json:"field"`
	Operator      string `json:"operator"`
	Value         string `json:"value"`
	CaseSensitive bool   `json:"case_sensitive"`
}

// ChannelErrorRetryValidationError carries a schema path and a short reason.
// The path is built only from schema field names and array indices so unknown
// configuration keys or values never leak into diagnostics.
type ChannelErrorRetryValidationError struct {
	Path   string
	Reason string
}

func (e *ChannelErrorRetryValidationError) Error() string {
	if e == nil {
		return ""
	}
	if e.Path == "" {
		return e.Reason
	}
	return e.Path + ": " + e.Reason
}

func retryPolicyError(path, reason string) *ChannelErrorRetryValidationError {
	return &ChannelErrorRetryValidationError{Path: path, Reason: reason}
}

// ParseChannelErrorRetryPolicyInSetting strictly validates and decodes the
// optional error_retry_policy field of a raw Channel.Setting JSON object.
//
// It returns the parsed policy when the canonical field is present and valid,
// present=false when the field is absent or null (inherit), and a validation
// error otherwise. Unknown fields, explicit nulls inside the policy, duplicate
// keys (exact or case-variant), wrong types and out-of-range values are all
// rejected before any typed decode. Unrelated legacy settings keys are ignored.
func ParseChannelErrorRetryPolicyInSetting(rawSetting []byte) (*ChannelErrorRetryPolicy, bool, *ChannelErrorRetryValidationError) {
	policy, present, _, verr := ParseChannelErrorRetryPolicyInSettingWithRaw(rawSetting)
	return policy, present, verr
}

// ParseChannelErrorRetryPolicyInSettingWithRaw is ParseChannelErrorRetryPolicyInSetting
// plus the raw bytes of the canonical field, decoded in the same pass. Callers
// that also need to bound or preserve that value (whole-policy byte budget,
// write-back of a stored-but-invalid policy) must use this form instead of
// decoding the same setting a second time.
func ParseChannelErrorRetryPolicyInSettingWithRaw(rawSetting []byte) (*ChannelErrorRetryPolicy, bool, json.RawMessage, *ChannelErrorRetryValidationError) {
	exact, folded, err := kitutil.TopLevelFieldOccurrences(rawSetting, "error_retry_policy")
	if err != nil {
		return nil, false, nil, retryPolicyError("", "invalid_json")
	}
	if folded == 0 {
		return nil, false, nil, nil
	}
	if exact != 1 || folded != 1 {
		return nil, false, nil, retryPolicyError("error_retry_policy", "non_canonical_field")
	}
	var fields map[string]json.RawMessage
	if err := kitutil.Unmarshal(rawSetting, &fields); err != nil {
		return nil, false, nil, retryPolicyError("", "invalid_json")
	}
	rawPolicy, ok := fields["error_retry_policy"]
	if !ok {
		return nil, false, nil, nil
	}
	value, err := kitutil.DecodeJSONValue(rawPolicy)
	if err != nil {
		return nil, false, nil, retryPolicyError("error_retry_policy", "invalid_json")
	}
	if value == nil {
		return nil, false, nil, nil
	}
	policy, verr := parseChannelErrorRetryPolicy(value)
	if verr != nil {
		return nil, false, rawPolicy, verr
	}
	return policy, true, rawPolicy, nil
}

// ChannelErrorRetryPolicyRawInSetting returns the raw bytes of the canonical
// error_retry_policy value when it is present and not JSON null. It is used to
// enforce the whole-policy byte budget and to preserve an invalid stored policy
// across internal read-modify-write paths.
func ChannelErrorRetryPolicyRawInSetting(rawSetting []byte) (json.RawMessage, bool, error) {
	var fields map[string]json.RawMessage
	if err := kitutil.Unmarshal(rawSetting, &fields); err != nil {
		return nil, false, err
	}
	raw, ok := fields["error_retry_policy"]
	if !ok {
		return nil, false, nil
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, false, nil
	}
	return raw, true, nil
}

// ChannelErrorRetryPolicyFieldAmbiguous reports whether a raw Channel.Setting
// references error_retry_policy more than once or under a non-canonical
// spelling. Such a value cannot be reproduced by re-encoding a map, so an
// internal read-modify-write must refuse the write instead of silently turning
// the last duplicate into a valid policy.
func ChannelErrorRetryPolicyFieldAmbiguous(rawSetting []byte) bool {
	exact, folded, err := kitutil.TopLevelFieldOccurrences(rawSetting, "error_retry_policy")
	if err != nil {
		return false
	}
	return folded > 0 && (exact != 1 || folded != 1)
}

func parseChannelErrorRetryPolicy(value any) (*ChannelErrorRetryPolicy, *ChannelErrorRetryValidationError) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, retryPolicyError("error_retry_policy", "not_object")
	}
	for key := range object {
		switch key {
		case "enabled", "rules":
		default:
			return nil, retryPolicyError("error_retry_policy", "unknown_field")
		}
	}
	policy := &ChannelErrorRetryPolicy{}
	rawEnabled, enabledPresent := object["enabled"]
	if enabledPresent {
		enabled, ok := rawEnabled.(bool)
		if !ok {
			return nil, retryPolicyError("error_retry_policy.enabled", "not_bool")
		}
		policy.Enabled = enabled
	}
	rawRules, ok := object["rules"]
	if !ok {
		return policy, nil
	}
	rules, ok := rawRules.([]any)
	if !ok {
		return nil, retryPolicyError("error_retry_policy.rules", "not_array")
	}
	if len(rules) > MaxChannelErrorRetryRules {
		return nil, retryPolicyError("error_retry_policy.rules", "too_many")
	}
	seenIDs := make(map[string]struct{}, len(rules))
	parsed := make([]ChannelErrorRetryRule, 0, len(rules))
	for index, rawRule := range rules {
		rule, verr := parseChannelErrorRetryRule(rawRule, index)
		if verr != nil {
			return nil, verr
		}
		if _, duplicate := seenIDs[rule.ID]; duplicate {
			return nil, retryPolicyError("error_retry_policy.rules["+strconv.Itoa(index)+"].id", "duplicate")
		}
		seenIDs[rule.ID] = struct{}{}
		parsed = append(parsed, rule)
	}
	if len(parsed) > 0 && !enabledPresent {
		// A policy that carries rules but never states whether it is enabled is a
		// silent no-op: a disabled policy is never evaluated and is not reported
		// in the decision audit, so the configured rules would never run and
		// nothing would say so. Rule-level problems are reported first; an empty
		// policy keeps inherit semantics.
		return nil, retryPolicyError("error_retry_policy.enabled", "required")
	}
	policy.Rules = parsed
	return policy, nil
}

func parseChannelErrorRetryRule(value any, index int) (ChannelErrorRetryRule, *ChannelErrorRetryValidationError) {
	path := "error_retry_policy.rules[" + strconv.Itoa(index) + "]"
	object, ok := value.(map[string]any)
	if !ok {
		return ChannelErrorRetryRule{}, retryPolicyError(path, "not_object")
	}
	for key := range object {
		switch key {
		case "id", "name", "enabled", "action", "status_codes", "conditions":
		default:
			return ChannelErrorRetryRule{}, retryPolicyError(path, "unknown_field")
		}
	}
	rule := ChannelErrorRetryRule{}

	rawID, ok := object["id"]
	if !ok {
		return rule, retryPolicyError(path+".id", "required")
	}
	id, ok := rawID.(string)
	if !ok {
		return rule, retryPolicyError(path+".id", "not_string")
	}
	if !validChannelErrorRetryID(id) {
		return rule, retryPolicyError(path+".id", "invalid_value")
	}
	rule.ID = id

	if rawName, ok := object["name"]; ok {
		name, ok := rawName.(string)
		if !ok {
			return rule, retryPolicyError(path+".name", "not_string")
		}
		if strings.ContainsRune(name, 0) || utf8.RuneCountInString(name) > MaxChannelErrorRetryNameRunes {
			return rule, retryPolicyError(path+".name", "invalid_value")
		}
		rule.Name = name
	}

	if rawEnabled, ok := object["enabled"]; ok {
		enabled, ok := rawEnabled.(bool)
		if !ok {
			return rule, retryPolicyError(path+".enabled", "not_bool")
		}
		rule.Enabled = enabled
	}

	rawAction, ok := object["action"]
	if !ok {
		return rule, retryPolicyError(path+".action", "required")
	}
	action, ok := rawAction.(string)
	if !ok {
		return rule, retryPolicyError(path+".action", "not_string")
	}
	switch action {
	case ChannelErrorRetryActionRetry, ChannelErrorRetryActionStop:
	default:
		return rule, retryPolicyError(path+".action", "invalid_value")
	}
	rule.Action = action

	if rawStatus, ok := object["status_codes"]; ok {
		statuses, ok := rawStatus.([]any)
		if !ok {
			return rule, retryPolicyError(path+".status_codes", "not_array")
		}
		if len(statuses) > MaxChannelErrorRetryStatusCodes {
			return rule, retryPolicyError(path+".status_codes", "too_many")
		}
		seen := make(map[int]struct{}, len(statuses))
		for _, rawStatus := range statuses {
			code, ok := retryPolicyInt(rawStatus)
			if !ok || code < ChannelErrorRetryStatusMin || code > ChannelErrorRetryStatusMax {
				return rule, retryPolicyError(path+".status_codes", "invalid_value")
			}
			if _, duplicate := seen[code]; duplicate {
				return rule, retryPolicyError(path+".status_codes", "duplicate")
			}
			seen[code] = struct{}{}
			rule.StatusCodes = append(rule.StatusCodes, code)
		}
	}

	if rawConditions, ok := object["conditions"]; ok {
		conditions, ok := rawConditions.([]any)
		if !ok {
			return rule, retryPolicyError(path+".conditions", "not_array")
		}
		if len(conditions) > MaxChannelErrorRetryConditions {
			return rule, retryPolicyError(path+".conditions", "too_many")
		}
		for conditionIndex, rawCondition := range conditions {
			condition, verr := parseChannelErrorRetryCondition(rawCondition, path+".conditions["+strconv.Itoa(conditionIndex)+"]")
			if verr != nil {
				return rule, verr
			}
			rule.Conditions = append(rule.Conditions, condition)
		}
	}

	if len(rule.StatusCodes) == 0 && len(rule.Conditions) == 0 {
		return rule, retryPolicyError(path, "empty_matcher")
	}
	return rule, nil
}

func parseChannelErrorRetryCondition(value any, path string) (ChannelErrorRetryCondition, *ChannelErrorRetryValidationError) {
	object, ok := value.(map[string]any)
	if !ok {
		return ChannelErrorRetryCondition{}, retryPolicyError(path, "not_object")
	}
	for key := range object {
		switch key {
		case "field", "operator", "value", "case_sensitive":
		default:
			return ChannelErrorRetryCondition{}, retryPolicyError(path, "unknown_field")
		}
	}
	condition := ChannelErrorRetryCondition{}

	rawField, ok := object["field"]
	if !ok {
		return condition, retryPolicyError(path+".field", "required")
	}
	field, ok := rawField.(string)
	if !ok {
		return condition, retryPolicyError(path+".field", "not_string")
	}
	switch field {
	case ChannelErrorRetryFieldMessage, ChannelErrorRetryFieldCode, ChannelErrorRetryFieldType:
	default:
		return condition, retryPolicyError(path+".field", "invalid_value")
	}
	condition.Field = field

	rawOperator, ok := object["operator"]
	if !ok {
		return condition, retryPolicyError(path+".operator", "required")
	}
	operator, ok := rawOperator.(string)
	if !ok {
		return condition, retryPolicyError(path+".operator", "not_string")
	}
	switch operator {
	case ChannelErrorRetryOperatorEquals, ChannelErrorRetryOperatorContains, ChannelErrorRetryOperatorNotContains:
	default:
		return condition, retryPolicyError(path+".operator", "invalid_value")
	}
	condition.Operator = operator

	rawValue, ok := object["value"]
	if !ok {
		return condition, retryPolicyError(path+".value", "required")
	}
	value_str, ok := rawValue.(string)
	if !ok {
		return condition, retryPolicyError(path+".value", "not_string")
	}
	if strings.ContainsRune(value_str, 0) || strings.TrimSpace(value_str) == "" || utf8.RuneCountInString(value_str) > MaxChannelErrorRetryValueRunes {
		return condition, retryPolicyError(path+".value", "invalid_value")
	}
	condition.Value = value_str

	if rawCase, ok := object["case_sensitive"]; ok {
		caseSensitive, ok := rawCase.(bool)
		if !ok {
			return condition, retryPolicyError(path+".case_sensitive", "not_bool")
		}
		condition.CaseSensitive = caseSensitive
	}
	return condition, nil
}

func retryPolicyInt(value any) (int, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	parsed, err := strconv.Atoi(number.String())
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func validChannelErrorRetryID(id string) bool {
	if len(id) == 0 || len(id) > MaxChannelErrorRetryIDBytes {
		return false
	}
	for _, char := range id {
		switch {
		case char >= 'a' && char <= 'z',
			char >= 'A' && char <= 'Z',
			char >= '0' && char <= '9',
			char == '-',
			char == '_':
		default:
			return false
		}
	}
	return true
}
