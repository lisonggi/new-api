package controller

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/redemption_dialog"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const redemptionDialogTestConfig = `{"enabled":true,"title":"兑换成功","content":"**好评**立即获得1元兑换码","close_button_text":"我知道了"}`

// A distinct valid document written directly to the options row to prove a
// reload reads the database instead of reusing the current snapshot.
const redemptionDialogReloadConfig = `{"enabled":true,"title":"第二版","content":"第二版正文","close_button_text":"好的"}`

func publishedRedemptionDialogJSON() string {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return common.OptionMap[redemption_dialog.OptionKey]
}

func setupRedemptionDialogTest(t *testing.T) {
	t.Helper()
	require.NoError(t, i18n.Init())
	modelManagementDB(t, "sqlite", "")
	require.NoError(t, model.DB.AutoMigrate(&model.Redemption{}, &model.Log{}))
	_, err := model.SaveRedemptionDialog(redemption_dialog.DefaultConfig())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = model.SaveRedemptionDialog(redemption_dialog.DefaultConfig())
	})
}

func TestRedemptionSuccessDialogConfigStrictness(t *testing.T) {
	invalid := []string{
		``,
		`true`,
		`{}`,
		`{"title":"t","content":"c","close_button_text":"b"}`,
		`{"enabled":null}`,
		`{"enabled":"yes"}`,
		`{"enabled":true,"extra":1}`,
		`{"enabled":true,"title":null}`,
		`{"enabled":true,"title":"t","content":"c","close_button_text":"b"} trailing`,
		`{"enabled":true,"title":"   ","content":"c","close_button_text":"b"}`,
		`{"enabled":true,"title":"t","content":"","close_button_text":"b"}`,
		`{"enabled":true,"title":"t","content":"c","close_button_text":"  "}`,
		`{"enabled":true,"title":"t","content":"c"}`,
	}
	for _, raw := range invalid {
		_, err := redemption_dialog.ParseConfig([]byte(raw))
		require.Error(t, err, "expected reject: %s", raw)
	}

	// Limits are rune-based and always apply, even when the dialog is disabled.
	_, err := redemption_dialog.ParseConfig([]byte(`{"enabled":false,"title":"` + strings.Repeat("好", redemption_dialog.MaxTitleLength+1) + `"}`))
	require.Error(t, err)
	_, err = redemption_dialog.ParseConfig([]byte(`{"enabled":false,"content":"` + strings.Repeat("a", redemption_dialog.MaxContentLength+1) + `"}`))
	require.Error(t, err)
	_, err = redemption_dialog.ParseConfig([]byte(`{"enabled":false,"close_button_text":"` + strings.Repeat("好", redemption_dialog.MaxCloseButtonLength+1) + `"}`))
	require.Error(t, err)

	// A disabled config may keep empty text, and enabled text at the exact
	// rune limit is accepted.
	disabled, err := redemption_dialog.ParseConfig([]byte(`{"enabled":false}`))
	require.NoError(t, err)
	require.Equal(t, redemption_dialog.DefaultConfig(), disabled)

	atLimit, err := redemption_dialog.ParseConfig([]byte(`{"enabled":true,"title":"` + strings.Repeat("好", redemption_dialog.MaxTitleLength) + `","content":"` + strings.Repeat("好", redemption_dialog.MaxContentLength) + `","close_button_text":"` + strings.Repeat("好", redemption_dialog.MaxCloseButtonLength) + `"}`))
	require.NoError(t, err)
	require.Len(t, []rune(atLimit.Title), redemption_dialog.MaxTitleLength)
}

func TestRedemptionSuccessDialogSettingsEndpoints(t *testing.T) {
	setupRedemptionDialogTest(t)

	getRecorder := httptest.NewRecorder()
	getCtx, _ := gin.CreateTestContext(getRecorder)
	GetRedemptionDialog(getCtx)
	require.Equal(t, http.StatusOK, getRecorder.Code)
	var getBody struct {
		Success bool                     `json:"success"`
		Data    redemption_dialog.Config `json:"data"`
	}
	require.NoError(t, common.Unmarshal(getRecorder.Body.Bytes(), &getBody))
	require.True(t, getBody.Success)
	require.False(t, getBody.Data.Enabled)

	// An invalid document is rejected and leaves the previous value active.
	putInvalidRecorder := httptest.NewRecorder()
	putInvalidCtx, _ := gin.CreateTestContext(putInvalidRecorder)
	putInvalidCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option/redemption_success_dialog", strings.NewReader(`{"enabled":true,"title":"t","content":"c","close_button_text":"b","extra":1}`))
	UpdateRedemptionDialog(putInvalidCtx)
	require.Equal(t, http.StatusBadRequest, putInvalidRecorder.Code)
	require.False(t, model.CurrentRedemptionDialog().Config.Enabled)

	putRecorder := httptest.NewRecorder()
	putCtx, _ := gin.CreateTestContext(putRecorder)
	putCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option/redemption_success_dialog", strings.NewReader(redemptionDialogTestConfig))
	UpdateRedemptionDialog(putCtx)
	require.Equal(t, http.StatusOK, putRecorder.Code)
	require.JSONEq(t, `{"success":true,"message":"","data":{"enabled":true,"title":"兑换成功","content":"**好评**立即获得1元兑换码","close_button_text":"我知道了"}}`, putRecorder.Body.String())
	require.True(t, model.CurrentRedemptionDialog().Config.Enabled)

	// Validation failures are translated from the stable error code.
	localizedRecorder := httptest.NewRecorder()
	localizedCtx, _ := gin.CreateTestContext(localizedRecorder)
	localizedCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option/redemption_success_dialog", strings.NewReader(`{"enabled":true,"title":"t","content":"c"}`))
	localizedCtx.Request.Header.Set("Accept-Language", "zh-CN")
	UpdateRedemptionDialog(localizedCtx)
	require.Equal(t, http.StatusBadRequest, localizedRecorder.Code)
	assert.Contains(t, localizedRecorder.Body.String(), "close_button_text")

	oversizedRecorder := httptest.NewRecorder()
	oversizedCtx, _ := gin.CreateTestContext(oversizedRecorder)
	oversizedCtx.Request = httptest.NewRequest(http.MethodPut, "/api/option/redemption_success_dialog", strings.NewReader(`{"enabled":true,"title":"`+strings.Repeat("a", redemption_dialog.MaxConfigBodyBytes)+`","content":"c","close_button_text":"b"}`))
	UpdateRedemptionDialog(oversizedCtx)
	require.Equal(t, http.StatusRequestEntityTooLarge, oversizedRecorder.Code)

	// The generic option writers must not be able to bypass the dedicated path.
	require.Error(t, model.UpdateOption(redemption_dialog.OptionKey, `{"enabled":true}`))
	require.Error(t, model.UpdateOptionsBulk(map[string]string{redemption_dialog.OptionKey: redemptionDialogTestConfig}))
}

// Reading a config must not let a caller change future redemption responses
// without a validated, successful save.
func TestRedemptionSuccessDialogReadCannotMutatePublishedConfig(t *testing.T) {
	setupRedemptionDialogTest(t)
	cfg, err := redemption_dialog.ParseConfig([]byte(redemptionDialogTestConfig))
	require.NoError(t, err)
	_, err = model.SaveRedemptionDialog(cfg)
	require.NoError(t, err)
	read := model.CurrentRedemptionDialog()
	read.Config.Content = "UNSAVED CONTENT"
	read.Config.Enabled = false
	assert.Equal(t, cfg, model.CurrentRedemptionDialog().Config,
		"mutating a read result must not publish an unsaved config")
	get := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(get)
	GetRedemptionDialog(ctx)
	assert.NotContains(t, get.Body.String(), "UNSAVED CONTENT")
}

// enableRedemptionDialogPayments satisfies the compliance gate that TopUp
// checks before redeeming, restoring the previous state afterwards.
func enableRedemptionDialogPayments(t *testing.T) {
	t.Helper()
	paymentSetting := operation_setting.GetPaymentSetting()
	previousConfirmed, previousVersion := paymentSetting.ComplianceConfirmed, paymentSetting.ComplianceTermsVersion
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	t.Cleanup(func() {
		paymentSetting.ComplianceConfirmed = previousConfirmed
		paymentSetting.ComplianceTermsVersion = previousVersion
	})
}

func newRedemptionDialogUser(t *testing.T, keys []string) *model.User {
	t.Helper()
	user := &model.User{Username: "redemption-dialog-user", Password: "password", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(user).Error)
	for index, key := range keys {
		require.NoError(t, model.DB.Create(&model.Redemption{
			Name:        "redemption-dialog",
			Key:         key,
			Status:      common.RedemptionCodeStatusEnabled,
			Quota:       500 + index,
			CreatedTime: common.GetTimestamp(),
		}).Error)
	}
	return user
}

func TestTopUpAttachesRedemptionSuccessDialog(t *testing.T) {
	setupRedemptionDialogTest(t)
	enableRedemptionDialogPayments(t)

	keys := []string{
		"20000000000000000000000000000001",
		"20000000000000000000000000000002",
	}
	user := newRedemptionDialogUser(t, keys)

	// Enabled: the optional dialog is attached and the amount stays a number.
	cfg, err := redemption_dialog.ParseConfig([]byte(redemptionDialogTestConfig))
	require.NoError(t, err)
	_, err = model.SaveRedemptionDialog(cfg)
	require.NoError(t, err)

	enabled := redeemViaTopUp(t, user.Id, keys[0])
	require.Equal(t, http.StatusOK, enabled.Code)
	require.JSONEq(t, `{"success":true,"message":"","data":500,"success_dialog":{"title":"兑换成功","content":"**好评**立即获得1元兑换码","close_button_text":"我知道了"}}`, enabled.Body.String())

	// Disabled: the legacy response shape is preserved with no dialog field.
	_, err = model.SaveRedemptionDialog(redemption_dialog.DefaultConfig())
	require.NoError(t, err)

	disabled := redeemViaTopUp(t, user.Id, keys[1])
	require.Equal(t, http.StatusOK, disabled.Code)
	require.JSONEq(t, `{"success":true,"message":"","data":501}`, disabled.Body.String())
	require.NotContains(t, disabled.Body.String(), "success_dialog")
}

func redeemViaTopUp(t *testing.T, userID int, key string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", userID)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/topup", strings.NewReader(`{"key":"`+key+`"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	TopUp(ctx)
	return recorder
}

func putRedemptionDialog(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/option/redemption_success_dialog", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	UpdateRedemptionDialog(ctx)
	return recorder
}

// execOptionDDL runs trigger DDL through the underlying database/sql pool.
// MySQL rejects CREATE/DROP TRIGGER on GORM's prepared statement protocol
// (error 1295), so the GORM layer must be bypassed for these statements.
func execOptionDDL(statement string) error {
	sqlDB, err := model.DB.DB()
	if err != nil {
		return err
	}
	_, err = sqlDB.Exec(statement)
	return err
}

// rejectOptionUpdate installs a database-side trigger that rejects UPDATEs to
// the redemption dialog option row. SELECTs still succeed, so the failure is
// raised by the real engine while reading and creating the row still work.
func rejectOptionUpdate(t *testing.T, kind string) {
	t.Helper()
	// Restore first so even a mid-install assertion failure cleans up whatever
	// was already created in this isolated database.
	t.Cleanup(func() { dropOptionUpdateRejection(t, kind) })
	for _, statement := range optionUpdateRejectionStatements(kind) {
		require.NoError(t, execOptionDDL(statement), "install %s rejection trigger", kind)
	}
}

func dropOptionUpdateRejection(t *testing.T, kind string) {
	t.Helper()
	for _, statement := range optionUpdateRejectionDropStatements(kind) {
		_ = execOptionDDL(statement)
	}
}

func optionUpdateRejectionStatements(kind string) []string {
	switch kind {
	case "mysql":
		return []string{
			"CREATE TRIGGER reject_redemption_dialog_update BEFORE UPDATE ON options FOR EACH ROW BEGIN IF NEW.`key` = 'RedemptionSuccessDialog' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'forced option update failure'; END IF; END",
		}
	case "postgres":
		return []string{
			`CREATE OR REPLACE FUNCTION reject_redemption_dialog_update() RETURNS trigger AS $$ BEGIN IF NEW."key" = 'RedemptionSuccessDialog' THEN RAISE EXCEPTION 'forced option update failure'; END IF; RETURN NEW; END; $$ LANGUAGE plpgsql`,
			"CREATE TRIGGER reject_redemption_dialog_update BEFORE UPDATE ON options FOR EACH ROW EXECUTE FUNCTION reject_redemption_dialog_update()",
		}
	default:
		return []string{
			"CREATE TRIGGER reject_redemption_dialog_update BEFORE UPDATE ON options WHEN NEW.key = 'RedemptionSuccessDialog' BEGIN SELECT RAISE(ABORT, 'forced option update failure'); END",
		}
	}
}

func optionUpdateRejectionDropStatements(kind string) []string {
	if kind == "postgres" {
		return []string{
			"DROP TRIGGER IF EXISTS reject_redemption_dialog_update ON options",
			"DROP FUNCTION IF EXISTS reject_redemption_dialog_update()",
		}
	}
	return []string{"DROP TRIGGER IF EXISTS reject_redemption_dialog_update"}
}

// The dialog is stored as a single JSON option. This matrix proves the shared
// save/read path round-trips the same document on every supported database.
func TestRedemptionSuccessDialogPersistenceAcrossDatabases(t *testing.T) {
	dialects := []struct{ kind, env string }{
		{"sqlite", ""},
		{"mysql", "TEST_MYSQL_DSN"},
		{"postgres", "TEST_POSTGRES_DSN"},
	}
	for _, dialect := range dialects {
		t.Run(dialect.kind, func(t *testing.T) {
			dsn := ""
			if dialect.env != "" {
				dsn = strings.TrimSpace(os.Getenv(dialect.env))
				if dsn == "" {
					t.Skipf("%s is not configured", dialect.env)
				}
			}
			require.NoError(t, i18n.Init())
			modelManagementDB(t, dialect.kind, dsn)

			cfg, err := redemption_dialog.ParseConfig([]byte(redemptionDialogTestConfig))
			require.NoError(t, err)
			require.NoError(t, model.UpdateOption(redemption_dialog.OptionKey, redemptionDialogTestConfig))
			require.Equal(t, cfg, model.CurrentRedemptionDialog().Config)

			var stored model.Option
			require.NoError(t, model.DB.Where(&model.Option{Key: redemption_dialog.OptionKey}).First(&stored).Error)
			persisted, err := redemption_dialog.ParseConfig([]byte(stored.Value))
			require.NoError(t, err)
			require.Equal(t, cfg, persisted)

			// A reload must read the database: a different valid row written
			// directly (as another instance would) replaces the snapshot.
			reloaded, err := redemption_dialog.ParseConfig([]byte(redemptionDialogReloadConfig))
			require.NoError(t, err)
			require.NoError(t, model.DB.Model(&model.Option{}).Where(&model.Option{Key: redemption_dialog.OptionKey}).Update("value", redemptionDialogReloadConfig).Error)
			model.InitOptionMap()
			require.Equal(t, reloaded, model.CurrentRedemptionDialog().Config)
			require.Equal(t, redemptionDialogReloadConfig, publishedRedemptionDialogJSON())

			// A database-side trigger that rejects the UPDATE must produce a real
			// engine error, surface as an HTTP failure and leave the database,
			// snapshot and option map on the last committed value.
			rejectOptionUpdate(t, dialect.kind)
			_, directErr := model.SaveRedemptionDialog(redemption_dialog.DefaultConfig())
			require.ErrorContains(t, directErr, "forced option update failure", "the database must reject the UPDATE")
			failedSave := putRedemptionDialog(t, `{"enabled":false,"title":"","content":"","close_button_text":""}`)
			require.Equal(t, http.StatusInternalServerError, failedSave.Code)
			require.Contains(t, failedSave.Body.String(), `"success":false`)
			require.Equal(t, reloaded, model.CurrentRedemptionDialog().Config)
			require.Equal(t, redemptionDialogReloadConfig, publishedRedemptionDialogJSON())
			dropOptionUpdateRejection(t, dialect.kind)
			var afterUpdateFailure model.Option
			require.NoError(t, model.DB.Where(&model.Option{Key: redemption_dialog.OptionKey}).First(&afterUpdateFailure).Error)
			require.Equal(t, redemptionDialogReloadConfig, afterUpdateFailure.Value)

			// Removing the trigger restores saving.
			recoveredSave := putRedemptionDialog(t, redemptionDialogTestConfig)
			require.Equal(t, http.StatusOK, recoveredSave.Code)
			require.Equal(t, cfg, model.CurrentRedemptionDialog().Config)

			// A table-level read failure also keeps the last valid value.
			require.NoError(t, model.DB.Migrator().RenameTable(&model.Option{}, "options_reject_save"))
			t.Cleanup(func() {
				_ = model.DB.Migrator().RenameTable("options_reject_save", &model.Option{}).Error
			})
			_, err = model.SaveRedemptionDialog(redemption_dialog.DefaultConfig())
			require.Error(t, err)
			require.Equal(t, cfg, model.CurrentRedemptionDialog().Config)
			require.Equal(t, redemptionDialogTestConfig, publishedRedemptionDialogJSON())
			require.NoError(t, model.DB.Migrator().RenameTable("options_reject_save", &model.Option{}))

			// A corrupt stored value keeps the last valid published config.
			require.NoError(t, model.DB.Model(&model.Option{}).Where(&model.Option{Key: redemption_dialog.OptionKey}).Update("value", "{not-json").Error)
			model.InitOptionMap()
			require.Equal(t, cfg, model.CurrentRedemptionDialog().Config)

			// A missing record reloads the disabled default.
			require.NoError(t, model.DB.Where(&model.Option{Key: redemption_dialog.OptionKey}).Delete(&model.Option{}).Error)
			model.InitOptionMap()
			require.Equal(t, redemption_dialog.DefaultConfig(), model.CurrentRedemptionDialog().Config)

			// Bulk writes must not bypass the dedicated validation path.
			require.Error(t, model.UpdateOptionsBulk(map[string]string{redemption_dialog.OptionKey: `{"enabled":true}`}))
		})
	}
}

// Invalid and already-consumed codes must never carry a success dialog, and the
// quota must be credited exactly once.
func TestTopUpFailureNeverIncludesSuccessDialog(t *testing.T) {
	setupRedemptionDialogTest(t)
	enableRedemptionDialogPayments(t)

	key := "30000000000000000000000000000001"
	user := newRedemptionDialogUser(t, []string{key})

	cfg, err := redemption_dialog.ParseConfig([]byte(redemptionDialogTestConfig))
	require.NoError(t, err)
	_, err = model.SaveRedemptionDialog(cfg)
	require.NoError(t, err)

	invalid := redeemViaTopUp(t, user.Id, "30000000000000000000000000000009")
	require.NotContains(t, invalid.Body.String(), "success_dialog")
	require.NotContains(t, invalid.Body.String(), `"success":true`)

	first := redeemViaTopUp(t, user.Id, key)
	require.Equal(t, http.StatusOK, first.Code)
	require.Contains(t, first.Body.String(), "success_dialog")

	duplicate := redeemViaTopUp(t, user.Id, key)
	require.NotContains(t, duplicate.Body.String(), "success_dialog")
	require.NotContains(t, duplicate.Body.String(), `"success":true`)

	var refreshed model.User
	require.NoError(t, model.DB.First(&refreshed, "id = ?", user.Id).Error)
	require.Equal(t, 500, refreshed.Quota, "quota must be credited exactly once")
}

// An expired code must fail with no dialog and no quota change while the dialog
// is enabled.
func TestTopUpExpiredCodeNeverIncludesSuccessDialog(t *testing.T) {
	setupRedemptionDialogTest(t)
	enableRedemptionDialogPayments(t)

	expiredKey := "40000000000000000000000000000001"
	user := &model.User{Username: "redemption-expired-user", Password: "password", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(user).Error)
	require.NoError(t, model.DB.Create(&model.Redemption{
		Name:        "expired",
		Key:         expiredKey,
		Status:      common.RedemptionCodeStatusEnabled,
		Quota:       500,
		CreatedTime: common.GetTimestamp(),
		ExpiredTime: common.GetTimestamp() - 60,
	}).Error)

	cfg, err := redemption_dialog.ParseConfig([]byte(redemptionDialogTestConfig))
	require.NoError(t, err)
	_, err = model.SaveRedemptionDialog(cfg)
	require.NoError(t, err)

	recorder := redeemViaTopUp(t, user.Id, expiredKey)
	require.NotContains(t, recorder.Body.String(), "success_dialog")
	require.NotContains(t, recorder.Body.String(), `"success":true`)

	var refreshed model.User
	require.NoError(t, model.DB.First(&refreshed, "id = ?", user.Id).Error)
	require.Equal(t, 0, refreshed.Quota, "an expired code must not credit quota")
}
