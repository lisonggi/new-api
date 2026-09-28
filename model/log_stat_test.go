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
