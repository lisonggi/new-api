package model

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/error_mapping"
	"gorm.io/gorm"
)

// ErrorMessageMappingSnapshot is the immutable read model behind the
// error-message-mapping settings. Requests read one snapshot and use it for
// their whole lifetime, so a save never changes the rules mid-request.
type ErrorMessageMappingSnapshot struct {
	Config  error_mapping.Config
	JSON    string
	Matcher *error_mapping.Matcher
}

var errorMessageMappingSnapshot atomic.Pointer[ErrorMessageMappingSnapshot]

// errorMessageMappingMutex serializes this feature's database reads, saves and
// snapshot publishes. Lock order is always errorMessageMappingMutex ->
// OptionMapRWMutex; it never nests with the other option mutexes.
var errorMessageMappingMutex sync.Mutex

func newErrorMessageMappingSnapshot(cfg error_mapping.Config) (*ErrorMessageMappingSnapshot, error) {
	normalized := cfg.Clone()
	if normalized.Rules == nil {
		normalized.Rules = []error_mapping.Rule{}
	}
	encoded, err := error_mapping.Encode(normalized)
	if err != nil {
		return nil, err
	}
	matcher, err := error_mapping.Compile(normalized)
	if err != nil {
		return nil, err
	}
	return &ErrorMessageMappingSnapshot{Config: normalized, JSON: encoded, Matcher: matcher}, nil
}

// CurrentErrorMessageMapping returns the active snapshot with its config
// cloned, so a caller cannot mutate the published rules. The compiled matcher
// is immutable and therefore safe to share. Before options load (or when no
// value is stored) it returns the disabled default.
func CurrentErrorMessageMapping() *ErrorMessageMappingSnapshot {
	snapshot := loadErrorMessageMappingSnapshot()
	return &ErrorMessageMappingSnapshot{
		Config:  snapshot.Config.Clone(),
		JSON:    snapshot.JSON,
		Matcher: snapshot.Matcher,
	}
}

// loadErrorMessageMappingSnapshot returns the shared immutable snapshot. The
// first read installs the default with CompareAndSwap so a concurrent
// save/load publish is never replaced by the lazily built default.
func loadErrorMessageMappingSnapshot() *ErrorMessageMappingSnapshot {
	if snapshot := errorMessageMappingSnapshot.Load(); snapshot != nil {
		return snapshot
	}
	snapshot, err := newErrorMessageMappingSnapshot(error_mapping.DefaultConfig())
	if err != nil {
		return &ErrorMessageMappingSnapshot{Config: error_mapping.DefaultConfig(), JSON: error_mapping.DefaultJSON()}
	}
	if errorMessageMappingSnapshot.CompareAndSwap(nil, snapshot) {
		return snapshot
	}
	return errorMessageMappingSnapshot.Load()
}

// ErrorMessageMappingOptionKey exposes the options-table key for host wiring
// without importing the setting package from the option loader.
func ErrorMessageMappingOptionKey() string {
	return error_mapping.OptionKey
}

// MapErrorLogContent applies the active error-message mapping to one stored
// error-log content string for user-facing display. Matching follows the same
// semantics as the client-facing relay output: plain substring containment on
// the first enabled rule, and a disabled config never matches. A miss keeps
// the original content byte for byte; a match replaces the whole content with
// the configured replacement so no fragment of the original error leaks.
func MapErrorLogContent(content string) string {
	if content == "" {
		return content
	}
	snapshot := loadErrorMessageMappingSnapshot()
	if snapshot.Matcher == nil {
		return content
	}
	result := snapshot.Matcher.Match(content)
	if !result.Matched {
		return content
	}
	return result.Message
}

// SaveErrorMessageMapping validates, persists and publishes the whole config.
// The snapshot is only replaced after the database transaction commits, so a
// failed save leaves the previous rules active.
func SaveErrorMessageMapping(cfg error_mapping.Config) (error_mapping.Config, error) {
	snapshot, err := newErrorMessageMappingSnapshot(cfg)
	if err != nil {
		return error_mapping.Config{}, err
	}

	errorMessageMappingMutex.Lock()
	defer errorMessageMappingMutex.Unlock()

	if err := DB.Transaction(func(tx *gorm.DB) error {
		option := Option{Key: error_mapping.OptionKey}
		if err := tx.FirstOrCreate(&option, Option{Key: error_mapping.OptionKey}).Error; err != nil {
			return err
		}
		return tx.Model(&option).Update("value", snapshot.JSON).Error
	}); err != nil {
		return error_mapping.Config{}, err
	}

	publishErrorMessageMapping(snapshot)
	return snapshot.Config.Clone(), nil
}

// loadErrorMessageMapping re-reads this key and republishes. A missing record
// publishes the disabled default; a query failure or invalid stored value
// keeps the last valid snapshot instead of replacing it.
func loadErrorMessageMapping() error {
	errorMessageMappingMutex.Lock()
	defer errorMessageMappingMutex.Unlock()

	var option Option
	err := DB.Where(commonKeyCol+" = ?", error_mapping.OptionKey).First(&option).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			snapshot, buildErr := newErrorMessageMappingSnapshot(error_mapping.DefaultConfig())
			if buildErr != nil {
				return buildErr
			}
			publishErrorMessageMapping(snapshot)
			return nil
		}
		return err
	}

	cfg, err := error_mapping.ParseConfig([]byte(option.Value))
	if err != nil {
		return err
	}
	snapshot, err := newErrorMessageMappingSnapshot(cfg)
	if err != nil {
		return err
	}
	publishErrorMessageMapping(snapshot)
	return nil
}

func publishErrorMessageMapping(snapshot *ErrorMessageMappingSnapshot) {
	common.OptionMapRWMutex.Lock()
	common.OptionMap[error_mapping.OptionKey] = snapshot.JSON
	common.OptionMapRWMutex.Unlock()
	errorMessageMappingSnapshot.Store(snapshot)
}
