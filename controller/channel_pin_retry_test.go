package controller

import (
	"errors"
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldRetryHonorsPinRetryMode(t *testing.T) {
	openaiErr := types.NewOpenAIError(errors.New("upstream"), types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError)

	c := newPinRetryContext()
	assert.True(t, service.ShouldRetryRelayError(c, openaiErr, 1))

	origin := newPinRetryContext()
	service.GetChannelConstraints(origin).AddPin(dto.ChannelPin{
		ChannelId: 2,
		Source:    dto.PinSourceOriginTask,
		Rank:      dto.PinRankOriginTask,
		RetryMode: dto.PinRetrySameChannel,
	})
	assert.True(t, service.ShouldRetryRelayError(origin, openaiErr, 1), "origin pin retries on the same channel")

	token := newPinRetryContext()
	service.GetChannelConstraints(token).AddPin(dto.ChannelPin{
		ChannelId: 1,
		Source:    dto.PinSourceToken,
		Rank:      dto.PinRankToken,
		RetryMode: dto.PinRetrySingleAttempt,
	})
	assert.Equal(t, service.PolicyDecision{Action: "stop", Reason: "pinned_channel", Source: "channel_constraint"}, service.DecideRelayRetry(token, openaiErr, 1), "token pin suppresses retry")
}

func TestShouldRetryTaskRelayHonorsPinRetryMode(t *testing.T) {
	taskErr := &dto.TaskError{StatusCode: http.StatusInternalServerError}

	c := newPinRetryContext()
	assert.Equal(t, "retry", decideTaskRetry(c, taskErr, 1).Action)

	origin := newPinRetryContext()
	service.GetChannelConstraints(origin).AddPin(dto.ChannelPin{
		ChannelId: 2,
		Source:    dto.PinSourceOriginTask,
		Rank:      dto.PinRankOriginTask,
		RetryMode: dto.PinRetrySameChannel,
	})
	assert.Equal(t, "retry", decideTaskRetry(origin, taskErr, 1).Action)

	token := newPinRetryContext()
	service.GetChannelConstraints(token).AddPin(dto.ChannelPin{
		ChannelId: 1,
		Source:    dto.PinSourceToken,
		Rank:      dto.PinRankToken,
		RetryMode: dto.PinRetrySingleAttempt,
	})
	assert.Equal(t, service.PolicyDecision{Action: "stop", Reason: "pinned_channel", Source: "channel_constraint"}, decideTaskRetry(token, taskErr, 1))
}

func TestSameChannelPinsMergeToStricterRetryMode(t *testing.T) {
	c := newPinRetryContext()
	constraints := service.GetChannelConstraints(c)
	constraints.AddPin(dto.ChannelPin{
		ChannelId: 7,
		Source:    dto.PinSourceOriginTask,
		Rank:      dto.PinRankOriginTask,
		RetryMode: dto.PinRetrySameChannel,
	})
	constraints.AddPin(dto.ChannelPin{
		ChannelId: 7,
		Source:    dto.PinSourceToken,
		Rank:      dto.PinRankToken,
		RetryMode: dto.PinRetrySingleAttempt,
	})
	pin, found, overridden := constraints.ResolvedPin()
	require.True(t, found)
	assert.Equal(t, 7, pin.ChannelId)
	assert.Equal(t, dto.PinRetrySingleAttempt, pin.RetryMode)
	assert.Empty(t, overridden)
	assert.False(t, service.ShouldRetryRelayError(c, types.NewOpenAIError(errors.New("upstream"), types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError), 1))
}

func newPinRetryContext() *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c
}

func TestRequestPolicyConfigReturnsSettingsWithoutMigration(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	GetRequestPolicy(ctx)
	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Contains(t, response.Data, "options")
	assert.NotContains(t, response.Data, "migration")
	assert.NotContains(t, response.Data, "differences")
}

func TestRequestPolicyRoutingDatabaseMatrix(t *testing.T) {
	require.NoError(t, i18n.Init())
	for _, dialect := range []struct{ kind, env string }{{"sqlite", ""}, {"mysql", "TEST_MYSQL_DSN"}, {"postgres", "TEST_POSTGRES_DSN"}} {
		t.Run(dialect.kind, func(t *testing.T) {
			dsn := ""
			if dialect.env != "" {
				dsn = os.Getenv(dialect.env)
				if dsn == "" {
					t.Skipf("%s not configured", dialect.env)
				}
			}
			db := modelManagementDB(t, dialect.kind, dsn)
			previousGroups := setting.UserUsableGroups2JSONString()
			previousRatios, err := common.Marshal(ratio_setting.GetGroupRatioCopy())
			require.NoError(t, err)
			require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
			require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
			t.Cleanup(func() {
				require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousGroups))
				require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(string(previousRatios)))
			})
			// Channel 1 holds the session binding but is no longer usable.
			channels := []model.Channel{
				{Id: 1, Name: "A", Type: 1, Key: "test-only", Status: common.ChannelStatusAutoDisabled, Models: "policy-test", Group: "default", Priority: common.GetPointer(int64(10))},
				{Id: 2, Name: "B", Type: 1, Key: "test-only", Status: common.ChannelStatusEnabled, Models: "policy-test", Group: "default", Priority: common.GetPointer(int64(5))},
			}
			for i := range channels {
				require.NoError(t, db.Create(&channels[i]).Error)
				require.NoError(t, channels[i].AddAbilities(db))
			}
			affinity := operation_setting.GetChannelAffinitySetting()
			previousAffinity := *affinity
			t.Cleanup(func() { *affinity = previousAffinity })
			for _, cached := range []bool{false, true} {
				for _, keep := range []bool{false, true} {
					for _, tc := range []struct {
						globalMode, ruleMode string
						blocked              bool
					}{
						{"strict", "inherit", true},
						{"prefer", "strict", true},
						{"prefer", "inherit", false},
						{"strict", "prefer", false},
					} {
						t.Run(fmt.Sprintf("cache=%t/keep=%t/global=%s/rule=%s", cached, keep, tc.globalMode, tc.ruleMode), func(t *testing.T) {
							common.MemoryCacheEnabled = cached
							model.InitChannelCache()
							snapshot, err := model.BuildRequestPolicy(map[string]string{
								"channel_affinity_setting.enabled":                  "true",
								"channel_affinity_setting.session_mode":             tc.globalMode,
								"channel_affinity_setting.keep_on_channel_disabled": fmt.Sprint(keep),
								"channel_affinity_setting.rules":                    fmt.Sprintf(`[{"name":"session","model_regex":[".*"],"key_sources":[{"type":"request_header","key":"X-Session"}],"session_mode":%q}]`, tc.ruleMode),
							})
							require.NoError(t, err)
							*affinity = snapshot.Affinity
							seed := newPinRetryContext()
							seed.Request.Header.Set("X-Session", t.Name())
							_, found := service.GetPreferredChannelByAffinity(seed, "policy-test", "default")
							require.False(t, found)
							seed.Set("channel_id", 1)
							service.RecordChannelAffinity(seed, 1)
							t.Cleanup(func() { service.ClearCurrentChannelAffinityCache(seed) })
							bound, found := service.GetPreferredChannelByAffinity(seed, "policy-test", "default")
							require.True(t, found)
							require.Equal(t, 1, bound)

							request := newPinRetryContext()
							request.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"policy-test"}`))
							request.Request.Header.Set("Content-Type", "application/json")
							request.Request.Header.Set("X-Session", t.Name())
							common.SetContextKey(request, constant.ContextKeyUsingGroup, "default")
							middleware.Distribute()(request)
							events := service.RequestPolicy(request).Events()
							assert.True(t, slices.ContainsFunc(events, func(event service.PolicyEvent) bool { return event.Decision.Reason == "session_rule_matched" }), "decision events are recorded for every request")
							bound, found = service.GetPreferredChannelByAffinity(seed, "policy-test", "default")
							if tc.blocked {
								assert.Equal(t, http.StatusServiceUnavailable, request.Writer.Status())
								assert.True(t, request.IsAborted())
								assert.Equal(t, keep, found, "binding retention is independent of strict request handling")
								if found {
									assert.Equal(t, 1, bound)
								}
								return
							}
							assert.False(t, request.IsAborted())
							assert.Equal(t, 2, common.GetContextKeyInt(request, constant.ContextKeyChannelId), "prefer falls back to the next eligible channel")
							require.True(t, found)
							assert.Equal(t, 2, bound, "a successful fallback rebinds the session")
						})
					}
				}
			}
		})
	}
}

// TestChannelErrorRetryPolicyControlsRealRelayAttempts runs the real relay loop
// against a mock upstream and proves a channel policy decides the number of
// upstream attempts, while the global budget, a miss and a token pin keep their
// original effect.
func TestChannelErrorRetryPolicyControlsRealRelayAttempts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user, token := setupResponsesWSRequestTest(t)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		switch key {
		case "billing_setting.billing_mode", "billing_setting.billing_expr", "group_ratio_setting.group_ratio":
			saved[key] = value
		}
		return nil
	}))
	t.Cleanup(func() { require.NoError(t, config.GlobalConfig.LoadFromDB(saved)) })
	expressions, err := common.Marshal(map[string]string{"retry-policy-test": `tier("request", fixed(0.002))`})
	require.NoError(t, err)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode":    `{"retry-policy-test":"tiered_expr"}`,
		"billing_setting.billing_expr":    string(expressions),
		"group_ratio_setting.group_ratio": `{"default":1}`,
	}))
	previousCountToken, previousQuotaPerUnit := constant.CountToken, common.QuotaPerUnit
	previousAutoDisable := common.AutomaticDisableChannelEnabled
	constant.CountToken, common.QuotaPerUnit = false, 500000
	common.AutomaticDisableChannelEnabled = false
	t.Cleanup(func() {
		constant.CountToken, common.QuotaPerUnit = previousCountToken, previousQuotaPerUnit
		common.AutomaticDisableChannelEnabled = previousAutoDisable
	})
	require.NoError(t, model.DB.Model(user).Updates(map[string]any{"quota": 100000000, "setting": `{"billing_preference":"wallet_only"}`}).Error)
	require.NoError(t, model.DB.Model(token).Update("remain_quota", 100000000).Error)
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))

	var attempts atomic.Int32
	var upstreamMessage atomic.Value
	upstreamMessage.Store("temporary supplier fault")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, `{"error":{"message":%q,"type":"server_error","code":"temporary"}}`, upstreamMessage.Load().(string))
	}))
	t.Cleanup(upstream.Close)

	baseURL := upstream.URL
	channel := &model.Channel{Name: "retry-policy", Key: "test-only", Status: common.ChannelStatusEnabled, Type: constant.ChannelTypeOpenAI, Group: "default", Models: "retry-policy-test", BaseURL: &baseURL}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{ChannelId: channel.Id, Model: "retry-policy-test", Group: "default", Enabled: true}).Error)
	t.Cleanup(func() {
		require.NoError(t, model.DB.Where("channel_id = ?", channel.Id).Delete(&model.Ability{}).Error)
		require.NoError(t, model.DB.Delete(channel).Error)
	})
	model.InitChannelCache()

	engine := gin.New()
	engine.POST("/v1/chat/completions", middleware.TokenAuth(), func(c *gin.Context) {
		if c.GetHeader("X-Test-Pin") == "single-attempt" {
			service.GetChannelConstraints(c).AddPin(dto.ChannelPin{ChannelId: channel.Id, Source: dto.PinSourceToken, Rank: dto.PinRankToken, RetryMode: dto.PinRetrySingleAttempt})
		}
		c.Next()
	}, middleware.Distribute(), func(c *gin.Context) {
		Relay(c, types.RelayFormatOpenAI)
	})
	gateway := httptest.NewServer(engine)
	t.Cleanup(gateway.Close)

	setPolicy := func(policy *kitdto.ChannelErrorRetryPolicy) {
		t.Helper()
		channel.SetSetting(kitdto.ChannelSettings{ErrorRetryPolicy: policy})
		require.NoError(t, channel.ValidateSettings())
		require.NoError(t, model.DB.Model(channel).Update("setting", *channel.Setting).Error)
	}
	call := func(pin string) int {
		t.Helper()
		attempts.Store(0)
		request, err := http.NewRequest(http.MethodPost, gateway.URL+"/v1/chat/completions", strings.NewReader(`{"model":"retry-policy-test","messages":[{"role":"user","content":"hi"}]}`))
		require.NoError(t, err)
		request.Header.Set("Authorization", "Bearer sk-"+token.Key)
		request.Header.Set("Content-Type", "application/json")
		if pin != "" {
			request.Header.Set("X-Test-Pin", pin)
		}
		response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
		require.NoError(t, err)
		_ = response.Body.Close()
		return int(attempts.Load())
	}
	retryRule := kitdto.ChannelErrorRetryRule{
		ID: "retry-temporary", Enabled: true, Action: kitdto.ChannelErrorRetryActionRetry, StatusCodes: []int{http.StatusBadRequest},
		Conditions: []kitdto.ChannelErrorRetryCondition{{Field: kitdto.ChannelErrorRetryFieldMessage, Operator: kitdto.ChannelErrorRetryOperatorContains, Value: "temporary supplier fault"}},
	}

	previousRetries := common.RetryTimes
	common.RetryTimes = 1
	t.Cleanup(func() { common.RetryTimes = previousRetries })

	t.Run("retry rule drives a second attempt despite the global status rule", func(t *testing.T) {
		upstreamMessage.Store("temporary supplier fault")
		setPolicy(&kitdto.ChannelErrorRetryPolicy{Enabled: true, Rules: []kitdto.ChannelErrorRetryRule{retryRule}})
		assert.Equal(t, 2, call(""))
	})

	t.Run("stop rule ends after the first attempt", func(t *testing.T) {
		upstreamMessage.Store("unsupported parameter")
		setPolicy(&kitdto.ChannelErrorRetryPolicy{Enabled: true, Rules: []kitdto.ChannelErrorRetryRule{{
			ID: "stop-parameter", Enabled: true, Action: kitdto.ChannelErrorRetryActionStop, StatusCodes: []int{http.StatusBadRequest},
			Conditions: []kitdto.ChannelErrorRetryCondition{{Field: kitdto.ChannelErrorRetryFieldMessage, Operator: kitdto.ChannelErrorRetryOperatorContains, Value: "unsupported parameter"}},
		}}})
		assert.Equal(t, 1, call(""))
	})

	t.Run("miss keeps the legacy single attempt for status 400", func(t *testing.T) {
		upstreamMessage.Store("unrelated supplier fault")
		assert.Equal(t, 1, call(""))
	})

	t.Run("retry rule cannot bypass the token single-attempt pin", func(t *testing.T) {
		upstreamMessage.Store("temporary supplier fault")
		setPolicy(&kitdto.ChannelErrorRetryPolicy{Enabled: true, Rules: []kitdto.ChannelErrorRetryRule{retryRule}})
		assert.Equal(t, 1, call("single-attempt"))
	})
}
