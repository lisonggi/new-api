package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelBatchEditTest(t *testing.T) *gorm.DB {
	t.Helper()
	wasMaster := common.IsMasterNode
	common.IsMasterNode = true
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	previousDB, previousLogDB := model.DB, model.LOG_DB
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, database.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.CasbinRule{}, &model.AuthzRole{}, &model.Log{}, &model.AuditLog{}, &model.User{}))
	model.DB = database
	model.LOG_DB = database
	require.NoError(t, authz.Init(database))
	t.Cleanup(func() {
		common.IsMasterNode = wasMaster
		common.RedisEnabled = previousRedisEnabled
		model.DB = previousDB
		model.LOG_DB = previousLogDB
	})
	return database
}

func callChannelBatchHandler(t *testing.T, handler func(*gin.Context), userID, role int, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set("id", userID)
	context.Set("role", role)
	context.Request = httptest.NewRequest(http.MethodPut, "/api/channel/batch", strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	handler(context)
	return recorder
}

func seedBatchEditChannel(t *testing.T, database *gorm.DB) model.Channel {
	t.Helper()
	channel := model.Channel{Type: 1, Key: "key", Name: "batch-channel", Status: common.ChannelStatusEnabled, Models: "gpt-4o", Group: "default"}
	require.NoError(t, database.Create(&channel).Error)
	return channel
}

func TestEditChannelBatchRejectsInvalidPayloads(t *testing.T) {
	setupChannelBatchEditTest(t)
	channel := seedBatchEditChannel(t, model.DB)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"empty ids", `{"ids":[],"models":"gpt-4o"}`, "参数错误"},
		{"no attributes", `{"ids":[1]}`, "参数错误"},
		{"empty settings object", `{"ids":[1],"settings":{}}`, "参数错误"},
		{"unknown models mode", fmt.Sprintf(`{"ids":[%d],"models":"gpt-4o","models_mode":"bogus"}`, channel.Id), "参数错误"},
		{"models mode merge", fmt.Sprintf(`{"ids":[%d],"models":"gpt-4o","models_mode":"merge"}`, channel.Id), "参数错误"},
		{"invalid proxy", fmt.Sprintf(`{"ids":[%d],"proxy":"ftp://nope"}`, channel.Id), "代理地址格式错误"},
		{"invalid shards", fmt.Sprintf(`{"ids":[%d],"http2_connection_shards":99}`, channel.Id), "HTTP/2 分片数量"},
		{"invalid retry policy", fmt.Sprintf(`{"ids":[%d],"settings":{"error_retry_policy":{"enabled":true,"unknown":1}}}`, channel.Id), "错误重试判断设置错误"},
		{"invalid timeout tier", fmt.Sprintf(`{"ids":[%d],"settings":{"model_first_response_timeout":{"gpt-4o":[{"context_tokens":0,"timeout_ms":5}]}}}`, channel.Id), "首字超时设置错误"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := callChannelBatchHandler(t, EditChannelBatch, 1, common.RoleRootUser, tc.body)
			assert.Contains(t, recorder.Body.String(), tc.want)
			assert.Contains(t, recorder.Body.String(), `"success":false`)
		})
	}
}

func TestEditChannelBatchSettingsRequiresSensitiveWrite(t *testing.T) {
	setupChannelBatchEditTest(t)
	channel := seedBatchEditChannel(t, model.DB)
	settingsBody := fmt.Sprintf(`{"ids":[%d],"settings":{"system_prompt":"hi"}}`, channel.Id)

	// An admin holds ChannelWrite but not ChannelSensitiveWrite.
	denied := callChannelBatchHandler(t, EditChannelBatch, 2, common.RoleAdminUser, settingsBody)
	assert.Contains(t, denied.Body.String(), `"success":false`)
	assert.NotContains(t, denied.Body.String(), `"success":true`)

	// The same admin may still edit a non-sensitive column.
	allowed := callChannelBatchHandler(t, EditChannelBatch, 2, common.RoleAdminUser, fmt.Sprintf(`{"ids":[%d],"models":"gpt-4o,claude-3"}`, channel.Id))
	assert.Contains(t, allowed.Body.String(), `"success":true`)

	// Root may change settings.
	granted := callChannelBatchHandler(t, EditChannelBatch, 1, common.RoleRootUser, settingsBody)
	assert.Contains(t, granted.Body.String(), `"success":true`)
	var reloaded model.Channel
	require.NoError(t, model.DB.First(&reloaded, channel.Id).Error)
	assert.Equal(t, "hi", reloaded.GetSetting().SystemPrompt)
}

func TestEditTagChannelsSettingsRequiresSensitiveWrite(t *testing.T) {
	setupChannelBatchEditTest(t)
	channel := seedBatchEditChannel(t, model.DB)
	tag := "batch-tag"
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", channel.Id).Update("tag", tag).Error)
	body := `{"tag":"batch-tag","settings":{"thinking_to_content":true}}`

	denied := callChannelBatchHandler(t, EditTagChannels, 2, common.RoleAdminUser, body)
	assert.Contains(t, denied.Body.String(), `"success":false`)

	granted := callChannelBatchHandler(t, EditTagChannels, 1, common.RoleRootUser, body)
	assert.Contains(t, granted.Body.String(), `"success":true`)
	var reloaded model.Channel
	require.NoError(t, model.DB.First(&reloaded, channel.Id).Error)
	assert.True(t, reloaded.GetSetting().ThinkingToContent)
}
