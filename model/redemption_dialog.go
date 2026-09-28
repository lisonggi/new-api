package model

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/redemption_dialog"
	"gorm.io/gorm"
)

// RedemptionDialogSnapshot is the immutable read model behind the
// redemption-success-dialog settings. A request reads one snapshot and uses it
// for the whole redemption, so a save never changes the payload mid-request.
// Config is a value type: copying it out is enough to keep the published value
// immutable.
type RedemptionDialogSnapshot struct {
	Config redemption_dialog.Config
	JSON   string
}

var redemptionDialogSnapshot atomic.Pointer[RedemptionDialogSnapshot]

// redemptionDialogMutex serializes this feature's database reads, saves and
// snapshot publishes. Lock order is always redemptionDialogMutex ->
// OptionMapRWMutex; it never nests with the other option mutexes.
var redemptionDialogMutex sync.Mutex

func newRedemptionDialogSnapshot(cfg redemption_dialog.Config) (*RedemptionDialogSnapshot, error) {
	if err := redemption_dialog.Validate(cfg); err != nil {
		return nil, err
	}
	encoded, err := redemption_dialog.Encode(cfg)
	if err != nil {
		return nil, err
	}
	return &RedemptionDialogSnapshot{Config: cfg, JSON: encoded}, nil
}

// CurrentRedemptionDialog returns the active snapshot with its config copied
// out, so a caller cannot mutate the published value. Before options load (or
// when no value is stored) it returns the disabled default.
func CurrentRedemptionDialog() *RedemptionDialogSnapshot {
	snapshot := loadRedemptionDialogSnapshot()
	return &RedemptionDialogSnapshot{
		Config: snapshot.Config,
		JSON:   snapshot.JSON,
	}
}

// loadRedemptionDialogSnapshot returns the shared immutable snapshot. It is
// private: callers must go through CurrentRedemptionDialog so they get a copy.
// The first read installs the default with CompareAndSwap so a concurrent
// save/load publish is never replaced by the lazily built default.
func loadRedemptionDialogSnapshot() *RedemptionDialogSnapshot {
	if snapshot := redemptionDialogSnapshot.Load(); snapshot != nil {
		return snapshot
	}
	snapshot, err := newRedemptionDialogSnapshot(redemption_dialog.DefaultConfig())
	if err != nil {
		return &RedemptionDialogSnapshot{Config: redemption_dialog.DefaultConfig(), JSON: redemption_dialog.DefaultJSON()}
	}
	if redemptionDialogSnapshot.CompareAndSwap(nil, snapshot) {
		return snapshot
	}
	return redemptionDialogSnapshot.Load()
}

// RedemptionDialogOptionKey exposes the options-table key for host wiring
// without importing the setting package from the option loader.
func RedemptionDialogOptionKey() string {
	return redemption_dialog.OptionKey
}

// SaveRedemptionDialog validates, persists and publishes the whole config. The
// snapshot is only replaced after the database transaction commits, so a failed
// save leaves the previous value active.
func SaveRedemptionDialog(cfg redemption_dialog.Config) (redemption_dialog.Config, error) {
	snapshot, err := newRedemptionDialogSnapshot(cfg)
	if err != nil {
		return redemption_dialog.Config{}, err
	}

	redemptionDialogMutex.Lock()
	defer redemptionDialogMutex.Unlock()

	if err := DB.Transaction(func(tx *gorm.DB) error {
		option := Option{Key: redemption_dialog.OptionKey}
		if err := tx.FirstOrCreate(&option, Option{Key: redemption_dialog.OptionKey}).Error; err != nil {
			return err
		}
		return tx.Model(&option).Update("value", snapshot.JSON).Error
	}); err != nil {
		return redemption_dialog.Config{}, err
	}

	publishRedemptionDialog(snapshot)
	return snapshot.Config, nil
}

// loadRedemptionDialog re-reads this key and republishes. A missing record
// publishes the disabled default; a query failure or invalid stored value keeps
// the last valid snapshot instead of replacing it.
func loadRedemptionDialog() error {
	redemptionDialogMutex.Lock()
	defer redemptionDialogMutex.Unlock()

	var option Option
	err := DB.Where(commonKeyCol+" = ?", redemption_dialog.OptionKey).First(&option).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			snapshot, buildErr := newRedemptionDialogSnapshot(redemption_dialog.DefaultConfig())
			if buildErr != nil {
				return buildErr
			}
			publishRedemptionDialog(snapshot)
			return nil
		}
		return err
	}

	cfg, err := redemption_dialog.ParseConfig([]byte(option.Value))
	if err != nil {
		return err
	}
	snapshot, err := newRedemptionDialogSnapshot(cfg)
	if err != nil {
		return err
	}
	publishRedemptionDialog(snapshot)
	return nil
}

func publishRedemptionDialog(snapshot *RedemptionDialogSnapshot) {
	common.OptionMapRWMutex.Lock()
	common.OptionMap[redemption_dialog.OptionKey] = snapshot.JSON
	common.OptionMapRWMutex.Unlock()
	redemptionDialogSnapshot.Store(snapshot)
}
