package error_mapping

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
)

// ErrorCode identifies a validation failure. The host maps the code to a
// localized message, so config errors stay translatable without the setting
// package depending on the request context or the i18n package.
type ErrorCode string

const (
	CodeConfigEmpty         ErrorCode = "config_empty"
	CodeConfigNotObject     ErrorCode = "config_not_object"
	CodeConfigInvalidJSON   ErrorCode = "config_invalid_json"
	CodeUnknownField        ErrorCode = "unknown_field"
	CodeEnabledRequired     ErrorCode = "enabled_required"
	CodeEnabledNotBool      ErrorCode = "enabled_not_bool"
	CodeRulesRequired       ErrorCode = "rules_required"
	CodeRulesNotArray       ErrorCode = "rules_not_array"
	CodeTooManyRules        ErrorCode = "too_many_rules"
	CodeRuleNotObject       ErrorCode = "rule_not_object"
	CodeRuleUnknownField    ErrorCode = "rule_unknown_field"
	CodeFieldRequired       ErrorCode = "rule_field_required"
	CodeFieldNotString      ErrorCode = "rule_field_not_string"
	CodeFieldNotBool        ErrorCode = "rule_field_not_bool"
	CodeIDInvalid           ErrorCode = "rule_id_invalid"
	CodeNameTooLong         ErrorCode = "rule_name_too_long"
	CodeKeywordRequired     ErrorCode = "rule_keyword_required"
	CodeKeywordTooLong      ErrorCode = "rule_keyword_too_long"
	CodeKeywordBlank        ErrorCode = "rule_keyword_blank"
	CodeReplacementRequired ErrorCode = "rule_replacement_required"
	CodeReplacementTooLong  ErrorCode = "rule_replacement_too_long"
	CodeReplacementBlank    ErrorCode = "rule_replacement_blank"
	CodeDuplicateID         ErrorCode = "rule_duplicate_id"
)

// ConfigError is a validation failure with a stable code and template
// parameters. Error() stays an English fallback for logs and tests; the host
// translates Code() with Params() for the client.
type ConfigError struct {
	code    ErrorCode
	message string
	params  map[string]any
}

func (e *ConfigError) Error() string { return e.message }

// Code returns the stable failure code.
func (e *ConfigError) Code() ErrorCode { return e.code }

// Params returns the template data for the localized message.
func (e *ConfigError) Params() map[string]any { return e.params }

func newConfigError(code ErrorCode, message string, params map[string]any) *ConfigError {
	return &ConfigError{code: code, message: message, params: params}
}

// OptionKey is the single options-table key that stores the whole
// error-message-mapping document as one normalized JSON value.
const OptionKey = "ErrorMessageMapping"

// Limits frozen by the phase B specification.
const (
	MaxRules             = 100
	MaxIDLength          = 64
	MaxNameLength        = 80
	MaxKeywordLength     = 256
	MaxReplacementLength = 2048

	// MaxConfigBodyBytes bounds the config PUT body and the preview body.
	MaxConfigBodyBytes  = 2 << 20
	MaxPreviewBodyBytes = 2 << 20
	// MaxPreviewMessageBytes bounds the sample error message in preview.
	MaxPreviewMessageBytes = 65536
)

const defaultJSON = `{"enabled":false,"rules":[]}`

// Rule is one ordered match rule. The order in Config.Rules is the priority
// order: the first enabled match wins.
type Rule struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Enabled       bool   `json:"enabled"`
	Keyword       string `json:"keyword"`
	CaseSensitive bool   `json:"case_sensitive"`
	Replacement   string `json:"replacement"`
}

// Config is the persisted and transmitted shape of the whole feature.
type Config struct {
	Enabled bool   `json:"enabled"`
	Rules   []Rule `json:"rules"`
}

// Result is the outcome of matching one client-facing error message.
type Result struct {
	Message string
	Matched bool
	RuleID  string
}

// DefaultConfig is the disabled, rule-less baseline used when no value is
// stored or when a stored value cannot be read.
func DefaultConfig() Config {
	return Config{Enabled: false, Rules: []Rule{}}
}

// DefaultJSON returns the canonical JSON for DefaultConfig.
func DefaultJSON() string {
	return defaultJSON
}

// Clone returns a deep-enough copy: Rule carries no reference fields. A non-nil
// empty slice stays a non-nil empty slice so callers that serialize the clone
// keep emitting [] instead of null.
func (c Config) Clone() Config {
	cloned := c
	if c.Rules != nil {
		cloned.Rules = make([]Rule, len(c.Rules))
		copy(cloned.Rules, c.Rules)
	}
	return cloned
}

// Encode normalizes nil rule slices and marshals the config through the host
// JSON wrapper.
func Encode(cfg Config) (string, error) {
	normalized := cfg
	if normalized.Rules == nil {
		normalized.Rules = []Rule{}
	}
	encoded, err := common.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// ParseConfig strictly decodes and validates a persisted or submitted config.
// Unknown fields, missing required fields, JSON null for required fields and
// trailing JSON are all rejected.
func ParseConfig(raw []byte) (Config, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return Config{}, newConfigError(CodeConfigEmpty, "config is empty", nil)
	}
	if isJSONNull(trimmed) {
		return Config{}, newConfigError(CodeConfigNotObject, "config must be an object", nil)
	}

	var top map[string]common.RawMessage
	if err := common.Unmarshal(raw, &top); err != nil {
		return Config{}, newConfigError(CodeConfigInvalidJSON, fmt.Sprintf("invalid config json: %v", err), map[string]any{"Detail": err.Error()})
	}
	for key := range top {
		switch key {
		case "enabled", "rules":
		default:
			return Config{}, newConfigError(CodeUnknownField, fmt.Sprintf("unknown field %q", key), map[string]any{"Field": key})
		}
	}

	rawEnabled, ok := top["enabled"]
	if !ok {
		return Config{}, newConfigError(CodeEnabledRequired, "enabled is required", nil)
	}
	if isJSONNull(rawEnabled) {
		return Config{}, newConfigError(CodeEnabledNotBool, "enabled must be a boolean", nil)
	}
	var enabled bool
	if err := common.Unmarshal(rawEnabled, &enabled); err != nil {
		return Config{}, newConfigError(CodeEnabledNotBool, "enabled must be a boolean", nil)
	}

	rawRules, ok := top["rules"]
	if !ok {
		return Config{}, newConfigError(CodeRulesRequired, "rules is required", nil)
	}
	if isJSONNull(rawRules) {
		return Config{}, newConfigError(CodeRulesNotArray, "rules must be an array", nil)
	}
	var rawRuleList []common.RawMessage
	if err := common.Unmarshal(rawRules, &rawRuleList); err != nil {
		return Config{}, newConfigError(CodeRulesNotArray, "rules must be an array", nil)
	}
	if len(rawRuleList) > MaxRules {
		return Config{}, newConfigError(CodeTooManyRules, fmt.Sprintf("too many rules: %d (max %d)", len(rawRuleList), MaxRules), map[string]any{"Count": len(rawRuleList), "Max": MaxRules})
	}

	cfg := Config{Enabled: enabled, Rules: make([]Rule, 0, len(rawRuleList))}
	for index, rawRule := range rawRuleList {
		rule, err := parseRule(rawRule, index)
		if err != nil {
			return Config{}, err
		}
		cfg.Rules = append(cfg.Rules, rule)
	}
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func parseRule(raw common.RawMessage, index int) (Rule, error) {
	trimmed := bytes.TrimSpace(raw)
	if isJSONNull(trimmed) {
		return Rule{}, newConfigError(CodeRuleNotObject, fmt.Sprintf("rule %d must be an object", index), map[string]any{"Index": index})
	}
	var fields map[string]common.RawMessage
	if err := common.Unmarshal(raw, &fields); err != nil {
		return Rule{}, newConfigError(CodeRuleNotObject, fmt.Sprintf("rule %d must be an object", index), map[string]any{"Index": index})
	}
	for key := range fields {
		switch key {
		case "id", "name", "enabled", "keyword", "case_sensitive", "replacement":
		default:
			return Rule{}, newConfigError(CodeRuleUnknownField, fmt.Sprintf("rule %d: unknown field %q", index, key), map[string]any{"Index": index, "Field": key})
		}
	}

	id, err := requiredRuleString(fields, "id", index)
	if err != nil {
		return Rule{}, err
	}
	name, err := optionalRuleString(fields, "name", index)
	if err != nil {
		return Rule{}, err
	}
	keyword, err := requiredRuleString(fields, "keyword", index)
	if err != nil {
		return Rule{}, err
	}
	replacement, err := requiredRuleString(fields, "replacement", index)
	if err != nil {
		return Rule{}, err
	}
	enabled, err := requiredRuleBool(fields, "enabled", index)
	if err != nil {
		return Rule{}, err
	}
	caseSensitive, err := requiredRuleBool(fields, "case_sensitive", index)
	if err != nil {
		return Rule{}, err
	}

	return Rule{
		ID:            id,
		Name:          name,
		Enabled:       enabled,
		Keyword:       keyword,
		CaseSensitive: caseSensitive,
		Replacement:   replacement,
	}, nil
}

// Validate checks limits, identifier shape and duplicate identifiers.
func Validate(cfg Config) error {
	if len(cfg.Rules) > MaxRules {
		return newConfigError(CodeTooManyRules, fmt.Sprintf("too many rules: %d (max %d)", len(cfg.Rules), MaxRules), map[string]any{"Count": len(cfg.Rules), "Max": MaxRules})
	}
	seen := make(map[string]struct{}, len(cfg.Rules))
	for index, rule := range cfg.Rules {
		if err := validateRule(rule, index); err != nil {
			return err
		}
		if _, duplicate := seen[rule.ID]; duplicate {
			return newConfigError(CodeDuplicateID, fmt.Sprintf("rule %d: duplicate id %q", index, rule.ID), map[string]any{"Index": index, "ID": rule.ID})
		}
		seen[rule.ID] = struct{}{}
	}
	return nil
}

func validateRule(rule Rule, index int) error {
	if !validID(rule.ID) {
		return newConfigError(CodeIDInvalid, fmt.Sprintf("rule %d: id must be 1-%d characters of letters, digits, '-' or '_'", index, MaxIDLength), map[string]any{"Index": index, "Max": MaxIDLength})
	}
	if utf8.RuneCountInString(rule.Name) > MaxNameLength {
		return newConfigError(CodeNameTooLong, fmt.Sprintf("rule %d: name is too long (max %d characters)", index, MaxNameLength), map[string]any{"Index": index, "Max": MaxNameLength})
	}
	if utf8.RuneCountInString(rule.Keyword) == 0 {
		return newConfigError(CodeKeywordRequired, fmt.Sprintf("rule %d: keyword is required", index), map[string]any{"Index": index})
	}
	if utf8.RuneCountInString(rule.Keyword) > MaxKeywordLength {
		return newConfigError(CodeKeywordTooLong, fmt.Sprintf("rule %d: keyword is too long (max %d characters)", index, MaxKeywordLength), map[string]any{"Index": index, "Max": MaxKeywordLength})
	}
	if strings.TrimSpace(rule.Keyword) == "" {
		return newConfigError(CodeKeywordBlank, fmt.Sprintf("rule %d: keyword must not be blank", index), map[string]any{"Index": index})
	}
	if utf8.RuneCountInString(rule.Replacement) == 0 {
		return newConfigError(CodeReplacementRequired, fmt.Sprintf("rule %d: replacement is required", index), map[string]any{"Index": index})
	}
	if utf8.RuneCountInString(rule.Replacement) > MaxReplacementLength {
		return newConfigError(CodeReplacementTooLong, fmt.Sprintf("rule %d: replacement is too long (max %d characters)", index, MaxReplacementLength), map[string]any{"Index": index, "Max": MaxReplacementLength})
	}
	if strings.TrimSpace(rule.Replacement) == "" {
		return newConfigError(CodeReplacementBlank, fmt.Sprintf("rule %d: replacement must not be blank", index), map[string]any{"Index": index})
	}
	return nil
}

func validID(id string) bool {
	if len(id) == 0 || len(id) > MaxIDLength {
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

func isJSONNull(raw common.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func requiredRuleString(fields map[string]common.RawMessage, key string, index int) (string, error) {
	raw, ok := fields[key]
	if !ok {
		return "", newConfigError(CodeFieldRequired, fmt.Sprintf("rule %d: %s is required", index, key), map[string]any{"Index": index, "Field": key})
	}
	if isJSONNull(raw) {
		return "", newConfigError(CodeFieldNotString, fmt.Sprintf("rule %d: %s must be a string", index, key), map[string]any{"Index": index, "Field": key})
	}
	var value string
	if err := common.Unmarshal(raw, &value); err != nil {
		return "", newConfigError(CodeFieldNotString, fmt.Sprintf("rule %d: %s must be a string", index, key), map[string]any{"Index": index, "Field": key})
	}
	return value, nil
}

func optionalRuleString(fields map[string]common.RawMessage, key string, index int) (string, error) {
	raw, ok := fields[key]
	if !ok {
		return "", nil
	}
	if isJSONNull(raw) {
		return "", newConfigError(CodeFieldNotString, fmt.Sprintf("rule %d: %s must be a string", index, key), map[string]any{"Index": index, "Field": key})
	}
	var value string
	if err := common.Unmarshal(raw, &value); err != nil {
		return "", newConfigError(CodeFieldNotString, fmt.Sprintf("rule %d: %s must be a string", index, key), map[string]any{"Index": index, "Field": key})
	}
	return value, nil
}

func requiredRuleBool(fields map[string]common.RawMessage, key string, index int) (bool, error) {
	raw, ok := fields[key]
	if !ok {
		return false, newConfigError(CodeFieldRequired, fmt.Sprintf("rule %d: %s is required", index, key), map[string]any{"Index": index, "Field": key})
	}
	if isJSONNull(raw) {
		return false, newConfigError(CodeFieldNotBool, fmt.Sprintf("rule %d: %s must be a boolean", index, key), map[string]any{"Index": index, "Field": key})
	}
	var value bool
	if err := common.Unmarshal(raw, &value); err != nil {
		return false, newConfigError(CodeFieldNotBool, fmt.Sprintf("rule %d: %s must be a boolean", index, key), map[string]any{"Index": index, "Field": key})
	}
	return value, nil
}

// compiledRule holds a rule with its precomputed lowercase keyword. Rules are
// private to Matcher so a published matcher stays immutable.
type compiledRule struct {
	id            string
	enabled       bool
	keyword       string
	lowerKeyword  string
	caseSensitive bool
	replacement   string
}

// Matcher is an immutable, compiled ruleset. It is safe for concurrent use and
// may be shared by every request that read the same snapshot.
type Matcher struct {
	enabled bool
	rules   []compiledRule
}

// Compile validates cfg and precomputes the lowercase keywords used for
// case-insensitive matching. A nil Matcher is safe to call Match on.
func Compile(cfg Config) (*Matcher, error) {
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	matcher := &Matcher{enabled: cfg.Enabled, rules: make([]compiledRule, 0, len(cfg.Rules))}
	for _, rule := range cfg.Rules {
		compiled := compiledRule{
			id:            rule.ID,
			enabled:       rule.Enabled,
			keyword:       rule.Keyword,
			caseSensitive: rule.CaseSensitive,
			replacement:   rule.Replacement,
		}
		if !rule.CaseSensitive {
			compiled.lowerKeyword = strings.ToLower(rule.Keyword)
		}
		matcher.rules = append(matcher.rules, compiled)
	}
	return matcher, nil
}

// Match returns the replacement for the first enabled rule whose keyword is
// contained in message. Matching is plain substring containment, case-folded
// with strings.ToLower when the rule is case-insensitive, and it is not
// recursive: the returned replacement is never matched again.
func (m *Matcher) Match(message string) Result {
	if m == nil || !m.enabled {
		return Result{Message: message}
	}
	lower := ""
	lowerReady := false
	for _, rule := range m.rules {
		if !rule.enabled {
			continue
		}
		if rule.caseSensitive {
			if strings.Contains(message, rule.keyword) {
				return Result{Message: rule.replacement, Matched: true, RuleID: rule.id}
			}
			continue
		}
		if !lowerReady {
			lower = strings.ToLower(message)
			lowerReady = true
		}
		if strings.Contains(lower, rule.lowerKeyword) {
			return Result{Message: rule.replacement, Matched: true, RuleID: rule.id}
		}
	}
	return Result{Message: message}
}
