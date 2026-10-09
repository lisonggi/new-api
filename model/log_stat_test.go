package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCacheReadTokens(t *testing.T) {
	cases := []struct {
		name  string
		other string
		want  int64
	}{
		{"cache read tokens", `{"cache_tokens":300,"model_ratio":1}`, 300},
		{"zero cache tokens", `{"cache_tokens":0}`, 0},
		{"missing key", `{"model_ratio":1}`, 0},
		{"empty string", "", 0},
		{"invalid json", "{oops", 0},
		{"string encoded number", `{"cache_tokens":"300"}`, 300},
		{"non numeric value", `{"cache_tokens":"abc"}`, 0},
		{"nested admin info only", `{"admin_info":{"cache_tokens":900}}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseCacheReadTokens(tc.other))
		})
	}
}

func TestSumUsedQuotaAggregatesCacheHitStats(t *testing.T) {
	truncateTables(t)

	seed := []Log{
		{UserId: 1, Username: "alice", CreatedAt: 1500, Type: LogTypeConsume, PromptTokens: 1000, CompletionTokens: 50, Quota: 10, Other: `{"cache_tokens":300}`},
		{UserId: 1, Username: "alice", CreatedAt: 1600, Type: LogTypeConsume, PromptTokens: 2000, CompletionTokens: 60, Quota: 20, Other: `{"model_ratio":1}`},
		{UserId: 1, Username: "alice", CreatedAt: 1700, Type: LogTypeConsume, PromptTokens: 500, CompletionTokens: 70, Quota: 30, Other: ""},
		{UserId: 1, Username: "alice", CreatedAt: 2500, Type: LogTypeConsume, PromptTokens: 9999, CompletionTokens: 80, Quota: 40, Other: `{"cache_tokens":700}`},
		{UserId: 1, Username: "alice", CreatedAt: 1500, Type: LogTypeTopup, PromptTokens: 777, Quota: 50, Other: `{"cache_tokens":700}`},
	}
	for i := range seed {
		require.NoError(t, DB.Create(&seed[i]).Error)
	}

	stat, err := SumUsedQuota(LogTypeUnknown, 1000, 2000, "", "alice", "", 0, "")
	require.NoError(t, err)
	assert.Equal(t, 60, stat.Quota)
	assert.Equal(t, int64(3500), stat.PromptTokens)
	assert.Equal(t, int64(300), stat.CacheTokens)
}

// Adversarial: every filter (model/token/channel/group/time/type/username) must
// apply identically to the SQL prompt_tokens sum and the streamed cache rows,
// and the cache parser must keep its documented edge semantics.
func TestSumUsedQuotaStreamingMatchesFilteredAggregation(t *testing.T) {
	truncateTables(t)

	seed := []Log{
		{UserId: 1, Username: "bob", TokenName: "tk", ModelName: "m1", ChannelId: 7, Group: "g1", CreatedAt: 1100, Type: LogTypeConsume, PromptTokens: 100, CompletionTokens: 10, Quota: 1, Other: `{"cache_tokens":11}`},
		{UserId: 1, Username: "bob", TokenName: "tk", ModelName: "m1", ChannelId: 7, Group: "g1", CreatedAt: 1200, Type: LogTypeConsume, PromptTokens: 200, CompletionTokens: 20, Quota: 2, Other: `{"cache_tokens":"22"}`},
		{UserId: 1, Username: "bob", TokenName: "tk", ModelName: "m1", ChannelId: 7, Group: "g1", CreatedAt: 1300, Type: LogTypeConsume, PromptTokens: 300, CompletionTokens: 30, Quota: 3, Other: `{"model_ratio":1}`},
		{UserId: 1, Username: "bob", TokenName: "tk", ModelName: "m1", ChannelId: 7, Group: "g1", CreatedAt: 1400, Type: LogTypeConsume, PromptTokens: 400, CompletionTokens: 40, Quota: 4, Other: ""},
		{UserId: 1, Username: "bob", TokenName: "tk", ModelName: "m1", ChannelId: 7, Group: "g1", CreatedAt: 1450, Type: LogTypeConsume, PromptTokens: 450, CompletionTokens: 45, Quota: 5, Other: `{"cache_tokens":"not-a-number"}`},
		{UserId: 1, Username: "bob", TokenName: "tk", ModelName: "m1", ChannelId: 7, Group: "g1", CreatedAt: 1480, Type: LogTypeConsume, PromptTokens: 480, CompletionTokens: 48, Quota: 6, Other: `{"cache_tokens":60.5}`},
		{UserId: 1, Username: "bob", TokenName: "tk", ModelName: "m1", ChannelId: 7, Group: "g1", CreatedAt: 2100, Type: LogTypeConsume, PromptTokens: 9000, CompletionTokens: 90, Quota: 9, Other: `{"cache_tokens":900}`},
		{UserId: 1, Username: "carol", TokenName: "tk", ModelName: "m1", ChannelId: 7, Group: "g1", CreatedAt: 1150, Type: LogTypeConsume, PromptTokens: 7000, CompletionTokens: 70, Quota: 7, Other: `{"cache_tokens":700}`},
		{UserId: 1, Username: "bob", TokenName: "tk", ModelName: "m1", ChannelId: 7, Group: "g1", CreatedAt: 1150, Type: LogTypeTopup, PromptTokens: 8000, Quota: 8, Other: `{"cache_tokens":800}`},
		{UserId: 1, Username: "bob", TokenName: "tk", ModelName: "m2", ChannelId: 7, Group: "g1", CreatedAt: 1150, Type: LogTypeConsume, PromptTokens: 6000, CompletionTokens: 60, Quota: 6, Other: `{"cache_tokens":600}`},
		{UserId: 1, Username: "bob", TokenName: "tk", ModelName: "m1", ChannelId: 8, Group: "g1", CreatedAt: 1150, Type: LogTypeConsume, PromptTokens: 5000, CompletionTokens: 50, Quota: 5, Other: `{"cache_tokens":500}`},
		{UserId: 1, Username: "bob", TokenName: "tk", ModelName: "m1", ChannelId: 7, Group: "g2", CreatedAt: 1150, Type: LogTypeConsume, PromptTokens: 4000, CompletionTokens: 40, Quota: 4, Other: `{"cache_tokens":400}`},
		{UserId: 1, Username: "bob", TokenName: "tk2", ModelName: "m1", ChannelId: 7, Group: "g1", CreatedAt: 1150, Type: LogTypeConsume, PromptTokens: 3000, CompletionTokens: 30, Quota: 3, Other: `{"cache_tokens":300}`},
	}
	for i := range seed {
		require.NoError(t, DB.Create(&seed[i]).Error)
	}

	stat, err := SumUsedQuota(LogTypeUnknown, 1000, 1999, "m1", "bob", "tk", 7, "g1")
	require.NoError(t, err)
	assert.Equal(t, 1+2+3+4+5+6, stat.Quota)
	assert.Equal(t, int64(100+200+300+400+450+480), stat.PromptTokens)
	assert.Equal(t, int64(11+22), stat.CacheTokens)
}

// Adversarial: a legacy row with NULL `other` must not abort the whole stat.
func TestSumUsedQuotaToleratesNullOther(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&Log{
		UserId: 1, Username: "nul", CreatedAt: 1200, Type: LogTypeConsume,
		PromptTokens: 10, Quota: 1, Other: `{"cache_tokens":5}`,
	}).Error)
	require.NoError(t, DB.Exec(
		"INSERT INTO logs (created_at, type, username, prompt_tokens, quota, other) VALUES (?, ?, ?, ?, ?, NULL)",
		1300, LogTypeConsume, "nul", 20, 2,
	).Error)

	stat, err := SumUsedQuota(LogTypeUnknown, 1000, 2000, "", "nul", "", 0, "")
	require.NoError(t, err)
	assert.Equal(t, 3, stat.Quota)
	assert.Equal(t, int64(30), stat.PromptTokens)
	assert.Equal(t, int64(5), stat.CacheTokens)
}
