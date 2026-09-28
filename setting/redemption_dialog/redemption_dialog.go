package redemption_dialog

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
	CodeConfigEmpty       ErrorCode = "config_empty"
	CodeConfigNotObject   ErrorCode = "config_not_object"
	CodeConfigInvalidJSON ErrorCode = "config_invalid_json"
	CodeUnknownField      ErrorCode = "unknown_field"
	CodeEnabledRequired   ErrorCode = "enabled_required"
	CodeEnabledNotBool    ErrorCode = "enabled_not_bool"
	CodeFieldNotString    ErrorCode = "field_not_string"
	CodeFieldTooLong      ErrorCode = "field_too_long"
	CodeFieldBlank        ErrorCode = "field_blank"
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
// redemption-success-dialog document as one normalized JSON value.
const OptionKey = "RedemptionSuccessDialog"

// Limits frozen by the phase B specification. Character limits are Unicode
// code points, matching the settings UI.
const (
	MaxTitleLength       = 80
	MaxContentLength     = 4000
	MaxCloseButtonLength = 20

	// MaxConfigBodyBytes bounds the config PUT body.
	MaxConfigBodyBytes = 2 << 20
)

const defaultJSON = `{"enabled":false,"title":"","content":"","close_button_text":""}`

// Config is the persisted and transmitted shape of the whole feature. The
// content is Markdown source, never pre-rendered HTML; title and close button
// text stay plain text.
type Config struct {
	Enabled         bool   `json:"enabled"`
	Title           string `json:"title"`
	Content         string `json:"content"`
	CloseButtonText string `json:"close_button_text"`
}

// DefaultConfig is the disabled, empty baseline used when no value is stored or
// when a stored value cannot be read.
func DefaultConfig() Config {
	return Config{Enabled: false}
}

// DefaultJSON returns the canonical JSON for DefaultConfig.
func DefaultJSON() string {
	return defaultJSON
}

// Encode normalizes and marshals the config through the host JSON wrapper.
func Encode(cfg Config) (string, error) {
	encoded, err := common.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// ParseConfig strictly decodes and validates a persisted or submitted config.
// Unknown fields, a non-boolean enabled flag, JSON null strings and trailing
// JSON are all rejected. The three text fields may be omitted, but when the
// dialog is enabled they must not be blank.
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
		case "enabled", "title", "content", "close_button_text":
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

	title, err := optionalString(top, "title")
	if err != nil {
		return Config{}, err
	}
	content, err := optionalString(top, "content")
	if err != nil {
		return Config{}, err
	}
	closeButtonText, err := optionalString(top, "close_button_text")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Enabled:         enabled,
		Title:           title,
		Content:         content,
		CloseButtonText: closeButtonText,
	}
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks the rune limits and, when the dialog is enabled, requires
// every visible text field to be non-blank. A disabled config may keep an empty
// title, content and close button text.
func Validate(cfg Config) error {
	if utf8.RuneCountInString(cfg.Title) > MaxTitleLength {
		return newConfigError(CodeFieldTooLong, fmt.Sprintf("title is too long (max %d characters)", MaxTitleLength), map[string]any{"Field": "title", "Max": MaxTitleLength})
	}
	if utf8.RuneCountInString(cfg.Content) > MaxContentLength {
		return newConfigError(CodeFieldTooLong, fmt.Sprintf("content is too long (max %d characters)", MaxContentLength), map[string]any{"Field": "content", "Max": MaxContentLength})
	}
	if utf8.RuneCountInString(cfg.CloseButtonText) > MaxCloseButtonLength {
		return newConfigError(CodeFieldTooLong, fmt.Sprintf("close_button_text is too long (max %d characters)", MaxCloseButtonLength), map[string]any{"Field": "close_button_text", "Max": MaxCloseButtonLength})
	}
	if !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.Title) == "" {
		return newConfigError(CodeFieldBlank, "title must not be blank", map[string]any{"Field": "title"})
	}
	if strings.TrimSpace(cfg.Content) == "" {
		return newConfigError(CodeFieldBlank, "content must not be blank", map[string]any{"Field": "content"})
	}
	if strings.TrimSpace(cfg.CloseButtonText) == "" {
		return newConfigError(CodeFieldBlank, "close_button_text must not be blank", map[string]any{"Field": "close_button_text"})
	}
	return nil
}

func isJSONNull(raw common.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func optionalString(fields map[string]common.RawMessage, key string) (string, error) {
	raw, ok := fields[key]
	if !ok {
		return "", nil
	}
	if isJSONNull(raw) {
		return "", newConfigError(CodeFieldNotString, fmt.Sprintf("%s must be a string", key), map[string]any{"Field": key})
	}
	var value string
	if err := common.Unmarshal(raw, &value); err != nil {
		return "", newConfigError(CodeFieldNotString, fmt.Sprintf("%s must be a string", key), map[string]any{"Field": key})
	}
	return value, nil
}
