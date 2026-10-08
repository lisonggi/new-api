package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	filterdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestChannelValidateSettingsRejectsInvalidHTTPTransport(t *testing.T) {
	tests := []struct {
		name    string
		setting dto.ChannelSettings
		wantErr string
	}{
		{
			name:    "auto with shards is valid",
			setting: dto.ChannelSettings{HTTPProtocol: "auto", HTTP2ConnectionShards: 4},
		},
		{
			name:    "http1 with shards greater than one rejected",
			setting: dto.ChannelSettings{HTTPProtocol: "http1", HTTP2ConnectionShards: 2},
			wantErr: "http2_connection_shards",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &Channel{}
			channel.SetSetting(tt.setting)
			err := channel.ValidateSettings()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestChannelGetSettingIsolatesInvalidErrorRetryPolicy(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantProxy   string
		forceFormat bool
	}{
		{
			name:        "policy is a string",
			raw:         `{"proxy":"http://127.0.0.1:8080","force_format":true,"error_retry_policy":"not-an-object"}`,
			wantProxy:   "http://127.0.0.1:8080",
			forceFormat: true,
		},
		{
			name:        "policy uses a case alias",
			raw:         `{"proxy":"http://127.0.0.1:8080","force_format":true,"ERROR_RETRY_POLICY":{"enabled":true}}`,
			wantProxy:   "http://127.0.0.1:8080",
			forceFormat: true,
		},
		{
			name:        "policy has duplicate keys",
			raw:         `{"proxy":"http://127.0.0.1:8080","force_format":true,"error_retry_policy":{"enabled":true,"enabled":false}}`,
			wantProxy:   "http://127.0.0.1:8080",
			forceFormat: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := tt.raw
			channel := &Channel{Setting: &raw}
			setting := channel.GetSetting()
			assert.Equal(t, tt.wantProxy, setting.Proxy)
			assert.Equal(t, tt.forceFormat, setting.ForceFormat)
			assert.Nil(t, setting.ErrorRetryPolicy)
			assert.NotEmpty(t, setting.ErrorRetryPolicyDiagnostic)
			require.NotNil(t, channel.Setting, "an invalid policy must not clear unrelated legacy settings")
			assert.Contains(t, *channel.Setting, "proxy")
		})
	}
}

func TestChannelGetSettingParsesValidErrorRetryPolicy(t *testing.T) {
	raw := `{"proxy":"http://127.0.0.1:8080","error_retry_policy":{"enabled":true,"rules":[{"id":"r1","enabled":true,"action":"retry","status_codes":[429]}]}}`
	channel := &Channel{Setting: &raw}
	setting := channel.GetSetting()
	require.NotNil(t, setting.ErrorRetryPolicy)
	assert.True(t, setting.ErrorRetryPolicy.Enabled)
	require.Len(t, setting.ErrorRetryPolicy.Rules, 1)
	assert.Empty(t, setting.ErrorRetryPolicyDiagnostic)
}

func TestChannelSetSettingPreservesInvalidStoredErrorRetryPolicy(t *testing.T) {
	raw := `{"task_plugin_key":"old","error_retry_policy":"not-an-object"}`
	channel := &Channel{Setting: &raw}
	setting := channel.GetSetting()
	require.Nil(t, setting.ErrorRetryPolicy)

	setting.TaskPluginKey = "new"
	channel.SetSetting(setting)

	require.NotNil(t, channel.Setting)
	assert.Contains(t, *channel.Setting, `"task_plugin_key":"new"`)
	assert.Contains(t, *channel.Setting, "not-an-object", "an invalid policy must survive an internal write-back")
}

func oversizedRetryPolicySetting(ruleCount, condCount int) string {
	value := strings.Repeat("a", 512)
	var builder strings.Builder
	builder.WriteString(`{"error_retry_policy":{"enabled":true,"rules":[`)
	for i := range ruleCount {
		if i > 0 {
			builder.WriteString(",")
		}
		fmt.Fprintf(&builder, `{"id":"r%d","enabled":true,"action":"retry","conditions":[`, i)
		for j := range condCount {
			if j > 0 {
				builder.WriteString(",")
			}
			fmt.Fprintf(&builder, `{"field":"message","operator":"equals","value":"%s"}`, value)
		}
		builder.WriteString(`]}`)
	}
	builder.WriteString(`]}}`)
	return builder.String()
}

func TestChannelValidateSettingsRejectsOversizedErrorRetryPolicy(t *testing.T) {
	raw := oversizedRetryPolicySetting(16, 4)
	require.Greater(t, len(raw), dto.MaxChannelErrorRetryPolicyBytes)
	require.LessOrEqual(t, len(raw), dto.MaxChannelSettingBytes)

	channel := &Channel{Setting: &raw}
	err := channel.ValidateSettings()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error_retry_policy exceeds")
}

func TestChannelValidateSettingsRejectsOversizedSetting(t *testing.T) {
	raw := oversizedRetryPolicySetting(32, 8)
	require.Greater(t, len(raw), dto.MaxChannelSettingBytes)

	channel := &Channel{Setting: &raw}
	err := channel.ValidateSettings()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

func TestChannelValidateSettingsAcceptsValidErrorRetryPolicy(t *testing.T) {
	raw := `{"error_retry_policy":{"enabled":true,"rules":[{"id":"r1","enabled":true,"action":"retry","status_codes":[429],"conditions":[{"field":"message","operator":"contains","value":"overloaded"}]}]}}`
	channel := &Channel{Setting: &raw}
	require.NoError(t, channel.ValidateSettings())
}

func TestChannelValidateSettingsAcceptsLegacyOversizedSettingWithoutPolicy(t *testing.T) {
	raw := `{"system_prompt":"` + strings.Repeat("a", dto.MaxChannelSettingBytes) + `"}`
	require.Greater(t, len(raw), dto.MaxChannelSettingBytes)

	channel := &Channel{Setting: &raw}
	require.NoError(t, channel.ValidateSettings(), "the setting budget only applies when error_retry_policy is present")
}

func TestChannelSetSettingRefusesAmbiguousErrorRetryPolicyWriteBack(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "case alias",
			raw:  `{"task_plugin_key":"old","ERROR_RETRY_POLICY":{"enabled":true}}`,
		},
		{
			name: "duplicate exact key",
			raw:  `{"task_plugin_key":"old","error_retry_policy":{"enabled":true},"error_retry_policy":{"enabled":false}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := tt.raw
			channel := &Channel{Setting: &raw}
			setting := channel.GetSetting()
			require.Nil(t, setting.ErrorRetryPolicy)
			require.NotEmpty(t, setting.ErrorRetryPolicyDiagnostic)

			setting.TaskPluginKey = "new"
			channel.SetSetting(setting)

			require.NotNil(t, channel.Setting)
			assert.Equal(t, raw, *channel.Setting, "an ambiguous policy must not be silently rewritten into a valid one")
		})
	}
}

func TestAdvancedCustomChannelRequiresModelListRouteOnlyWhenUpdateChecksEnabled(t *testing.T) {
	inferenceRoute := dto.AdvancedCustomRoute{
		IncomingPath: "/v1/chat/completions",
		UpstreamPath: "/v1/chat/completions",
		Converter:    "none",
	}

	tests := []struct {
		name          string
		checksEnabled bool
		routes        []dto.AdvancedCustomRoute
		wantErr       string
	}{
		{
			name:   "legacy channel without discovery route remains valid",
			routes: []dto.AdvancedCustomRoute{inferenceRoute},
		},
		{
			name:          "enabled checks require discovery route",
			checksEnabled: true,
			routes:        []dto.AdvancedCustomRoute{inferenceRoute},
			wantErr:       dto.AdvancedCustomModelListPath,
		},
		{
			name:          "enabled checks accept discovery route",
			checksEnabled: true,
			routes: []dto.AdvancedCustomRoute{
				inferenceRoute,
				{
					IncomingPath: dto.AdvancedCustomModelListPath,
					UpstreamPath: dto.AdvancedCustomModelListPath,
					Converter:    "none",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &Channel{Type: constant.ChannelTypeAdvancedCustom}
			channel.SetOtherSettings(dto.ChannelOtherSettings{
				UpstreamModelUpdateCheckEnabled: tt.checksEnabled,
				AdvancedCustom: &dto.AdvancedCustomConfig{
					Routes: tt.routes,
				},
			})

			err := channel.ValidateSettings()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestInferencePresetSettingsAndDatabaseRoundTrip(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(filepath.Join(t.TempDir(), "presets.db"))
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				driver = postgres.Open(dsn)
			}
			db, err := gorm.Open(driver, &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			table := db.Table("inference_preset_channels").Session(&gorm.Session{})
			require.NoError(t, table.AutoMigrate(&Channel{}))
			t.Cleanup(func() { require.NoError(t, db.Migrator().DropTable("inference_preset_channels")) })
			var version string
			if dialect == "sqlite" {
				require.NoError(t, db.Raw("select sqlite_version()").Scan(&version).Error)
			} else {
				require.NoError(t, db.Raw("select version()").Scan(&version).Error)
			}
			t.Logf("%s version: %s", dialect, version)
			for _, channelType := range []int{constant.ChannelTypeVLLM, constant.ChannelTypeSGLang} {
				t.Run(fmt.Sprint(channelType), func(t *testing.T) {
					channel := &Channel{Type: channelType, Key: "EMPTY", Name: "inference", Status: common.ChannelStatusEnabled}
					require.NoError(t, channel.ValidateSettings())
					require.NotNil(t, channel.GetOtherSettings().AdvancedCustom)
					require.NoError(t, table.Create(channel).Error)
					for range 2 {
						var loaded Channel
						require.NoError(t, table.First(&loaded, channel.Id).Error)
						assert.Equal(t, channelType, loaded.Type)
						assert.Empty(t, loaded.OtherSettings)
						defaults := loaded.GetOtherSettings().AdvancedCustom
						require.NotNil(t, defaults)
						assert.True(t, defaults.SupportsPath("/v1/messages"))
						assert.Empty(t, loaded.OtherSettings, "reading defaults must not rewrite saved settings")
						defaults.Routes[0].UpstreamPath = "/changed-locally"
						assert.Equal(t, "/v1/chat/completions", loaded.GetOtherSettings().AdvancedCustom.Routes[0].UpstreamPath)
					}
					settings := dto.ChannelOtherSettings{AdvancedCustom: &dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{{IncomingPath: "/v1/chat/completions", UpstreamPath: "/custom/chat", Models: []string{"allowed"}, Auth: &dto.AdvancedCustomRouteAuth{Type: "none"}}}}}
					channel.SetOtherSettings(settings)
					require.NoError(t, channel.ValidateSettings())
					require.NoError(t, table.Save(channel).Error)
					for range 2 {
						var loaded Channel
						require.NoError(t, table.First(&loaded, channel.Id).Error)
						actual := loaded.GetOtherSettings().AdvancedCustom
						require.Equal(t, common.GetAdvancedCustomPreset(channelType), actual, "named channels must ignore editable advanced_custom overrides")
						assert.Equal(t, channel.OtherSettings, loaded.OtherSettings)
						for _, tc := range []struct {
							path, model string
							allowed     bool
						}{
							{"/v1/chat/completions", "allowed", true},
							{"/v1/chat/completions", "other", true},
							{"/v1/messages", "allowed", true},
							{"/v1/images/generations", "allowed", false},
						} {
							ok, _ := ChannelSatisfiesFilters(&loaded, tc.model, []filterdto.ChannelFilter{{Kind: filterdto.FilterRequestPath, RequestPath: tc.path}})
							assert.Equal(t, tc.allowed, ok)
						}
						endpoints := getPricingEndpointTypesForAbility(AbilityWithChannel{ChannelType: channelType, Ability: Ability{Model: "allowed", ChannelId: loaded.Id}}, map[int]*dto.AdvancedCustomConfig{loaded.Id: actual})
						expectedEndpoints := []constant.EndpointType{constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAIResponse, constant.EndpointTypeAnthropic, constant.EndpointTypeEmbeddings}
						if channelType == constant.ChannelTypeSGLang {
							expectedEndpoints = append(expectedEndpoints, constant.EndpointTypeJinaRerank)
						}
						assert.ElementsMatch(t, expectedEndpoints, endpoints)
					}
				})
			}
		})
	}
}

// TestChannelErrorRetryPolicyDatabaseRoundTrip verifies the new policy is
// stored and read back unchanged on every supported database, that an invalid
// stored policy is isolated and preserved across an internal write-back, and
// that an over-limit policy is rejected before any insert is attempted.
func TestChannelErrorRetryPolicyDatabaseRoundTrip(t *testing.T) {
	policy := &dto.ChannelErrorRetryPolicy{
		Enabled: true,
		Rules: []dto.ChannelErrorRetryRule{
			{
				ID:          "client-parameter",
				Name:        "参数不支持",
				Enabled:     true,
				Action:      dto.ChannelErrorRetryActionStop,
				StatusCodes: []int{500},
				Conditions: []dto.ChannelErrorRetryCondition{
					{Field: dto.ChannelErrorRetryFieldMessage, Operator: dto.ChannelErrorRetryOperatorContains, Value: "unsupported \"parameter\" \\ 中文", CaseSensitive: false},
				},
			},
			{
				ID:          "supplier-balance",
				Enabled:     true,
				Action:      dto.ChannelErrorRetryActionRetry,
				StatusCodes: []int{402},
				Conditions: []dto.ChannelErrorRetryCondition{
					{Field: dto.ChannelErrorRetryFieldCode, Operator: dto.ChannelErrorRetryOperatorEquals, Value: "insufficient_balance", CaseSensitive: true},
				},
			},
		},
	}

	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(filepath.Join(t.TempDir(), "retry-policy.db"))
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				driver = postgres.Open(dsn)
			}
			db, err := gorm.Open(driver, &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			table := db.Table("channel_error_retry_policy_channels").Session(&gorm.Session{})
			require.NoError(t, table.AutoMigrate(&Channel{}))
			t.Cleanup(func() { require.NoError(t, db.Migrator().DropTable("channel_error_retry_policy_channels")) })
			var version string
			if dialect == "sqlite" {
				require.NoError(t, db.Raw("select sqlite_version()").Scan(&version).Error)
			} else {
				require.NoError(t, db.Raw("select version()").Scan(&version).Error)
			}
			t.Logf("%s version: %s", dialect, version)

			channel := &Channel{Name: "retry-policy", Key: "EMPTY", Status: common.ChannelStatusEnabled, Type: 1}
			channel.SetSetting(dto.ChannelSettings{ErrorRetryPolicy: policy, Proxy: "http://127.0.0.1:8080", ForceFormat: true})
			require.NoError(t, channel.ValidateSettings())
			require.NoError(t, table.Create(channel).Error)

			var loaded Channel
			require.NoError(t, table.First(&loaded, channel.Id).Error)
			setting := loaded.GetSetting()
			require.NotNil(t, setting.ErrorRetryPolicy)
			assert.Equal(t, policy, setting.ErrorRetryPolicy)
			assert.Equal(t, "http://127.0.0.1:8080", setting.Proxy)
			assert.True(t, setting.ForceFormat)

			setting.ErrorRetryPolicy.Rules[0].Action = dto.ChannelErrorRetryActionRetry
			setting.ErrorRetryPolicy.Rules[0].Name = "改了名字"
			loaded.SetSetting(setting)
			require.NoError(t, loaded.ValidateSettings())
			require.NoError(t, table.Save(&loaded).Error)
			var reloaded Channel
			require.NoError(t, table.First(&reloaded, channel.Id).Error)
			assert.Equal(t, "改了名字", reloaded.GetSetting().ErrorRetryPolicy.Rules[0].Name)

			copied := &Channel{Name: "retry-policy-copy", Key: "EMPTY", Status: common.ChannelStatusEnabled, Type: 1}
			copied.SetSetting(reloaded.GetSetting())
			require.NoError(t, copied.ValidateSettings())
			require.NoError(t, table.Create(copied).Error)
			var loadCopy Channel
			require.NoError(t, table.First(&loadCopy, copied.Id).Error)
			assert.Equal(t, reloaded.Setting, loadCopy.Setting)

			// A case-variant legacy key must not break the legacy decode and must
			// keep the encoding/json overwrite order (last key wins) without the
			// raw bytes changing.
			precedenceRaw := `{"proxy":"http://first:1","Proxy":"http://second:2"}`
			precedence := &Channel{Name: "retry-precedence", Key: "EMPTY", Status: common.ChannelStatusEnabled, Type: 1, Setting: &precedenceRaw}
			require.NoError(t, table.Create(precedence).Error)
			var loadPrecedence Channel
			require.NoError(t, table.First(&loadPrecedence, precedence.Id).Error)
			assert.Equal(t, "http://second:2", loadPrecedence.GetSetting().Proxy)
			assert.Equal(t, precedenceRaw, *loadPrecedence.Setting)

			// Invalid stored policy: isolated at runtime, never silently dropped.
			rawInvalid := `{"task_plugin_key":"old","error_retry_policy":{"enabled":true,"rules":[{"id":"r1","enabled":true,"action":"retry"}]}}`
			bad := &Channel{Name: "retry-bad", Key: "EMPTY", Status: common.ChannelStatusEnabled, Type: 1, Setting: &rawInvalid}
			require.NoError(t, table.Create(bad).Error)
			var loadBad Channel
			require.NoError(t, table.First(&loadBad, bad.Id).Error)
			badSetting := loadBad.GetSetting()
			assert.Nil(t, badSetting.ErrorRetryPolicy)
			assert.NotEmpty(t, badSetting.ErrorRetryPolicyDiagnostic)
			require.NotNil(t, loadBad.Setting)
			assert.Contains(t, *loadBad.Setting, "error_retry_policy")
			badSetting.TaskPluginKey = "kept"
			require.NoError(t, loadBad.SetSetting(badSetting))
			require.NoError(t, table.Save(&loadBad).Error)
			var loadBad2 Channel
			require.NoError(t, table.First(&loadBad2, bad.Id).Error)
			assert.Contains(t, *loadBad2.Setting, `"task_plugin_key":"kept"`)
			assert.Contains(t, *loadBad2.Setting, `"action":"retry"`)

			// A duplicated or non-canonically cased stored error_retry_policy
			// cannot be reproduced by re-encoding, so SetSetting must report the
			// refusal and leave the stored bytes alone: an internal
			// read-modify-write that ignored it would persist a stale setting and
			// still report success.
			rawAmbiguous := `{"proxy":"http://first:1","error_retry_policy":{"enabled":true,"rules":[{"id":"r1","enabled":true,"action":"retry","status_codes":[500]}]},"Error_Retry_Policy":{"enabled":false}}`
			ambiguous := &Channel{Name: "retry-ambiguous", Key: "EMPTY", Status: common.ChannelStatusEnabled, Type: 1, Setting: common.GetPointer(rawAmbiguous)}
			require.NoError(t, table.Create(ambiguous).Error)
			var loadAmbiguous Channel
			require.NoError(t, table.First(&loadAmbiguous, ambiguous.Id).Error)
			ambiguousSetting := loadAmbiguous.GetSetting()
			require.NotEmpty(t, ambiguousSetting.ErrorRetryPolicyDiagnostic)
			ambiguousSetting.Proxy = "http://127.0.0.1:9"
			require.Error(t, loadAmbiguous.SetSetting(ambiguousSetting))
			assert.Equal(t, rawAmbiguous, *loadAmbiguous.Setting)

			// A large but accepted policy (well under both caps) must be stored
			// and read back byte-for-byte without truncation on every database.
			large := oversizedRetryPolicySetting(8, 4)
			require.LessOrEqual(t, len(large), dto.MaxChannelSettingBytes)
			require.NoError(t, (&Channel{Setting: common.GetPointer(large)}).ValidateSettings())
			largeChannel := &Channel{Name: "retry-large", Key: "EMPTY", Status: common.ChannelStatusEnabled, Type: 1, Setting: common.GetPointer(large)}
			require.NoError(t, table.Create(largeChannel).Error)
			var loadLarge Channel
			require.NoError(t, table.First(&loadLarge, largeChannel.Id).Error)
			require.NotNil(t, loadLarge.Setting)
			assert.Equal(t, large, *loadLarge.Setting)
			largeSetting := loadLarge.GetSetting()
			require.NotNil(t, largeSetting.ErrorRetryPolicy)
			assert.Empty(t, largeSetting.ErrorRetryPolicyDiagnostic)
			require.Len(t, largeSetting.ErrorRetryPolicy.Rules, 8)

			// Over the TEXT limit is rejected by the application before any insert.
			huge := oversizedRetryPolicySetting(32, 8)
			require.Greater(t, len(huge), dto.MaxChannelSettingBytes)
			require.Error(t, (&Channel{Setting: common.GetPointer(huge)}).ValidateSettings())
		})
	}
}

func TestEditChannelByTagPatchesChannelSettings(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(filepath.Join(t.TempDir(), "tag-edit.db"))
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				driver = postgres.Open(dsn)
			}
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "tag_edit_test_"}})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)

			// EditChannelByTag uses the package-level DB, so swap it in and restore
			// the shared test database when this dialect case finishes.
			previousDB, previousType := DB, common.MainDatabaseType()
			DB = db
			common.SetMainDatabaseType(common.DatabaseType(dialect))
			initCol()
			t.Cleanup(func() {
				require.NoError(t, db.Migrator().DropTable(&Channel{}, &Ability{}))
				DB = previousDB
				common.SetMainDatabaseType(previousType)
				initCol()
				require.NoError(t, sqlDB.Close())
			})
			require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))

			target := common.GetPointer("surplus-a")
			other := common.GetPointer("other-tag")
			channels := []*Channel{
				{Type: 58, Key: "key-a", Name: "a", Status: common.ChannelStatusEnabled, Tag: target, Setting: common.GetPointer(`{"proxy":"","force_format":true}`)},
				{Type: 58, Key: "key-b", Name: "b", Status: common.ChannelStatusEnabled, Tag: target},
				{Type: 58, Key: "key-c", Name: "c", Status: common.ChannelStatusEnabled, Tag: other, Setting: common.GetPointer(`{"proxy":"http://keep.local:1"}`)},
				{Type: 58, Key: "key-e", Name: "e", Status: common.ChannelStatusEnabled, Tag: target, Setting: common.GetPointer(`{"proxy":`)},
			}
			for _, channel := range channels {
				require.NoError(t, db.Create(channel).Error)
			}

			proxy := "http://new-proxy.local:8080"
			protocol := dto.HTTPProtocolHTTP1
			require.NoError(t, EditChannelByTag("surplus-a", nil, ChannelBatchFields{Proxy: &proxy, HTTPProtocol: &protocol}))

			var gotA, gotB, gotC, gotE Channel
			require.NoError(t, db.First(&gotA, channels[0].Id).Error)
			require.NoError(t, db.First(&gotB, channels[1].Id).Error)
			require.NoError(t, db.First(&gotC, channels[2].Id).Error)
			require.NoError(t, db.First(&gotE, channels[3].Id).Error)

			for _, got := range []*Channel{&gotA, &gotB} {
				setting := got.GetSetting()
				assert.Equal(t, proxy, setting.Proxy)
				assert.Equal(t, dto.HTTPProtocolHTTP1, setting.HTTPProtocol)
				assert.Equal(t, 1, setting.HTTP2ConnectionShards, "http1 must force a single connection shard")
			}
			assert.True(t, gotA.GetSetting().ForceFormat, "unrelated setting fields must survive the patch")
			assert.Equal(t, "key-a", gotA.Key, "the settings patch must not wipe the channel key")
			assert.Equal(t, "key-b", gotB.Key)
			require.NotNil(t, gotB.Setting, "a channel with no setting must receive one")
			require.NotNil(t, gotA.Tag)
			assert.Equal(t, "surplus-a", *gotA.Tag)

			assert.Equal(t, "http://keep.local:1", gotC.GetSetting().Proxy, "channels under another tag must be untouched")

			// A malformed stored setting makes GetSetting rewrite the row; the
			// credential must survive that path.
			assert.Equal(t, "key-e", gotE.Key, "a malformed stored setting must not wipe the channel key")
			assert.Equal(t, proxy, gotE.GetSetting().Proxy)

			// More than one shard only means anything for HTTP/2, so it also
			// lifts the HTTP/1.1 pin instead of storing a contradiction.
			shards := 4
			require.NoError(t, EditChannelByTag("surplus-a", nil, ChannelBatchFields{HTTP2ConnectionShards: &shards}))
			var gotA2 Channel
			require.NoError(t, db.First(&gotA2, channels[0].Id).Error)
			settingA2 := gotA2.GetSetting()
			assert.Equal(t, 4, settingA2.HTTP2ConnectionShards)
			assert.Empty(t, settingA2.HTTPProtocol, "multiple shards must not coexist with an HTTP/1.1 pin")
			assert.Equal(t, proxy, settingA2.Proxy, "a shard-only patch must leave the proxy alone")

			cleared := ""
			require.NoError(t, EditChannelByTag("other-tag", nil, ChannelBatchFields{Proxy: &cleared}))
			var gotC2 Channel
			require.NoError(t, db.First(&gotC2, channels[2].Id).Error)
			assert.Empty(t, gotC2.GetSetting().Proxy, "an empty proxy patch must clear the stored proxy")

			// Renaming a tag onto a tag that already exists must not spill the
			// settings patch onto the channels that were already there.
			existing := &Channel{Type: 58, Key: "key-existing", Name: "existing", Status: common.ChannelStatusEnabled, Tag: common.GetPointer("target-tag"), Setting: common.GetPointer(`{"proxy":"http://existing.local:1"}`)}
			require.NoError(t, db.Create(existing).Error)
			source := &Channel{Type: 58, Key: "key-source", Name: "source", Status: common.ChannelStatusEnabled, Tag: common.GetPointer("source-tag")}
			require.NoError(t, db.Create(source).Error)

			mergeTarget := "target-tag"
			mergeProxy := "http://merge.local:8080"
			require.NoError(t, EditChannelByTag("source-tag", &mergeTarget, ChannelBatchFields{Proxy: &mergeProxy}))

			var gotExisting, gotSource Channel
			require.NoError(t, db.First(&gotExisting, existing.Id).Error)
			require.NoError(t, db.First(&gotSource, source.Id).Error)
			assert.Equal(t, "http://existing.local:1", gotExisting.GetSetting().Proxy, "channels already in the target tag must keep their settings")
			assert.Equal(t, mergeProxy, gotSource.GetSetting().Proxy)
			require.NotNil(t, gotSource.Tag)
			assert.Equal(t, "target-tag", *gotSource.Tag)
		})
	}
}

func TestEditChannelByIDsPatchesChannelSettings(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(filepath.Join(t.TempDir(), "id-edit.db"))
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				driver = postgres.Open(dsn)
			}
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "id_edit_test_"}})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)

			// EditChannelByIDs uses the package-level DB, so swap it in and
			// restore the shared test database when this dialect case finishes.
			previousDB, previousType := DB, common.MainDatabaseType()
			DB = db
			common.SetMainDatabaseType(common.DatabaseType(dialect))
			initCol()
			t.Cleanup(func() {
				require.NoError(t, db.Migrator().DropTable(&Channel{}, &Ability{}))
				DB = previousDB
				common.SetMainDatabaseType(previousType)
				initCol()
				require.NoError(t, sqlDB.Close())
			})
			require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))

			selectedTag := common.GetPointer("sel")
			otherTag := common.GetPointer("other")
			channels := []*Channel{
				{Type: 58, Key: "key-a", Name: "a", Status: common.ChannelStatusEnabled, Tag: selectedTag, Setting: common.GetPointer(`{"proxy":"","force_format":true}`)},
				{Type: 58, Key: "key-b", Name: "b", Status: common.ChannelStatusEnabled, Tag: selectedTag},
				{Type: 58, Key: "key-c", Name: "c", Status: common.ChannelStatusEnabled, Tag: otherTag, Setting: common.GetPointer(`{"proxy":"http://keep.local:1"}`)},
				{Type: 58, Key: "key-e", Name: "e", Status: common.ChannelStatusEnabled, Tag: selectedTag, Setting: common.GetPointer(`{"proxy":`)},
			}
			for _, channel := range channels {
				require.NoError(t, db.Create(channel).Error)
			}

			proxy := "http://new-proxy.local:8080"
			protocol := dto.HTTPProtocolHTTP1
			require.NoError(t, EditChannelByIDs([]int{channels[0].Id, channels[1].Id, channels[3].Id}, ChannelBatchFields{Proxy: &proxy, HTTPProtocol: &protocol}))

			var gotA, gotB, gotC, gotE Channel
			require.NoError(t, db.First(&gotA, channels[0].Id).Error)
			require.NoError(t, db.First(&gotB, channels[1].Id).Error)
			require.NoError(t, db.First(&gotC, channels[2].Id).Error)
			require.NoError(t, db.First(&gotE, channels[3].Id).Error)

			for _, got := range []*Channel{&gotA, &gotB} {
				setting := got.GetSetting()
				assert.Equal(t, proxy, setting.Proxy)
				assert.Equal(t, dto.HTTPProtocolHTTP1, setting.HTTPProtocol)
				assert.Equal(t, 1, setting.HTTP2ConnectionShards, "http1 must force a single connection shard")
			}
			assert.True(t, gotA.GetSetting().ForceFormat, "unrelated setting fields must survive the patch")
			assert.Equal(t, "key-a", gotA.Key, "the settings patch must not wipe the channel key")
			assert.Equal(t, "key-b", gotB.Key)
			require.NotNil(t, gotB.Setting, "a channel with no setting must receive one")

			assert.Equal(t, "http://keep.local:1", gotC.GetSetting().Proxy, "channels outside the id set must be untouched")

			// A malformed stored setting makes GetSetting rewrite the row; the
			// credential must survive that path.
			assert.Equal(t, "key-e", gotE.Key, "a malformed stored setting must not wipe the channel key")
			assert.Equal(t, proxy, gotE.GetSetting().Proxy)

			// More than one shard only means anything for HTTP/2, so it also
			// lifts the HTTP/1.1 pin instead of storing a contradiction.
			shards := 4
			require.NoError(t, EditChannelByIDs([]int{channels[0].Id}, ChannelBatchFields{HTTP2ConnectionShards: &shards}))
			var gotA2 Channel
			require.NoError(t, db.First(&gotA2, channels[0].Id).Error)
			settingA2 := gotA2.GetSetting()
			assert.Equal(t, 4, settingA2.HTTP2ConnectionShards)
			assert.Empty(t, settingA2.HTTPProtocol, "multiple shards must not coexist with an HTTP/1.1 pin")
			assert.Equal(t, proxy, settingA2.Proxy, "a shard-only patch must leave the proxy alone")

			cleared := ""
			require.NoError(t, EditChannelByIDs([]int{channels[2].Id}, ChannelBatchFields{Proxy: &cleared}))
			var gotC2 Channel
			require.NoError(t, db.First(&gotC2, channels[2].Id).Error)
			assert.Empty(t, gotC2.GetSetting().Proxy, "an empty proxy patch must clear the stored proxy")

			// A model change must rebuild routing abilities for the selected
			// channels and leave the rest of the columns/rows alone.
			models := "gpt-4o,claude-3"
			require.NoError(t, EditChannelByIDs([]int{channels[0].Id, channels[1].Id}, ChannelBatchFields{Models: &models}))
			var gotA3, gotC3 Channel
			require.NoError(t, db.First(&gotA3, channels[0].Id).Error)
			require.NoError(t, db.First(&gotC3, channels[2].Id).Error)
			assert.Equal(t, models, gotA3.Models)
			assert.Empty(t, gotC3.Models, "channels outside the id set must not receive models")
		})
	}
}

func TestEditChannelByIDsBatchModesAndSettings(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(filepath.Join(t.TempDir(), "id-modes.db"))
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				driver = postgres.Open(dsn)
			}
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "id_modes_test_"}})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)

			previousDB, previousType := DB, common.MainDatabaseType()
			DB = db
			common.SetMainDatabaseType(common.DatabaseType(dialect))
			initCol()
			t.Cleanup(func() {
				require.NoError(t, db.Migrator().DropTable(&Channel{}, &Ability{}))
				DB = previousDB
				common.SetMainDatabaseType(previousType)
				initCol()
				require.NoError(t, sqlDB.Close())
			})
			require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))

			group := "default"
			mapping := `{"gpt-4o":"gpt-4o-mini"}`
			channels := []*Channel{
				{Type: 1, Key: "key-a", Name: "a", Status: common.ChannelStatusEnabled, Models: "gpt-4o", Group: group, ModelMapping: &mapping, Setting: common.GetPointer(`{"proxy":"http://keep.local:1"}`)},
				{Type: 1, Key: "key-b", Name: "b", Status: common.ChannelStatusEnabled, Models: "claude-3", Group: group},
				{Type: 1, Key: "key-c", Name: "c", Status: common.ChannelStatusEnabled, Models: "other-model", Group: group},
			}
			for _, channel := range channels {
				require.NoError(t, db.Create(channel).Error)
			}
			a, b, c := channels[0].Id, channels[1].Id, channels[2].Id

			// Append models: union with the current list, deduped, existing first.
			addedModels := "gpt-4o,gpt-4.1"
			require.NoError(t, EditChannelByIDs([]int{a, b}, ChannelBatchFields{Models: &addedModels, ModelsMode: "append"}))
			var gotA, gotB, gotC Channel
			require.NoError(t, db.First(&gotA, a).Error)
			require.NoError(t, db.First(&gotB, b).Error)
			require.NoError(t, db.First(&gotC, c).Error)
			assert.Equal(t, "gpt-4o,gpt-4.1", gotA.Models)
			assert.Equal(t, "claude-3,gpt-4o,gpt-4.1", gotB.Models)
			assert.Equal(t, "other-model", gotC.Models, "unselected channels must keep their models")

			// Merge model_mapping: overlay the new entries and keep the old ones.
			addedMapping := `{"gpt-4.1":"upstream-41"}`
			require.NoError(t, EditChannelByIDs([]int{a}, ChannelBatchFields{ModelMapping: &addedMapping, ModelMappingMode: "merge"}))
			require.NoError(t, db.First(&gotA, a).Error)
			mergedMapping := map[string]string{}
			require.NoError(t, common.UnmarshalJsonStr(*gotA.ModelMapping, &mergedMapping))
			assert.Equal(t, map[string]string{"gpt-4o": "gpt-4o-mini", "gpt-4.1": "upstream-41"}, mergedMapping)

			// Append groups: union with the current list.
			addedGroup := "vip"
			require.NoError(t, EditChannelByIDs([]int{a}, ChannelBatchFields{Group: &addedGroup, GroupMode: "append"}))
			require.NoError(t, db.First(&gotA, a).Error)
			assert.Equal(t, "default,vip", gotA.Group)

			// Settings patch: booleans, string, JSON config and retry policy in
			// the "setting" column.
			backfill := true
			prompt := "be nice"
			promptOverride := true
			timeout := map[string][]dto.FirstResponseTimeoutTier{
				"gpt-4o": {{ContextTokens: 200000, TimeoutMs: 3000}},
			}
			policy := &dto.ChannelErrorRetryPolicy{
				Enabled: true,
				Rules: []dto.ChannelErrorRetryRule{
					{ID: "r1", Enabled: true, Action: dto.ChannelErrorRetryActionRetry, StatusCodes: []int{500}},
				},
			}
			require.NoError(t, EditChannelByIDs([]int{a}, ChannelBatchFields{Settings: &ChannelBatchSettings{
				ReasoningContentBackfill:  &backfill,
				SystemPrompt:              &prompt,
				SystemPromptOverride:      &promptOverride,
				ModelFirstResponseTimeout: &timeout,
				ErrorRetryPolicy:          policy,
			}}))
			require.NoError(t, db.First(&gotA, a).Error)
			setting := gotA.GetSetting()
			assert.True(t, setting.ReasoningContentBackfill)
			assert.Equal(t, "be nice", setting.SystemPrompt)
			assert.True(t, setting.SystemPromptOverride)
			assert.Equal(t, timeout, setting.ModelFirstResponseTimeout)
			require.NotNil(t, setting.ErrorRetryPolicy)
			assert.True(t, setting.ErrorRetryPolicy.Enabled)
			assert.Equal(t, "http://keep.local:1", setting.Proxy, "unrelated settings must survive the patch")

			// Merge model_first_response_timeout: keep the old model key.
			timeoutAdd := map[string][]dto.FirstResponseTimeoutTier{
				"claude-3": {{ContextTokens: 1000, TimeoutMs: 1000}},
			}
			require.NoError(t, EditChannelByIDs([]int{a}, ChannelBatchFields{Settings: &ChannelBatchSettings{
				ModelFirstResponseTimeout:     &timeoutAdd,
				ModelFirstResponseTimeoutMode: "merge",
			}}))
			require.NoError(t, db.First(&gotA, a).Error)
			setting = gotA.GetSetting()
			assert.Len(t, setting.ModelFirstResponseTimeout, 2)
			assert.Equal(t, 3000, setting.ModelFirstResponseTimeout["gpt-4o"][0].TimeoutMs)
			assert.Equal(t, 1000, setting.ModelFirstResponseTimeout["claude-3"][0].TimeoutMs)

			// disable_task_polling_sleep lives in the "settings" column.
			sleep := true
			require.NoError(t, EditChannelByIDs([]int{a}, ChannelBatchFields{OtherSettings: &ChannelBatchOtherSettings{DisableTaskPollingSleep: &sleep}}))
			require.NoError(t, db.First(&gotA, a).Error)
			assert.True(t, gotA.GetOtherSettings().DisableTaskPollingSleep)
		})
	}
}
