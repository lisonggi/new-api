package dto

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdvancedCustomValidateResponsesToChatConverterPath(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
			},
		},
	}
	require.NoError(t, valid.Validate())

	validGemini := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
			},
		},
	}
	require.NoError(t, validGemini.Validate())

	tests := []struct {
		name         string
		incomingPath string
	}{
		{name: "chat completions", incomingPath: "/v1/chat/completions"},
		{name: "responses compact", incomingPath: "/v1/responses/compact"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &AdvancedCustomConfig{
				Routes: []AdvancedCustomRoute{
					{
						IncomingPath: tt.incomingPath,
						UpstreamPath: "/v1/chat/completions",
						Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
					},
				},
			}
			err := config.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "converter does not match incoming_path")
		})
	}
}

func TestAdvancedCustomValidateModelListRouteConstraints(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: AdvancedCustomModelListPath,
				UpstreamPath: "https://upstream.example/custom/models",
				Converter:    advancedCustomConverterNone,
			},
		},
	}
	require.NoError(t, valid.Validate())

	tests := []struct {
		name   string
		routes []AdvancedCustomRoute
		want   string
	}{
		{
			name: "model matching rules",
			routes: []AdvancedCustomRoute{
				{
					IncomingPath: AdvancedCustomModelListPath,
					UpstreamPath: "/v1/models",
					Models:       []string{"gpt-4o"},
				},
			},
			want: "models must be empty",
		},
		{
			name: "converter",
			routes: []AdvancedCustomRoute{
				{
					IncomingPath: AdvancedCustomModelListPath,
					UpstreamPath: "/v1/models",
					Converter:    advancedCustomConverterOpenAIChatToOpenAIResponses,
				},
			},
			want: "converter must be none",
		},
		{
			name: "model placeholder",
			routes: []AdvancedCustomRoute{
				{
					IncomingPath: AdvancedCustomModelListPath,
					UpstreamPath: "/v1/models/{model}",
				},
			},
			want: "upstream_path must not contain {model}",
		},
		{
			name: "duplicate routes",
			routes: []AdvancedCustomRoute{
				{IncomingPath: AdvancedCustomModelListPath, UpstreamPath: "/v1/models"},
				{IncomingPath: AdvancedCustomModelListPath, UpstreamPath: "/provider/models"},
			},
			want: "duplicates the /v1/models route",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&AdvancedCustomConfig{Routes: tt.routes}).Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestAdvancedCustomModelListRouteRequiresExactIncomingPath(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/{model}",
				UpstreamPath: "/generic/{model}",
			},
			{
				IncomingPath: AdvancedCustomModelListPath,
				UpstreamPath: "/provider/models",
			},
		},
	}
	require.NoError(t, config.Validate())

	route, ok := config.ModelListRoute()
	require.True(t, ok)
	assert.Equal(t, "/provider/models", route.UpstreamPath)
}

func TestAdvancedCustomValidateBalanceRouteConstraints(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{{
			IncomingPath: AdvancedCustomBalancePath,
			UpstreamPath: "/provider/balance",
			Converter:    advancedCustomConverterNone,
		}},
	}
	require.NoError(t, valid.Validate())

	route, ok := valid.BalanceRoute()
	require.True(t, ok)
	assert.Equal(t, "/provider/balance", route.UpstreamPath)

	tests := []struct {
		name   string
		routes []AdvancedCustomRoute
		want   string
	}{
		{
			name: "model matching rules",
			routes: []AdvancedCustomRoute{{
				IncomingPath: AdvancedCustomBalancePath,
				UpstreamPath: "/provider/balance",
				Models:       []string{"gpt-4o"},
			}},
			want: "models must be empty",
		},
		{
			name: "converter",
			routes: []AdvancedCustomRoute{{
				IncomingPath: AdvancedCustomBalancePath,
				UpstreamPath: "/provider/balance",
				Converter:    advancedCustomConverterOpenAIChatToOpenAIResponses,
			}},
			want: "converter must be none",
		},
		{
			name: "model placeholder",
			routes: []AdvancedCustomRoute{{
				IncomingPath: AdvancedCustomBalancePath,
				UpstreamPath: "/provider/{model}/balance",
			}},
			want: "upstream_path must not contain {model}",
		},
		{
			name: "duplicate routes",
			routes: []AdvancedCustomRoute{
				{IncomingPath: AdvancedCustomBalancePath, UpstreamPath: "/provider/balance"},
				{IncomingPath: AdvancedCustomBalancePath, UpstreamPath: "/provider/credits"},
			},
			want: "duplicates the /v1/dashboard/billing/credit_grants route",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&AdvancedCustomConfig{Routes: tt.routes}).Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestAdvancedCustomValidateDuplicateIncomingPathWithDisjointModels(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
				Models:       []string{"gpt-4o"},
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
				Models:       []string{"gemini-2.5-flash"},
			},
		},
	}

	require.NoError(t, config.Validate())
}

func TestAdvancedCustomValidateDuplicateIncomingPathRejectsOverlappingModels(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
				Models:       []string{"shared-model"},
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
				Models:       []string{"shared-model"},
			},
		},
	}

	err := config.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "models overlaps")
}

func TestAdvancedCustomValidateDuplicateIncomingPathRejectsMultipleCatchAllRoutes(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
			},
		},
	}

	err := config.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "catch-all already exists")
}

func TestAdvancedCustomValidateDuplicateIncomingPathRequiresCatchAllLast(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
				Models:       []string{"gemini-2.5-flash"},
			},
		},
	}

	err := config.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "catch-all route must be last")
}

func TestAdvancedCustomMatchPathForModel(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
				Models:       []string{"gemini-2.5-flash"},
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
				Models:       []string{"gpt-4o"},
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/responses",
				Converter:    advancedCustomConverterNone,
			},
		},
	}
	require.NoError(t, config.Validate())

	geminiRoute, ok := config.MatchPathForModel("/v1/responses", "gemini-2.5-flash")
	require.True(t, ok)
	assert.Equal(t, advancedCustomConverterOpenAIResponsesToGemini, geminiRoute.Converter)

	chatRoute, ok := config.MatchPathForModel("/v1/responses", "gpt-4o")
	require.True(t, ok)
	assert.Equal(t, advancedCustomConverterOpenAIResponsesToOpenAIChat, chatRoute.Converter)

	fallbackRoute, ok := config.MatchPathForModel("/v1/responses", "unknown-model")
	require.True(t, ok)
	assert.Equal(t, advancedCustomConverterNone, fallbackRoute.Converter)
}

func TestAdvancedCustomMatchPathForModelRegexRules(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
				Models:       []string{"re:^gemini-"},
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
				Models:       []string{"re:(?i)^OAI-"},
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/responses",
				Converter:    advancedCustomConverterNone,
			},
		},
	}
	require.NoError(t, config.Validate())

	geminiRoute, ok := config.MatchPathForModel("/v1/responses", "gemini-2.5-flash")
	require.True(t, ok)
	assert.Equal(t, advancedCustomConverterOpenAIResponsesToGemini, geminiRoute.Converter)

	chatRoute, ok := config.MatchPathForModel("/v1/responses", "oai-test")
	require.True(t, ok)
	assert.Equal(t, advancedCustomConverterOpenAIResponsesToOpenAIChat, chatRoute.Converter)

	fallbackRoute, ok := config.MatchPathForModel("/v1/responses", "gpt-4o")
	require.True(t, ok)
	assert.Equal(t, advancedCustomConverterNone, fallbackRoute.Converter)
}

func TestAdvancedCustomRouteModelRegexRulesAreCachedCompiled(t *testing.T) {
	require.True(t, matchAdvancedCustomRouteModelRule("re:^cache-probe-", "cache-probe-model"))

	cached, ok := advancedCustomModelRegexCache.Load("^cache-probe-")
	require.True(t, ok)
	require.NotNil(t, cached)
	_, isRegexp := cached.(*regexp.Regexp)
	require.True(t, isRegexp)

	// Invalid patterns never match and are cached as nil so they are not recompiled.
	require.False(t, matchAdvancedCustomRouteModelRule("re:(", "anything"))
	cached, ok = advancedCustomModelRegexCache.Load("(")
	require.True(t, ok)
	re, _ := cached.(*regexp.Regexp)
	require.Nil(t, re)

	// Cached entries keep matching correctly on subsequent calls.
	require.True(t, matchAdvancedCustomRouteModelRule("re:^cache-probe-", "cache-probe-other"))
	require.False(t, matchAdvancedCustomRouteModelRule("re:^cache-probe-", "other-model"))
}

func TestAdvancedCustomMatchPathForModelExactRuleDoesNotMatchPrefix(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
				Models:       []string{"gemini"},
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/responses",
				Converter:    advancedCustomConverterNone,
			},
		},
	}
	require.NoError(t, config.Validate())

	fallbackRoute, ok := config.MatchPathForModel("/v1/responses", "gemini-2.5-flash")
	require.True(t, ok)
	assert.Equal(t, advancedCustomConverterNone, fallbackRoute.Converter)
}

func TestAdvancedCustomValidateDuplicateIncomingPathRejectsInvalidRegexModels(t *testing.T) {
	tests := []struct {
		name   string
		models []string
		want   string
	}{
		{name: "empty regex", models: []string{"re:"}, want: "regex is empty"},
		{name: "invalid regex", models: []string{"re:["}, want: "regex is invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &AdvancedCustomConfig{
				Routes: []AdvancedCustomRoute{
					{
						IncomingPath: "/v1/responses",
						UpstreamPath: "/v1beta/models/{model}:generateContent",
						Converter:    advancedCustomConverterOpenAIResponsesToGemini,
						Models:       tt.models,
					},
				},
			}

			err := config.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestAdvancedCustomValidateDuplicateIncomingPathRejectsDuplicateRegexModels(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
				Models:       []string{"re:^gemini-"},
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
				Models:       []string{"re:^gemini-"},
			},
		},
	}

	err := config.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "models overlaps")
}

func TestAdvancedCustomMatchPathForModelUsesFirstMatchingRegexRoute(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
				Models:       []string{"re:^gemini-"},
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
				Models:       []string{"gemini-2.5-flash"},
			},
		},
	}
	require.NoError(t, config.Validate())

	route, ok := config.MatchPathForModel("/v1/responses", "gemini-2.5-flash")
	require.True(t, ok)
	assert.Equal(t, advancedCustomConverterOpenAIResponsesToGemini, route.Converter)
}

func TestAdvancedCustomSupportedEndpointTypesForModel(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
				Models:       []string{"re:^gemini-"},
			},
			{
				IncomingPath: "/v1beta/models/{model}:generateContent",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Models:       []string{"re:^gemini-"},
			},
			{
				IncomingPath: "/v1beta/models/{model}:streamGenerateContent",
				UpstreamPath: "/v1beta/models/{model}:streamGenerateContent",
				Models:       []string{"re:^gemini-"},
			},
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "/v1/chat/completions",
				Models:       []string{"gpt-4o"},
			},
			{
				IncomingPath: "/v1/messages",
				UpstreamPath: "/v1/messages",
			},
			{
				IncomingPath: "/custom/endpoint",
				UpstreamPath: "/custom/endpoint",
			},
		},
	}
	require.NoError(t, config.Validate())

	assert.Equal(t, []types.EndpointType{
		types.EndpointTypeOpenAIResponse,
		types.EndpointTypeGemini,
		types.EndpointTypeAnthropic,
	}, config.SupportedEndpointTypesForModel("gemini-2.5-flash"))
	assert.Equal(t, []types.EndpointType{
		types.EndpointTypeOpenAI,
		types.EndpointTypeAnthropic,
	}, config.SupportedEndpointTypesForModel("gpt-4o"))
	assert.Equal(t, []types.EndpointType{
		types.EndpointTypeAnthropic,
	}, config.SupportedEndpointTypesForModel("other-model"))
}

func TestAdvancedCustomValidateAlphaSearchConverterPath(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/alpha/search",
				UpstreamPath: "/v1/alpha/search",
				Converter:    advancedCustomConverterNone,
			},
		},
	}
	require.NoError(t, valid.Validate())
	assert.Equal(t, []types.EndpointType{
		types.EndpointTypeOpenAIAlphaSearch,
	}, valid.SupportedEndpointTypesForModel("gpt-5.1"))

	nonNoneConverters := []string{
		advancedCustomConverterClaudeMessagesToOpenAIChat,
		advancedCustomConverterOpenAIChatToClaudeMessages,
		advancedCustomConverterOpenAIChatToOpenAIResponses,
		advancedCustomConverterOpenAIResponsesToOpenAIChat,
		advancedCustomConverterOpenAIResponsesToGemini,
		advancedCustomConverterGeminiContentToOpenAIChat,
		advancedCustomConverterOpenAIChatToGeminiContent,
	}
	for _, converter := range nonNoneConverters {
		t.Run(converter, func(t *testing.T) {
			config := &AdvancedCustomConfig{
				Routes: []AdvancedCustomRoute{
					{
						IncomingPath: "/v1/alpha/search",
						UpstreamPath: "/v1/alpha/search",
						Converter:    converter,
					},
				},
			}
			err := config.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "converter does not match incoming_path")
		})
	}
}

func TestAdvancedCustomValidateRoutePassThroughBody(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath:           "/v1/chat/completions",
				UpstreamPath:           "/v1/chat/completions",
				PassThroughBodyEnabled: true,
			},
			{
				IncomingPath:           "/v1/rerank",
				UpstreamPath:           "/v1/rerank",
				Converter:              AdvancedCustomConverterSGLangRerank,
				PassThroughBodyEnabled: true,
			},
			{
				IncomingPath: "/v1/messages",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterClaudeMessagesToOpenAIChat,
			},
		},
	}
	require.NoError(t, valid.Validate())

	route, ok := valid.MatchPathForModel("/v1/chat/completions", "gpt-4o")
	require.True(t, ok)
	assert.True(t, route.PassThroughBodyEnabled)
	route, ok = valid.MatchPathForModel("/v1/messages", "gpt-4o")
	require.True(t, ok)
	assert.False(t, route.PassThroughBodyEnabled)

	encoded, err := json.Marshal(valid.Routes[2])
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "pass_through_body_enabled")

	tests := []struct {
		name  string
		route AdvancedCustomRoute
		want  string
	}{
		{
			name: "converter route",
			route: AdvancedCustomRoute{
				IncomingPath:           "/v1/messages",
				UpstreamPath:           "/v1/chat/completions",
				Converter:              advancedCustomConverterClaudeMessagesToOpenAIChat,
				PassThroughBodyEnabled: true,
			},
			want: "pass_through_body_enabled requires converter none",
		},
		{
			name: "model list route",
			route: AdvancedCustomRoute{
				IncomingPath:           AdvancedCustomModelListPath,
				UpstreamPath:           "/v1/models",
				PassThroughBodyEnabled: true,
			},
			want: "pass_through_body_enabled must be false for /v1/models",
		},
		{
			name: "balance route",
			route: AdvancedCustomRoute{
				IncomingPath:           AdvancedCustomBalancePath,
				UpstreamPath:           "/provider/balance",
				PassThroughBodyEnabled: true,
			},
			want: "pass_through_body_enabled must be false for /v1/dashboard/billing/credit_grants",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&AdvancedCustomConfig{Routes: []AdvancedCustomRoute{tt.route}}).Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestChannelSettingsHTTPTransportJSONRoundTrip(t *testing.T) {
	legacy := `{"proxy":"http://127.0.0.1:8080","force_format":true}`
	var settings ChannelSettings
	require.NoError(t, json.Unmarshal([]byte(legacy), &settings))
	assert.Equal(t, "http://127.0.0.1:8080", settings.Proxy)
	assert.True(t, settings.ForceFormat)
	assert.Empty(t, settings.HTTPProtocol)
	assert.Zero(t, settings.HTTP2ConnectionShards)

	encoded, err := json.Marshal(settings)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "http_protocol")
	assert.NotContains(t, string(encoded), "http2_connection_shards")

	explicit := ChannelSettings{
		Proxy:                 "socks5://127.0.0.1:1080",
		HTTPProtocol:          HTTPProtocolHTTP1,
		HTTP2ConnectionShards: 1,
	}
	encoded, err = json.Marshal(explicit)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"http_protocol":"http1"`)

	var decoded ChannelSettings
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, explicit.HTTPProtocol, decoded.HTTPProtocol)
	assert.Equal(t, 1, decoded.HTTP2ConnectionShards)

	sharded := ChannelSettings{HTTP2ConnectionShards: 4}
	encoded, err = json.Marshal(sharded)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"http2_connection_shards":4`)
	assert.NotContains(t, string(encoded), "http_protocol")
}

func TestChannelSettingsValidateHTTPTransport(t *testing.T) {
	require.NoError(t, (&ChannelSettings{}).ValidateHTTPTransport())
	require.NoError(t, (&ChannelSettings{HTTPProtocol: "AUTO"}).ValidateHTTPTransport())
	require.NoError(t, (&ChannelSettings{HTTPProtocol: "http1"}).ValidateHTTPTransport())
	require.NoError(t, (&ChannelSettings{HTTP2ConnectionShards: 8}).ValidateHTTPTransport())

	err := (&ChannelSettings{HTTPProtocol: "http2"}).ValidateHTTPTransport()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http_protocol")

	err = (&ChannelSettings{HTTP2ConnectionShards: -1}).ValidateHTTPTransport()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http2_connection_shards")

	err = (&ChannelSettings{HTTP2ConnectionShards: 9}).ValidateHTTPTransport()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http2_connection_shards")

	err = (&ChannelSettings{HTTPProtocol: "http1", HTTP2ConnectionShards: 2}).ValidateHTTPTransport()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http2_connection_shards")
}

func TestChannelOtherSettingsValidateToolLossPolicy(t *testing.T) {
	require.NoError(t, (*ChannelOtherSettings)(nil).ValidateToolLossPolicy())
	require.NoError(t, (&ChannelOtherSettings{}).ValidateToolLossPolicy())
	require.NoError(t, (&ChannelOtherSettings{ToolLossPolicy: "allow"}).ValidateToolLossPolicy())
	require.NoError(t, (&ChannelOtherSettings{ToolLossPolicy: "safe"}).ValidateToolLossPolicy())
	require.NoError(t, (&ChannelOtherSettings{ToolLossPolicy: "strict"}).ValidateToolLossPolicy())

	err := (&ChannelOtherSettings{ToolLossPolicy: "drop"}).ValidateToolLossPolicy()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool_loss_policy")
}

func TestChannelSettingsResolveFirstResponseTimeout(t *testing.T) {
	settings := ChannelSettings{
		ModelFirstResponseTimeout: map[string][]FirstResponseTimeoutTier{
			"deepseek-v4.1": {
				{ContextTokens: 32000, TimeoutMs: 1000},
				{ContextTokens: 200000, TimeoutMs: 3000},
				{ContextTokens: 1000000, TimeoutMs: 5000},
			},
			"claude-sonnet": {
				{ContextTokens: 200000, TimeoutMs: 2500},
			},
		},
	}

	tests := []struct {
		name         string
		model        string
		promptTokens int
		want         int
	}{
		{name: "first tier", model: "deepseek-v4.1", promptTokens: 16000, want: 1000},
		{name: "boundary of first tier", model: "deepseek-v4.1", promptTokens: 32000, want: 1000},
		{name: "middle tier", model: "deepseek-v4.1", promptTokens: 100000, want: 3000},
		{name: "boundary of middle tier", model: "deepseek-v4.1", promptTokens: 200000, want: 3000},
		{name: "last tier", model: "deepseek-v4.1", promptTokens: 500000, want: 5000},
		{name: "beyond last tier falls back to last", model: "deepseek-v4.1", promptTokens: 2000000, want: 5000},
		{name: "single tier model", model: "claude-sonnet", promptTokens: 100, want: 2500},
		{name: "unconfigured model", model: "gpt-4o", promptTokens: 100, want: 0},
		{name: "unknown context returns zero", model: "deepseek-v4.1", promptTokens: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, settings.ResolveFirstResponseTimeout(tt.model, tt.promptTokens))
		})
	}

	assert.Equal(t, 0, ChannelSettings{}.ResolveFirstResponseTimeout("any", 100))
}

func TestChannelSettingsResolveFirstResponseTimeoutToleratesUnsortedTiers(t *testing.T) {
	settings := ChannelSettings{
		ModelFirstResponseTimeout: map[string][]FirstResponseTimeoutTier{
			"deepseek-v4.1": {
				{ContextTokens: 200000, TimeoutMs: 3000},
				{ContextTokens: 32000, TimeoutMs: 1000},
			},
		},
	}

	// 16K fits both tiers; the tightest (smallest) context bound must win, even
	// though the tiers are stored out of order.
	assert.Equal(t, 1000, settings.ResolveFirstResponseTimeout("deepseek-v4.1", 16000))
	// 250K exceeds every bound and falls back to the largest (catch-all) tier.
	assert.Equal(t, 3000, settings.ResolveFirstResponseTimeout("deepseek-v4.1", 250000))
}

func TestChannelSettingsResolveFirstResponseTimeoutClampsOverflow(t *testing.T) {
	settings := ChannelSettings{
		ModelFirstResponseTimeout: map[string][]FirstResponseTimeoutTier{
			"legacy": {
				// 18446744073710 ms overflows time.Duration when multiplied by
				// time.Millisecond; the resolver must clamp it.
				{ContextTokens: 200000, TimeoutMs: 18446744073710},
			},
		},
	}
	assert.Equal(t, MaxFirstResponseTimeoutMs, settings.ResolveFirstResponseTimeout("legacy", 100))
}

func TestChannelSettingsValidateFirstResponseTimeout(t *testing.T) {
	require.NoError(t, (&ChannelSettings{}).ValidateFirstResponseTimeout())
	require.NoError(t, (&ChannelSettings{
		ModelFirstResponseTimeout: map[string][]FirstResponseTimeoutTier{
			"m": {{ContextTokens: 32000, TimeoutMs: 1000}, {ContextTokens: 200000, TimeoutMs: 3000}},
		},
	}).ValidateFirstResponseTimeout())

	tests := []struct {
		name  string
		tiers []FirstResponseTimeoutTier
		want  string
	}{
		{
			name:  "unsorted tiers",
			tiers: []FirstResponseTimeoutTier{{ContextTokens: 200000, TimeoutMs: 3000}, {ContextTokens: 32000, TimeoutMs: 1000}},
			want:  "strictly increasing",
		},
		{
			name:  "duplicate context bound",
			tiers: []FirstResponseTimeoutTier{{ContextTokens: 32000, TimeoutMs: 1000}, {ContextTokens: 32000, TimeoutMs: 2000}},
			want:  "strictly increasing",
		},
		{
			name:  "non-positive context bound",
			tiers: []FirstResponseTimeoutTier{{ContextTokens: 0, TimeoutMs: 1000}},
			want:  "context_tokens must be positive",
		},
		{
			name:  "non-positive timeout",
			tiers: []FirstResponseTimeoutTier{{ContextTokens: 32000, TimeoutMs: 0}},
			want:  "timeout_ms must be positive",
		},
		{
			name:  "negative timeout",
			tiers: []FirstResponseTimeoutTier{{ContextTokens: 32000, TimeoutMs: -5}},
			want:  "timeout_ms must be positive",
		},
		{
			name:  "overflow timeout",
			tiers: []FirstResponseTimeoutTier{{ContextTokens: 32000, TimeoutMs: 18446744073710}},
			want:  "timeout_ms exceeds maximum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&ChannelSettings{
				ModelFirstResponseTimeout: map[string][]FirstResponseTimeoutTier{"m": tt.tiers},
			}).ValidateFirstResponseTimeout()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestParseChannelErrorRetryPolicyInSettingAcceptsValidPolicy(t *testing.T) {
	raw := `{"proxy":"http://127.0.0.1:8080","error_retry_policy":{"enabled":true,"rules":[` +
		`{"id":"r1","name":"overloaded","enabled":true,"action":"retry","status_codes":[429,503],` +
		`"conditions":[{"field":"message","operator":"contains","value":"overloaded","case_sensitive":false}]},` +
		`{"id":"r2","enabled":true,"action":"stop","conditions":[{"field":"code","operator":"equals","value":"content_filter","case_sensitive":true}]}` +
		`]}}`

	policy, present, verr := ParseChannelErrorRetryPolicyInSetting([]byte(raw))
	require.Nil(t, verr)
	require.True(t, present)
	require.NotNil(t, policy)
	assert.True(t, policy.Enabled)
	require.Len(t, policy.Rules, 2)
	assert.Equal(t, "r1", policy.Rules[0].ID)
	assert.Equal(t, ChannelErrorRetryActionRetry, policy.Rules[0].Action)
	assert.Equal(t, []int{429, 503}, policy.Rules[0].StatusCodes)
	require.Len(t, policy.Rules[0].Conditions, 1)
	assert.True(t, policy.Rules[1].Enabled)
	assert.Equal(t, ChannelErrorRetryActionStop, policy.Rules[1].Action)
}

func TestParseChannelErrorRetryPolicyInSettingInheritsWhenAbsentOrNull(t *testing.T) {
	for name, raw := range map[string]string{
		"absent": `{"proxy":"http://127.0.0.1:8080"}`,
		"null":   `{"error_retry_policy":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			policy, present, verr := ParseChannelErrorRetryPolicyInSetting([]byte(raw))
			require.Nil(t, verr)
			require.False(t, present)
			require.Nil(t, policy)
		})
	}
}

func TestParseChannelErrorRetryPolicyInSettingAcceptsEmptyObjectAsInherit(t *testing.T) {
	policy, present, verr := ParseChannelErrorRetryPolicyInSetting([]byte(`{"error_retry_policy":{}}`))
	require.Nil(t, verr)
	require.True(t, present)
	require.NotNil(t, policy)
	assert.False(t, policy.Enabled)
	assert.Empty(t, policy.Rules)
}

func TestParseChannelErrorRetryPolicyInSettingRejectsInvalid(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "non-canonical key",
			raw:  `{"ERROR_RETRY_POLICY":{"enabled":true}}`,
			want: "non_canonical_field",
		},
		{
			name: "duplicate canonical key",
			raw:  `{"error_retry_policy":{"enabled":true},"error_retry_policy":{"enabled":false}}`,
			want: "non_canonical_field",
		},
		{
			name: "unknown top-level policy field",
			raw:  `{"error_retry_policy":{"enabled":true,"mode":"x"}}`,
			want: "unknown_field",
		},
		{
			name: "null enabled",
			raw:  `{"error_retry_policy":{"enabled":null}}`,
			want: "not_bool",
		},
		{
			name: "policy not object",
			raw:  `{"error_retry_policy":"retry"}`,
			want: "not_object",
		},
		{
			name: "rule missing action",
			raw:  `{"error_retry_policy":{"rules":[{"id":"r1","status_codes":[500]}]}}`,
			want: "action",
		},
		{
			name: "enabled policy carries rules without saying whether it is enabled",
			raw:  `{"error_retry_policy":{"rules":[{"id":"r1","enabled":true,"action":"retry","status_codes":[500]}]}}`,
			want: "required",
		},
		{
			name: "invalid action",
			raw:  `{"error_retry_policy":{"rules":[{"id":"r1","action":"inherit","status_codes":[500]}]}}`,
			want: "invalid_value",
		},
		{
			name: "empty matcher",
			raw:  `{"error_retry_policy":{"rules":[{"id":"r1","action":"retry"}]}}`,
			want: "empty_matcher",
		},
		{
			name: "status out of range",
			raw:  `{"error_retry_policy":{"rules":[{"id":"r1","action":"retry","status_codes":[600]}]}}`,
			want: "invalid_value",
		},
		{
			name: "non-integer status",
			raw:  `{"error_retry_policy":{"rules":[{"id":"r1","action":"retry","status_codes":[429.5]}]}}`,
			want: "invalid_value",
		},
		{
			name: "duplicate status",
			raw:  `{"error_retry_policy":{"rules":[{"id":"r1","action":"retry","status_codes":[429,429]}]}}`,
			want: "duplicate",
		},
		{
			name: "duplicate rule id",
			raw:  `{"error_retry_policy":{"rules":[{"id":"r1","action":"retry","status_codes":[500]},{"id":"r1","action":"stop","status_codes":[400]}]}}`,
			want: "duplicate",
		},
		{
			name: "invalid rule id",
			raw:  `{"error_retry_policy":{"rules":[{"id":"bad id","action":"retry","status_codes":[500]}]}}`,
			want: "invalid_value",
		},
		{
			name: "unknown condition operator",
			raw:  `{"error_retry_policy":{"rules":[{"id":"r1","action":"retry","conditions":[{"field":"message","operator":"regex","value":"x"}]}]}}`,
			want: "invalid_value",
		},
		{
			name: "unknown condition field",
			raw:  `{"error_retry_policy":{"rules":[{"id":"r1","action":"retry","conditions":[{"field":"body","operator":"equals","value":"x"}]}]}}`,
			want: "invalid_value",
		},
		{
			name: "blank condition value",
			raw:  `{"error_retry_policy":{"rules":[{"id":"r1","action":"retry","conditions":[{"field":"message","operator":"equals","value":"   "}]}]}}`,
			want: "invalid_value",
		},
		{
			name: "null condition value",
			raw:  `{"error_retry_policy":{"rules":[{"id":"r1","action":"retry","conditions":[{"field":"message","operator":"equals","value":null}]}]}}`,
			want: "not_string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy, present, verr := ParseChannelErrorRetryPolicyInSetting([]byte(tt.raw))
			require.NotNil(t, verr)
			require.False(t, present)
			require.Nil(t, policy)
			assert.Contains(t, verr.Error(), tt.want)
		})
	}
}

func TestParseChannelErrorRetryPolicyInSettingRejectsDuplicateKeysInSubtree(t *testing.T) {
	raw := `{"error_retry_policy":{"enabled":true,"enabled":false}}`
	_, _, verr := ParseChannelErrorRetryPolicyInSetting([]byte(raw))
	require.NotNil(t, verr)
	assert.Contains(t, verr.Error(), "invalid_json")
}

func TestChannelErrorRetryPolicyTypedRoundTrip(t *testing.T) {
	settings := ChannelSettings{
		ErrorRetryPolicy: &ChannelErrorRetryPolicy{
			Enabled: true,
			Rules: []ChannelErrorRetryRule{{
				ID:          "rule_a",
				Enabled:     true,
				Action:      ChannelErrorRetryActionRetry,
				StatusCodes: []int{429},
			}},
		},
	}
	encoded, err := json.Marshal(settings)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"error_retry_policy"`)
	assert.NotContains(t, string(encoded), "error_retry_policy_diagnostic")

	decoded, present, verr := ParseChannelErrorRetryPolicyInSetting(encoded)
	require.Nil(t, verr)
	require.True(t, present)
	require.Equal(t, settings.ErrorRetryPolicy, decoded)
}
