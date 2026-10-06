package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestShouldRetryRelayErrorHonorsChannelPinOnChannelError(t *testing.T) {
	err := types.NewError(errors.New("channel failed"), types.ErrorCodeChannelNoAvailableKey)
	for _, test := range []struct {
		name      string
		pin       *dto.ChannelPin
		wantRetry bool
	}{
		{name: "unrestricted channel error", wantRetry: true},
		{
			name: "single attempt pin suppresses channel error retry",
			pin: &dto.ChannelPin{
				ChannelId: 1, Source: dto.PinSourceToken, Rank: dto.PinRankToken, RetryMode: dto.PinRetrySingleAttempt,
			},
			wantRetry: false,
		},
		{
			name: "origin task pin permits retry on the same channel",
			pin: &dto.ChannelPin{
				ChannelId: 1, Source: dto.PinSourceOriginTask, Rank: dto.PinRankOriginTask, RetryMode: dto.PinRetrySameChannel,
			},
			wantRetry: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			if test.pin != nil {
				GetChannelConstraints(c).AddPin(*test.pin)
			}
			assert.Equal(t, test.wantRetry, ShouldRetryRelayError(c, err, 1))
		})
	}
}

func TestProcessChannelErrorMasksDisableReasonAndNotification(t *testing.T) {
	previousDB, previousType := model.DB, common.MainDatabaseType()
	previousCache, previousRedis := common.MemoryCacheEnabled, common.RedisEnabled
	previousAutoDisable, previousErrorLog := common.AutomaticDisableChannelEnabled, constant.ErrorLogEnabled
	previousNotifyLimit := constant.NotifyLimitCount
	previousClient, previousWorker := httpClient, system_setting.WorkerUrl
	fetch := system_setting.GetFetchSetting()
	previousFetch := *fetch
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.MemoryCacheEnabled, common.RedisEnabled = previousCache, previousRedis
		common.AutomaticDisableChannelEnabled, constant.ErrorLogEnabled = previousAutoDisable, previousErrorLog
		constant.NotifyLimitCount = previousNotifyLimit
		httpClient, system_setting.WorkerUrl = previousClient, previousWorker
		*fetch = previousFetch
	})
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, database.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.User{}))
	model.DB = database
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.MemoryCacheEnabled, common.RedisEnabled = false, false
	common.AutomaticDisableChannelEnabled, constant.ErrorLogEnabled = true, false
	constant.NotifyLimitCount = 10
	channel := &model.Channel{Name: "relay-review", Key: "fixture-key", Type: 1, Status: common.ChannelStatusEnabled, Group: "default", Models: "test-model"}
	require.NoError(t, channel.Insert())
	notifications := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		notifications <- body
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	httpClient, system_setting.WorkerUrl = server.Client(), ""
	fetch.EnableSSRFProtection = false
	settings, err := common.Marshal(kitdto.UserSetting{NotifyType: kitdto.NotifyTypeWebhook, WebhookUrl: server.URL})
	require.NoError(t, err)
	root := &model.User{Username: "notification-test-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled, Setting: string(settings)}
	require.NoError(t, database.Create(root).Error)
	notifyKey := fmt.Sprintf("%d:%s:%s", root.Id, formatNotifyType(channel.Id, common.ChannelStatusAutoDisabled), time.Now().Format("2006010215"))
	notifyLimitStore.Delete(notifyKey)
	t.Cleanup(func() { notifyLimitStore.Delete(notifyKey) })
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	apiErr := types.NewErrorWithStatusCode(errors.New("upstream https://private.example.com/path?token=review-token api_key:review-secret"), types.ErrorCodeChannelNoAvailableKey, http.StatusUnauthorized)
	ProcessChannelError(c, types.ChannelError{ChannelId: channel.Id, ChannelName: channel.Name, AutoBan: true}, apiErr, nil)
	var notification WebhookPayload
	select {
	case payload := <-notifications:
		require.NoError(t, common.Unmarshal(payload, &notification))
	case <-time.After(5 * time.Second):
		t.Fatal("automatic channel-disable notification was not delivered")
	}
	loaded, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusAutoDisabled, loaded.Status)
	wantReason := "status_code=401, upstream https://***.com/***?token=*** api_key:***"
	assert.Equal(t, wantReason, loaded.GetOtherInfo()["status_reason"])
	assert.Contains(t, notification.Content, wantReason)
	assert.NotContains(t, notification.Content, "review-token")
	assert.NotContains(t, notification.Content, "review-secret")
	assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode)
}

func TestDecideRelayRetryReasons(t *testing.T) {
	upstream := func(status int) *types.NewAPIError {
		return types.NewOpenAIError(errors.New("upstream"), types.ErrorCodeBadResponseStatusCode, status)
	}
	for _, tc := range []struct {
		name    string
		err     *types.NewAPIError
		retries int
		setup   func(*gin.Context)
		want    PolicyDecision
	}{
		{name: "retry status matched", err: upstream(http.StatusTooManyRequests), retries: 1, want: PolicyDecision{Action: "retry", Reason: "retry_status_matched", Source: "global"}},
		{name: "status outside retry rules", err: upstream(http.StatusBadRequest), retries: 1, want: PolicyDecision{Action: "stop", Reason: "status_not_retryable", Source: "global"}},
		{name: "attempt budget exhausted", err: upstream(http.StatusTooManyRequests), retries: 0, want: PolicyDecision{Action: "stop", Reason: "attempt_budget_exhausted", Source: "global"}},
		{name: "always skipped status", err: upstream(http.StatusGatewayTimeout), retries: 1, want: PolicyDecision{Action: "stop", Reason: "system_retry_exclusion", Source: "system"}},
		{name: "success status never retries", err: upstream(http.StatusOK), retries: 1, want: PolicyDecision{Action: "stop", Reason: "system_retry_exclusion", Source: "system"}},
		{name: "skip retry error", err: types.NewErrorWithStatusCode(errors.New("local"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry()), retries: 1, want: PolicyDecision{Action: "stop", Reason: "non_retryable_error", Source: "system"}},
		{name: "channel error retries without budget", err: types.NewError(errors.New("no key"), types.ErrorCodeChannelNoAvailableKey), retries: 0, want: PolicyDecision{Action: "retry", Reason: "channel_error", Source: "system"}},
		{name: "single attempt pin", err: upstream(http.StatusTooManyRequests), retries: 1, setup: func(c *gin.Context) {
			GetChannelConstraints(c).AddPin(dto.ChannelPin{ChannelId: 1, Source: dto.PinSourceToken, Rank: dto.PinRankToken, RetryMode: dto.PinRetrySingleAttempt})
		}, want: PolicyDecision{Action: "stop", Reason: "pinned_channel", Source: "channel_constraint"}},
		{name: "strict session", err: upstream(http.StatusTooManyRequests), retries: 1, setup: func(c *gin.Context) {
			c.Set(ginKeyChannelAffinitySkipRetry, true)
			RequestPolicy(c).SessionModeSource = "global"
		}, want: PolicyDecision{Action: "stop", Reason: "strict_session", Source: "global"}},
		{name: "nil error", retries: 1, want: PolicyDecision{Action: "stop", Reason: "request_completed", Source: "system"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			if tc.setup != nil {
				tc.setup(c)
			}
			decision := DecideRelayRetry(c, tc.err, tc.retries)
			assert.Equal(t, tc.want, decision)
			assert.Equal(t, tc.want.Action == "retry", ShouldRetryRelayError(c, tc.err, tc.retries))
		})
	}
}

func TestWouldRetryFirstByteTimeoutGatesOnRetryAndChannelChange(t *testing.T) {
	previousRetryTimes := common.RetryTimes
	previousRanges := operation_setting.AutomaticRetryStatusCodeRanges
	t.Cleanup(func() {
		common.RetryTimes = previousRetryTimes
		operation_setting.AutomaticRetryStatusCodeRanges = previousRanges
	})
	common.RetryTimes = 1

	// The timeout must not abort the request when the operator's retry status-code
	// rules do not cover 500, even if another candidate channel were available.
	operation_setting.AutomaticRetryStatusCodeRanges = []operation_setting.StatusCodeRange{{Start: 429, End: 429}}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.False(t, WouldRetryFirstByteTimeout(c, "default", "m", 0))

	// An exhausted retry budget also suppresses the timeout.
	operation_setting.AutomaticRetryStatusCodeRanges = previousRanges
	require.False(t, WouldRetryFirstByteTimeout(c, "default", "m", 1))

	// A same-channel pin retries on the very same channel, so a timeout would not
	// fail over anywhere: it must stay disarmed.
	pinned, _ := gin.CreateTestContext(httptest.NewRecorder())
	GetChannelConstraints(pinned).AddPin(dto.ChannelPin{
		ChannelId: 7, Source: dto.PinSourceOriginTask, Rank: dto.PinRankOriginTask, RetryMode: dto.PinRetrySameChannel,
	})
	require.False(t, WouldRetryFirstByteTimeout(pinned, "default", "m", 0))
}

func TestRequestPolicyEventsReachLogAdminInfo(t *testing.T) {
	previousAutoDisable := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() { common.AutomaticDisableChannelEnabled = previousAutoDisable })
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("auto_ban", true)
	c.Set("channel_id", 7)
	state := RequestPolicy(c)
	state.BeginAttempt(&model.Channel{Id: 7}, "default")
	apiErr := types.NewOpenAIError(errors.New("invalid credential"), types.ErrorCodeBadResponseStatusCode, http.StatusUnauthorized)
	RecordPolicyFailure(c, 7, apiErr, DecideRelayRetry(c, apiErr, 0))

	failed := model.NewLogOther()
	AppendRelayLogAdminInfo(c, nil, failed)
	events, ok := failed.Snapshot()["admin_info"].(map[string]any)["request_policy"].([]PolicyEvent)
	require.True(t, ok, "a failed relay exposes its decision events to administrators")
	require.Len(t, events, 3)
	assert.Equal(t, PolicyDecision{Action: "attempt", Reason: "channel_selected", Source: "routing"}, events[0].Decision)
	assert.Equal(t, "default", events[0].Group)
	assert.Equal(t, PolicyDecision{Action: "failure", Reason: "upstream_failure", Source: "upstream"}, events[1].Decision)
	assert.Equal(t, http.StatusUnauthorized, events[1].Status)
	assert.Equal(t, PolicyDecision{Action: "stop", Reason: "attempt_budget_exhausted", Source: "global"}, events[2].Decision)
	assert.Equal(t, "channel_disable_requested", events[2].Health, "the health entry follows the automatic disable rules")
	common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, true)
	RecordPolicyFailure(c, 7, apiErr, DecideRelayRetry(c, apiErr, 0))
	assert.Equal(t, "key_disable_requested", state.Events()[4].Health)

	state.BeginAttempt(&model.Channel{Id: 8}, "default")
	c.Set("channel_id", 8)
	MarkRequestPolicySuccess(c, nil)
	MarkRequestPolicySuccess(c, nil)
	succeeded := model.NewLogOther()
	AppendRelayLogAdminInfo(c, nil, succeeded)
	events, ok = succeeded.Snapshot()["admin_info"].(map[string]any)["request_policy"].([]PolicyEvent)
	require.True(t, ok, "a successful relay exposes its decision events to administrators")
	require.Len(t, events, 7, "the outcome is recorded once")
	assert.Equal(t, PolicyDecision{Action: "success", Reason: "request_completed", Source: "upstream"}, events[6].Decision)
	assert.Equal(t, 8, events[6].ChannelID)
	assert.Equal(t, 2, events[6].Attempt)
	assert.True(t, state.Successful)

	untouched, _ := gin.CreateTestContext(httptest.NewRecorder())
	other := model.NewLogOther()
	AppendRelayLogAdminInfo(untouched, nil, other)
	assert.NotContains(t, other.Snapshot()["admin_info"], "request_policy", "requests without decisions do not carry an empty record")
}

func channelErrorRetryPolicy(rules ...kitdto.ChannelErrorRetryRule) *kitdto.ChannelErrorRetryPolicy {
	return &kitdto.ChannelErrorRetryPolicy{Enabled: true, Rules: rules}
}

func channelErrorRetryContext(policy *kitdto.ChannelErrorRetryPolicy) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	BeginChannelErrorRetryHTTP(c)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, kitdto.ChannelSettings{ErrorRetryPolicy: policy})
	return c
}

func upstreamHTTPError(status int, code, message string) *types.NewAPIError {
	err := types.NewOpenAIError(errors.New(message), types.ErrorCode(code), status)
	err.SetUpstreamHTTPError(status)
	return err
}

func TestBeginChannelErrorRetryHTTPEntryScope(t *testing.T) {
	for _, tc := range []struct {
		name    string
		method  string
		path    string
		upgrade string
		want    bool
	}{
		{name: "chat", method: http.MethodPost, path: "/v1/chat/completions", want: true},
		{name: "messages", method: http.MethodPost, path: "/v1/messages", want: true},
		{name: "responses", method: http.MethodPost, path: "/v1/responses", want: true},
		{name: "responses compact", method: http.MethodPost, path: "/v1/responses/compact", want: true},
		{name: "get excluded", method: http.MethodGet, path: "/v1/chat/completions", want: false},
		{name: "realtime excluded", method: http.MethodPost, path: "/v1/realtime", want: false},
		{name: "gemini excluded", method: http.MethodPost, path: "/v1beta/models/gemini:generateContent", want: false},
		{name: "websocket upgrade excluded", method: http.MethodPost, path: "/v1/responses", upgrade: "websocket", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(tc.method, tc.path, nil)
			if tc.upgrade != "" {
				c.Request.Header.Set("Upgrade", tc.upgrade)
			}
			BeginChannelErrorRetryHTTP(c)
			assert.Equal(t, tc.want, ChannelErrorRetryHTTPActive(c))
		})
	}
}

func TestDecideRelayRetryChannelRuleStopIsNotBypassed(t *testing.T) {
	stopRule := kitdto.ChannelErrorRetryRule{
		ID: "client-parameter", Enabled: true, Action: "stop", StatusCodes: []int{400},
		Conditions: []kitdto.ChannelErrorRetryCondition{{Field: "message", Operator: "contains", Value: "unsupported parameter"}},
	}

	t.Run("supplier channel:code early retry cannot bypass stop", func(t *testing.T) {
		c := channelErrorRetryContext(channelErrorRetryPolicy(stopRule))
		err := upstreamHTTPError(http.StatusBadRequest, "channel:invalid_key", "unsupported parameter")
		decision := DecideRelayRetry(c, err, 1)
		assert.Equal(t, "stop", decision.Action)
		assert.Equal(t, "channel_error_rule_stop", decision.Reason)
		assert.Equal(t, "channel_rule", decision.Source)
		assert.Equal(t, "client-parameter", decision.RuleID)
		require.NotNil(t, decision.Audit)
		assert.Equal(t, "matched", decision.Audit.Status)
		assert.Equal(t, http.StatusBadRequest, decision.Audit.UpstreamStatus)
	})

	t.Run("unrecognized effective status cannot bypass stop", func(t *testing.T) {
		c := channelErrorRetryContext(channelErrorRetryPolicy(kitdto.ChannelErrorRetryRule{
			ID: "network", Enabled: true, Action: "stop",
			Conditions: []kitdto.ChannelErrorRetryCondition{{Field: "message", Operator: "contains", Value: "temporary"}},
		}))
		err := upstreamHTTPError(http.StatusBadRequest, "", "temporary network error")
		err.StatusCode = 0 // effective status is unrecognized; the source stays a real upstream 400
		decision := DecideRelayRetry(c, err, 1)
		assert.Equal(t, "stop", decision.Action)
		assert.Equal(t, "channel_error_rule_stop", decision.Reason)
	})
}

func TestDecideRelayRetryChannelRuleRetryOverridesGlobal(t *testing.T) {
	rule := kitdto.ChannelErrorRetryRule{ID: "recoverable", Enabled: true, Action: "retry", StatusCodes: []int{400}}
	c := channelErrorRetryContext(channelErrorRetryPolicy(rule))
	err := upstreamHTTPError(http.StatusBadRequest, "", "temporary")
	decision := DecideRelayRetry(c, err, 1)
	assert.Equal(t, "retry", decision.Action)
	assert.Equal(t, "channel_error_rule_retry", decision.Reason)
	assert.Equal(t, "channel_rule", decision.Source)
	assert.Equal(t, "recoverable", decision.RuleID)
	require.NotNil(t, decision.Audit)
	assert.Equal(t, "matched", decision.Audit.Status)
}

func TestDecideRelayRetryChannelRuleRespectsHardLimits(t *testing.T) {
	rule := kitdto.ChannelErrorRetryRule{
		ID: "recoverable", Enabled: true, Action: "retry",
		Conditions: []kitdto.ChannelErrorRetryCondition{{Field: "message", Operator: "contains", Value: "temporary"}},
	}
	policy := channelErrorRetryPolicy(rule)

	assertLimited := func(t *testing.T, decision PolicyDecision, reason string) {
		t.Helper()
		assert.Equal(t, "stop", decision.Action)
		assert.Equal(t, reason, decision.Reason)
		require.NotNil(t, decision.Audit)
		assert.Equal(t, "match_limited", decision.Audit.Status)
		assert.Equal(t, "recoverable", decision.Audit.CandidateRuleID)
		assert.Empty(t, decision.RuleID)
	}

	t.Run("skip retry", func(t *testing.T) {
		c := channelErrorRetryContext(policy)
		err := types.NewOpenAIError(errors.New("temporary"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		err.SetUpstreamHTTPError(http.StatusBadRequest)
		assertLimited(t, DecideRelayRetry(c, err, 1), "non_retryable_error")
	})

	t.Run("attempt budget", func(t *testing.T) {
		c := channelErrorRetryContext(policy)
		assertLimited(t, DecideRelayRetry(c, upstreamHTTPError(http.StatusBadRequest, "", "temporary"), 0), "attempt_budget_exhausted")
	})

	t.Run("success status", func(t *testing.T) {
		c := channelErrorRetryContext(policy)
		err := upstreamHTTPError(http.StatusBadRequest, "", "temporary")
		err.StatusCode = http.StatusOK // a 400 source remapped to 200 still hits the 2xx exclusion
		assertLimited(t, DecideRelayRetry(c, err, 1), "system_retry_exclusion")
	})

	t.Run("always skip status", func(t *testing.T) {
		c := channelErrorRetryContext(policy)
		assertLimited(t, DecideRelayRetry(c, upstreamHTTPError(http.StatusGatewayTimeout, "", "temporary"), 1), "system_retry_exclusion")
	})

	t.Run("pin single attempt", func(t *testing.T) {
		c := channelErrorRetryContext(policy)
		GetChannelConstraints(c).AddPin(dto.ChannelPin{ChannelId: 1, Source: dto.PinSourceToken, Rank: dto.PinRankToken, RetryMode: dto.PinRetrySingleAttempt})
		assertLimited(t, DecideRelayRetry(c, upstreamHTTPError(http.StatusBadRequest, "", "temporary"), 1), "pinned_channel")
	})

	t.Run("committed response", func(t *testing.T) {
		c := channelErrorRetryContext(policy)
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.WriteHeaderNow()
		assertLimited(t, DecideRelayRetry(c, upstreamHTTPError(http.StatusBadRequest, "", "temporary"), 1), "response_committed")
	})
}

func TestDecideRelayRetryChannelPolicyInheritanceFallbacks(t *testing.T) {
	rule := kitdto.ChannelErrorRetryRule{
		ID: "stop-500", Enabled: true, Action: "stop", StatusCodes: []int{500},
		Conditions: []kitdto.ChannelErrorRetryCondition{{Field: "message", Operator: "contains", Value: "x"}},
	}
	policy := channelErrorRetryPolicy(rule)

	t.Run("miss keeps legacy decision with miss audit", func(t *testing.T) {
		c := channelErrorRetryContext(policy)
		decision := DecideRelayRetry(c, upstreamHTTPError(http.StatusBadRequest, "", "other"), 1)
		assert.Equal(t, "status_not_retryable", decision.Reason)
		assert.Empty(t, decision.RuleID)
		require.NotNil(t, decision.Audit)
		assert.Equal(t, "miss", decision.Audit.Status)
	})

	t.Run("disabled policy omits audit", func(t *testing.T) {
		c := channelErrorRetryContext(&kitdto.ChannelErrorRetryPolicy{Enabled: false, Rules: []kitdto.ChannelErrorRetryRule{rule}})
		decision := DecideRelayRetry(c, upstreamHTTPError(http.StatusBadRequest, "", "x"), 1)
		assert.Equal(t, "status_not_retryable", decision.Reason)
		assert.Nil(t, decision.Audit)
	})

	t.Run("out of scope omits audit", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)
		common.SetContextKey(c, constant.ContextKeyChannelSetting, kitdto.ChannelSettings{ErrorRetryPolicy: policy})
		decision := DecideRelayRetry(c, upstreamHTTPError(http.StatusBadRequest, "", "x"), 1)
		assert.Equal(t, "status_not_retryable", decision.Reason)
		assert.Nil(t, decision.Audit)
	})

	t.Run("error without upstream marker omits audit", func(t *testing.T) {
		c := channelErrorRetryContext(policy)
		err := types.NewOpenAIError(errors.New("x"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)
		decision := DecideRelayRetry(c, err, 1)
		assert.Equal(t, "status_not_retryable", decision.Reason)
		assert.Nil(t, decision.Audit)
	})

	t.Run("invalid policy inherits with bounded diagnostic", func(t *testing.T) {
		c := channelErrorRetryContext(nil)
		common.SetContextKey(c, constant.ContextKeyChannelSetting, kitdto.ChannelSettings{ErrorRetryPolicyDiagnostic: "error_retry_policy.rules[0].action: invalid_value"})
		decision := DecideRelayRetry(c, upstreamHTTPError(http.StatusBadRequest, "", "x"), 1)
		assert.Equal(t, "status_not_retryable", decision.Reason)
		require.NotNil(t, decision.Audit)
		assert.Equal(t, "invalid", decision.Audit.Status)
		assert.Contains(t, decision.Audit.Diagnostic, "error_retry_policy.rules[0].action")
	})
}

func TestDecideRelayRetryChannelPolicyUsesEffectiveStatus(t *testing.T) {
	err := upstreamHTTPError(http.StatusInternalServerError, "", "boom")
	err.SetUpstreamHTTPError(http.StatusBadRequest)

	matching := channelErrorRetryPolicy(kitdto.ChannelErrorRetryRule{ID: "s500", Enabled: true, Action: "stop", StatusCodes: []int{500}})
	decision := DecideRelayRetry(channelErrorRetryContext(matching), err, 1)
	assert.Equal(t, "s500", decision.RuleID, "the mapped effective status is matched")

	rawOnly := channelErrorRetryPolicy(kitdto.ChannelErrorRetryRule{ID: "s400", Enabled: true, Action: "stop", StatusCodes: []int{400}})
	decision = DecideRelayRetry(channelErrorRetryContext(rawOnly), err, 1)
	assert.Empty(t, decision.RuleID, "the raw upstream status is never matched")
	require.NotNil(t, decision.Audit)
	assert.Equal(t, "miss", decision.Audit.Status)
	// The audit must be self-describing: the raw upstream status and the
	// effective status the rules were matched against, so an administrator can
	// configure status_codes from the log without having to guess which one it is.
	assert.Equal(t, http.StatusBadRequest, decision.Audit.UpstreamStatus)
	assert.Equal(t, http.StatusInternalServerError, decision.Audit.MatchedStatus)
}

func TestMatchChannelErrorRetryTruncationSemantics(t *testing.T) {
	stopContains := kitdto.ChannelErrorRetryRule{
		ID: "stop-late", Enabled: true, Action: "stop",
		Conditions: []kitdto.ChannelErrorRetryCondition{{Field: "message", Operator: "contains", Value: "late-marker"}},
	}
	retryStatusOnly := kitdto.ChannelErrorRetryRule{ID: "retry-any", Enabled: true, Action: "retry", StatusCodes: []int{400}}

	t.Run("truncated prefix without marker is input_incomplete and does not reach later retry", func(t *testing.T) {
		policy := channelErrorRetryPolicy(stopContains, retryStatusOnly)
		result := MatchChannelErrorRetry(policy, ChannelErrorRetryView{
			StatusCode: 400, Message: strings.Repeat("a", MaxChannelErrorRetryFieldBytes), MessageTruncated: true,
		})
		assert.Equal(t, ChannelErrorRetryInputIncomplete, result.Status)
	})

	t.Run("prefix contains the stop keyword matches stop", func(t *testing.T) {
		policy := channelErrorRetryPolicy(stopContains, retryStatusOnly)
		result := MatchChannelErrorRetry(policy, ChannelErrorRetryView{
			StatusCode: 400, Message: strings.Repeat("a", 10) + "late-marker", MessageTruncated: true,
		})
		require.Equal(t, ChannelErrorRetryMatched, result.Status)
		assert.Equal(t, "stop-late", result.RuleID)
	})

	t.Run("not_contains prefix proves false and falls through", func(t *testing.T) {
		notContains := kitdto.ChannelErrorRetryRule{
			ID: "stop-not", Enabled: true, Action: "stop", StatusCodes: []int{400},
			Conditions: []kitdto.ChannelErrorRetryCondition{{Field: "message", Operator: "not_contains", Value: "keyword"}},
		}
		policy := channelErrorRetryPolicy(notContains, retryStatusOnly)
		result := MatchChannelErrorRetry(policy, ChannelErrorRetryView{StatusCode: 400, Message: "xxkeywordxx", MessageTruncated: true})
		require.Equal(t, ChannelErrorRetryMatched, result.Status)
		assert.Equal(t, "retry-any", result.RuleID)
	})

	t.Run("equals on a truncated field is decidable false and falls through", func(t *testing.T) {
		equalsRule := kitdto.ChannelErrorRetryRule{
			ID: "stop-eq", Enabled: true, Action: "stop",
			Conditions: []kitdto.ChannelErrorRetryCondition{{Field: "message", Operator: "equals", Value: "x"}},
		}
		result := MatchChannelErrorRetry(channelErrorRetryPolicy(equalsRule, retryStatusOnly), ChannelErrorRetryView{
			StatusCode: 400, Message: "x", MessageTruncated: true,
		})
		require.Equal(t, ChannelErrorRetryMatched, result.Status)
		assert.Equal(t, "retry-any", result.RuleID)
	})

	t.Run("empty field never matches not_contains", func(t *testing.T) {
		notContains := kitdto.ChannelErrorRetryRule{
			ID: "stop-not", Enabled: true, Action: "stop",
			Conditions: []kitdto.ChannelErrorRetryCondition{{Field: "message", Operator: "not_contains", Value: "keyword"}},
		}
		result := MatchChannelErrorRetry(channelErrorRetryPolicy(notContains), ChannelErrorRetryView{Message: ""})
		assert.Equal(t, ChannelErrorRetryMiss, result.Status)
	})
}

func TestDecideRelayRetryTruncatedPolicyInherits(t *testing.T) {
	stopLate := kitdto.ChannelErrorRetryRule{
		ID: "stop-late", Enabled: true, Action: "stop",
		Conditions: []kitdto.ChannelErrorRetryCondition{{Field: "message", Operator: "contains", Value: "late-marker"}},
	}
	retryStatus := kitdto.ChannelErrorRetryRule{ID: "retry-any", Enabled: true, Action: "retry", StatusCodes: []int{500}}

	c := channelErrorRetryContext(channelErrorRetryPolicy(stopLate, retryStatus))
	err := upstreamHTTPError(http.StatusInternalServerError, "", "temporary")
	err.SetMessage(strings.Repeat("a", MaxChannelErrorRetryFieldBytes+128))

	decision := DecideRelayRetry(c, err, 1)
	assert.NotEqual(t, "channel_error_rule_retry", decision.Reason, "an uncertain policy must not reach a later retry rule")
	require.NotNil(t, decision.Audit)
	assert.Equal(t, "input_incomplete", decision.Audit.Status)

	legacy := decideRelayRetryLegacy(c, err, 1)
	assert.Equal(t, legacy.Action, decision.Action)
	assert.Equal(t, legacy.Reason, decision.Reason)
}

func TestChannelErrorRetryDecisionAuditReachesLog(t *testing.T) {
	c := channelErrorRetryContext(channelErrorRetryPolicy(kitdto.ChannelErrorRetryRule{
		ID: "recoverable", Enabled: true, Action: "retry", StatusCodes: []int{400},
	}))
	err := upstreamHTTPError(http.StatusBadRequest, "", "temporary")
	decision := DecideRelayRetry(c, err, 1)
	RecordPolicyFailure(c, 7, err, decision)

	other := model.NewLogOther()
	AppendRelayLogAdminInfo(c, nil, other)
	events, ok := other.Snapshot()["admin_info"].(map[string]any)["request_policy"].([]PolicyEvent)
	require.True(t, ok)
	require.NotEmpty(t, events)
	last := events[len(events)-1]
	assert.Equal(t, "recoverable", last.Decision.RuleID)
	require.NotNil(t, last.Decision.Audit)
	assert.Equal(t, "matched", last.Decision.Audit.Status)
	assert.Equal(t, http.StatusBadRequest, last.UpstreamStatus)
}

// TestChannelErrorRetryPolicyMatchesRelayErrorHandlerOutput exercises the real
// source: the parsed upstream HTTP error feeds the decision with the message
// before display mapping and the wire status code.
func TestChannelErrorRetryPolicyMatchesRelayErrorHandlerOutput(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body: io.NopCloser(strings.NewReader(
			`{"error":{"message":"rate limited by upstream","type":"rate_limit_error","code":"rate_limit"}}`,
		)),
	}
	err := RelayErrorHandler(context.Background(), resp, false)
	require.NotNil(t, err)

	c := channelErrorRetryContext(channelErrorRetryPolicy(kitdto.ChannelErrorRetryRule{
		ID: "rate-limit", Enabled: true, Action: "retry", StatusCodes: []int{http.StatusTooManyRequests},
		Conditions: []kitdto.ChannelErrorRetryCondition{
			{Field: "message", Operator: "contains", Value: "rate limited"},
		},
	}))
	decision := DecideRelayRetry(c, err, 1)
	assert.Equal(t, "retry", decision.Action)
	assert.Equal(t, "channel_error_rule_retry", decision.Reason)
	assert.Equal(t, "rate-limit", decision.RuleID)
}

// TestDecideRelayRetryTypeViewIsEntryIndependent covers the review's exact
// Claude-shaped upstream body on every supported entry: the type condition must
// see the standardized OpenAI-normalized type, not the Claude projection of a
// non-Claude error (which used to yield the literal "<nil>" on /v1/messages).
func TestDecideRelayRetryTypeViewIsEntryIndependent(t *testing.T) {
	for _, entry := range []string{"/v1/messages", "/v1/chat/completions", "/v1/responses"} {
		t.Run(entry, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body: io.NopCloser(strings.NewReader(
					`{"type":"error","error":{"type":"overloaded_error","message":"supplier fault"}}`,
				)),
			}
			err := RelayErrorHandler(context.Background(), resp, false)
			require.NotNil(t, err)
			require.Equal(t, "overloaded_error", err.ToOpenAIError().Type)

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, entry, nil)
			BeginChannelErrorRetryHTTP(c)
			common.SetContextKey(c, constant.ContextKeyChannelSetting, kitdto.ChannelSettings{
				ErrorRetryPolicy: channelErrorRetryPolicy(kitdto.ChannelErrorRetryRule{
					ID: "by-type", Enabled: true, Action: "stop", StatusCodes: []int{http.StatusInternalServerError},
					Conditions: []kitdto.ChannelErrorRetryCondition{
						{Field: "type", Operator: "equals", Value: "overloaded_error"},
					},
				}),
			})

			decision := DecideRelayRetry(c, err, 1)
			assert.Equal(t, "channel_error_rule_stop", decision.Reason)
			assert.Equal(t, "by-type", decision.RuleID)
			require.NotNil(t, decision.Audit)
			assert.Equal(t, "overloaded_error", decision.Audit.RetryErrorType)
		})
	}
}

// TestDecideRelayRetryIgnoresNonErrorSourceStatus guards the original 400-599
// source range independently of the effective status used for matching.
func TestDecideRelayRetryIgnoresNonErrorSourceStatus(t *testing.T) {
	c := channelErrorRetryContext(channelErrorRetryPolicy(kitdto.ChannelErrorRetryRule{
		ID: "redirect", Enabled: true, Action: "retry", StatusCodes: []int{http.StatusFound},
	}))
	decision := DecideRelayRetry(c, upstreamHTTPError(http.StatusFound, "", "redirect"), 1)
	assert.NotEqual(t, "channel_error_rule_retry", decision.Reason)
}
