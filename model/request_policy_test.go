package model

import (
	"errors"
	"maps"
	"math"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/error_mapping"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestRequestPolicyDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN not configured")
				}
				driver = postgres.Open(dsn)
			}
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "policy_test_"}})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			previousDB, previousType, previousSnapshot := DB, common.MainDatabaseType(), CurrentRequestPolicy()
			common.OptionMapRWMutex.Lock()
			previousOptions := maps.Clone(common.OptionMap)
			common.OptionMap = maps.Clone(previousSnapshot.Options)
			common.OptionMapRWMutex.Unlock()
			DB = db
			common.SetMainDatabaseType(common.DatabaseType(dialect))
			initCol()
			t.Cleanup(func() {
				require.NoError(t, db.Migrator().DropTable(&Option{}))
				for k, v := range previousSnapshot.Options {
					require.NoError(t, updateOptionMap(k, v))
				}
				requestPolicySnapshot.Store(previousSnapshot)
				common.OptionMapRWMutex.Lock()
				common.OptionMap = previousOptions
				common.OptionMapRWMutex.Unlock()
				DB = previousDB
				common.SetMainDatabaseType(previousType)
				initCol()
				require.NoError(t, sqlDB.Close())
			})
			require.NoError(t, db.AutoMigrate(&Option{}))
			var version string
			query := "SELECT version()"
			if dialect == "sqlite" {
				query = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(query).Scan(&version).Error)
			t.Logf("database version: %s", version)
			rules := `[{"name":"session","model_regex":[".*"],"key_sources":[{"type":"request_header","key":"X-Session"}],"ttl_seconds":0,"skip_retry_on_failure":false,"include_using_group":false,"param_override_template":{"temperature":0},"future_field":{"enabled":false}}]`
			require.NoError(t, UpdateRequestPolicyOptions(map[string]string{"RetryTimes": "2", "channel_affinity_setting.rules": rules}))
			assert.Equal(t, 2, CurrentRequestPolicy().RetryTimes)
			assert.Equal(t, 2, common.RetryTimes, "the runtime global follows the same write")
			require.NoError(t, UpdateOption("AutomaticRetryStatusCodes", "429,500-503"))
			assert.Equal(t, "429,500-503", CurrentRequestPolicy().Options["AutomaticRetryStatusCodes"])
			assert.Equal(t, "429,500-503", operation_setting.AutomaticRetryStatusCodesToString())
			assert.Error(t, UpdateRequestPolicyOptions(map[string]string{"request_policy_setting.mode": "off"}), "the removed mode switch is not a policy option")
			assert.False(t, CurrentRequestPolicy().Affinity.Rules[0].SkipRetryOnFailure)
			assert.Equal(t, rules, CurrentRequestPolicy().Options["channel_affinity_setting.rules"])
			loadOptionsFromDatabase()
			loadOptionsFromDatabase()
			assert.Equal(t, rules, CurrentRequestPolicy().Options["channel_affinity_setting.rules"], "legacy rule JSON survives reloads without dropping extension fields")
			require.NoError(t, UpdateRequestPolicyOptions(map[string]string{"channel_affinity_setting.session_mode": "strict"}))
			loadOptionsFromDatabase()
			loadOptionsFromDatabase()
			assert.Equal(t, "strict", CurrentRequestPolicy().Affinity.SessionMode)
			assert.Equal(t, rules, CurrentRequestPolicy().Options["channel_affinity_setting.rules"], "a global mode never rewrites rule fields")
			snapshot := CurrentRequestPolicy()
			assert.Error(t, UpdateRequestPolicyOptions(map[string]string{"channel_affinity_setting.session_mode": "unknown"}))
			assert.Same(t, snapshot, CurrentRequestPolicy())
			for _, value := range []string{"-1", "1.5", "bad", strconv.Itoa(math.MaxInt)} {
				assert.Error(t, UpdateRequestPolicyOptions(map[string]string{"RetryTimes": value}))
				assert.Same(t, snapshot, CurrentRequestPolicy())
			}
			assert.Error(t, UpdateRequestPolicyOptions(map[string]string{"channel_affinity_setting.rules": "[", "RetryTimes": "8"}))
			assert.Same(t, snapshot, CurrentRequestPolicy())
			var option Option
			require.NoError(t, db.Where(map[string]any{"key": "RetryTimes"}).First(&option).Error)
			assert.Equal(t, "2", option.Value)
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("policy_fail", func(tx *gorm.DB) {
				if tx.Statement.Table == "policy_test_options" {
					tx.AddError(errors.New("save rejected"))
				}
			}))
			assert.Error(t, UpdateRequestPolicyOptions(map[string]string{"RetryTimes": "8", "channel_affinity_setting.session_mode": "off"}))
			assert.Same(t, snapshot, CurrentRequestPolicy())
			require.NoError(t, db.Callback().Update().Remove("policy_fail"))
			option = Option{}
			require.NoError(t, db.Where(map[string]any{"key": "RetryTimes"}).First(&option).Error)
			assert.Equal(t, "2", option.Value)
			loadOptionsFromDatabase()
			assert.Equal(t, "strict", CurrentRequestPolicy().Affinity.SessionMode, "a failed save keeps the persisted global mode")
		})
	}
}

func TestErrorMessageMappingDatabaseMatrix(t *testing.T) {
	const configJSON = `{"enabled":true,"rules":[{"id":"reasoning-format","name":"思考模式格式错误","enabled":true,"keyword":"reasoning_content","case_sensitive":false,"replacement":"当前请求格式与模型不兼容，请调整后重试。"}]}`
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN not configured")
				}
				driver = postgres.Open(dsn)
			}
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "errmsg_test_"}})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)

			previousDB, previousType := DB, common.MainDatabaseType()
			previousSnapshot := CurrentErrorMessageMapping()
			common.OptionMapRWMutex.Lock()
			previousOptions := common.OptionMap
			common.OptionMap = map[string]string{}
			common.OptionMapRWMutex.Unlock()
			DB = db
			common.SetMainDatabaseType(common.DatabaseType(dialect))
			initCol()
			t.Cleanup(func() {
				_ = db.Migrator().DropTable(&Option{})
				errorMessageMappingSnapshot.Store(previousSnapshot)
				common.OptionMapRWMutex.Lock()
				common.OptionMap = previousOptions
				common.OptionMapRWMutex.Unlock()
				DB = previousDB
				common.SetMainDatabaseType(previousType)
				initCol()
				_ = sqlDB.Close()
			})
			require.NoError(t, db.AutoMigrate(&Option{}))

			var version string
			query := "SELECT version()"
			if dialect == "sqlite" {
				query = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(query).Scan(&version).Error)
			t.Logf("database version: %s", version)

			// A missing record publishes the disabled default instead of failing.
			require.NoError(t, loadErrorMessageMapping())
			require.False(t, CurrentErrorMessageMapping().Config.Enabled)
			require.Empty(t, CurrentErrorMessageMapping().Config.Rules)

			cfg, err := error_mapping.ParseConfig([]byte(configJSON))
			require.NoError(t, err)
			_, err = SaveErrorMessageMapping(cfg)
			require.NoError(t, err)
			normalized, err := error_mapping.Encode(cfg)
			require.NoError(t, err)

			var option Option
			require.NoError(t, db.Where(map[string]any{"key": error_mapping.OptionKey}).First(&option).Error)
			require.Equal(t, normalized, option.Value, "the stored value is the normalized JSON document")
			common.OptionMapRWMutex.RLock()
			require.Equal(t, normalized, common.OptionMap[error_mapping.OptionKey])
			common.OptionMapRWMutex.RUnlock()

			// Reloads converge on the stored rules.
			require.NoError(t, loadErrorMessageMapping())
			require.NoError(t, loadErrorMessageMapping())
			result := CurrentErrorMessageMapping().Matcher.Match("upstream says reasoning_content error")
			require.True(t, result.Matched)
			require.Equal(t, "当前请求格式与模型不兼容，请调整后重试。", result.Message)
			require.Equal(t, "reasoning-format", result.RuleID)

			// The public getter returns a copy: mutating it cannot change the
			// active rules or the shared matcher.
			exposed := CurrentErrorMessageMapping()
			exposed.Config.Enabled = false
			exposed.Config.Rules[0].Replacement = "TAMPERED"
			stillActive := CurrentErrorMessageMapping()
			require.True(t, stillActive.Config.Enabled)
			require.Equal(t, "当前请求格式与模型不兼容，请调整后重试。", stillActive.Config.Rules[0].Replacement)
			require.Equal(t, "当前请求格式与模型不兼容，请调整后重试。", stillActive.Matcher.Match("upstream says reasoning_content error").Message)

			// Re-saving the same document is idempotent.
			_, err = SaveErrorMessageMapping(cfg)
			require.NoError(t, err)

			// A failed database write keeps the previous snapshot and stored value.
			snapshot := loadErrorMessageMappingSnapshot()
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("errmsg_fail", func(tx *gorm.DB) {
				if tx.Statement.Table == "errmsg_test_options" {
					tx.AddError(errors.New("save rejected"))
				}
			}))
			disabled, err := error_mapping.ParseConfig([]byte(`{"enabled":false,"rules":[]}`))
			require.NoError(t, err)
			_, err = SaveErrorMessageMapping(disabled)
			assert.Error(t, err)
			assert.Same(t, snapshot, loadErrorMessageMappingSnapshot())
			require.NoError(t, db.Callback().Update().Remove("errmsg_fail"))
			option = Option{}
			require.NoError(t, db.Where(map[string]any{"key": error_mapping.OptionKey}).First(&option).Error)
			assert.Equal(t, normalized, option.Value)

			// An invalid persisted value must not replace the valid snapshot.
			require.NoError(t, db.Model(&Option{}).Where(map[string]any{"key": error_mapping.OptionKey}).Update("value", "{").Error)
			assert.Error(t, loadErrorMessageMapping())
			assert.Same(t, snapshot, loadErrorMessageMappingSnapshot())

			// The generic option path is inert for this key.
			require.NoError(t, updateOptionMap(error_mapping.OptionKey, `{"enabled":true,"rules":[]}`))
			assert.Same(t, snapshot, loadErrorMessageMappingSnapshot())

			// Bulk updates are rejected; the single-key path routes through the
			// validated save function.
			assert.Error(t, UpdateOptionsBulk(map[string]string{error_mapping.OptionKey: normalized}))
			require.NoError(t, UpdateOption(error_mapping.OptionKey, normalized))
			assert.NotSame(t, snapshot, loadErrorMessageMappingSnapshot())

			if dialect != "sqlite" {
				return
			}
			// Concurrent readers must always observe one complete snapshot while a
			// save swaps in the next one. Run with -race.
			first, err := error_mapping.ParseConfig([]byte(`{"enabled":true,"rules":[{"id":"first","enabled":true,"keyword":"first-keyword","case_sensitive":false,"replacement":"FIRST"}]}`))
			require.NoError(t, err)
			_, err = SaveErrorMessageMapping(first)
			require.NoError(t, err)
			second, err := error_mapping.ParseConfig([]byte(`{"enabled":true,"rules":[{"id":"second","enabled":true,"keyword":"second-keyword","case_sensitive":false,"replacement":"SECOND"}]}`))
			require.NoError(t, err)
			start := make(chan struct{})
			seen := make(chan string, 32)
			var wg sync.WaitGroup
			for range 16 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					seen <- CurrentErrorMessageMapping().Matcher.Match("first-keyword then second-keyword").Message
				}()
			}
			close(start)
			_, err = SaveErrorMessageMapping(second)
			require.NoError(t, err)
			wg.Wait()
			close(seen)
			for message := range seen {
				require.Contains(t, []string{"FIRST", "SECOND"}, message, "every reader sees a complete snapshot")
			}
		})
	}
}
