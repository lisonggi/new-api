package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

// HTTPTransportPolicy is the runtime-normalized outbound HTTP transport policy
// for a channel. Unknown or out-of-range stored values are clamped safely.
type HTTPTransportPolicy struct {
	Protocol string // dto.HTTPProtocolAuto or dto.HTTPProtocolHTTP1
	Shards   int    // 1..dto.MaxHTTP2ConnectionShards
	// ResponseHeaderTimeout overrides the transport's response-header
	// (time-to-first-byte) timeout. Zero means use the global
	// RELAY_RESPONSE_HEADER_TIMEOUT default.
	ResponseHeaderTimeout time.Duration
}

var httpTransportPolicyWarnings sync.Map

func defaultHTTPTransportPolicy() HTTPTransportPolicy {
	return HTTPTransportPolicy{
		Protocol: dto.HTTPProtocolAuto,
		Shards:   1,
	}
}

// NormalizeHTTPTransportPolicy converts channel settings into a safe runtime policy.
// Invalid stored values never panic; they clamp to defaults and warn once per bad value.
func NormalizeHTTPTransportPolicy(settings dto.ChannelSettings) HTTPTransportPolicy {
	policy := defaultHTTPTransportPolicy()

	protocol := strings.ToLower(strings.TrimSpace(settings.HTTPProtocol))
	switch protocol {
	case "", dto.HTTPProtocolAuto:
		policy.Protocol = dto.HTTPProtocolAuto
	case dto.HTTPProtocolHTTP1:
		policy.Protocol = dto.HTTPProtocolHTTP1
	default:
		warnHTTPTransportPolicyOnce("http_protocol", settings.HTTPProtocol)
		policy.Protocol = dto.HTTPProtocolAuto
	}

	shards := settings.HTTP2ConnectionShards
	switch {
	case shards == 0:
		policy.Shards = 1
	case shards < 1:
		warnHTTPTransportPolicyOnce("http2_connection_shards", fmt.Sprintf("%d", shards))
		policy.Shards = 1
	case shards > dto.MaxHTTP2ConnectionShards:
		warnHTTPTransportPolicyOnce("http2_connection_shards", fmt.Sprintf("%d", shards))
		policy.Shards = dto.MaxHTTP2ConnectionShards
	default:
		policy.Shards = shards
	}

	if policy.Protocol == dto.HTTPProtocolHTTP1 {
		if settings.HTTP2ConnectionShards > 1 {
			warnHTTPTransportPolicyOnce(
				"http_protocol+http2_connection_shards",
				fmt.Sprintf("%s+%d", dto.HTTPProtocolHTTP1, settings.HTTP2ConnectionShards),
			)
		}
		policy.Shards = 1
	}
	if policy.Shards < 1 {
		policy.Shards = 1
	}
	return policy
}

func warnHTTPTransportPolicyOnce(field, value string) {
	key := field + "=" + value
	if _, loaded := httpTransportPolicyWarnings.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	logger.LogWarn(
		context.Background(),
		fmt.Sprintf("invalid channel http transport setting clamped: %s=%q", field, value),
	)
}

func (p HTTPTransportPolicy) cacheKeyPart() string {
	return fmt.Sprintf("%s|%d|%d", p.Protocol, p.Shards, p.ResponseHeaderTimeout.Milliseconds())
}

// WithResponseHeaderTimeout returns a copy of the policy with the given
// response-header (time-to-first-byte) timeout applied.
func (p HTTPTransportPolicy) WithResponseHeaderTimeout(timeout time.Duration) HTTPTransportPolicy {
	p.ResponseHeaderTimeout = timeout
	return p
}

func (p HTTPTransportPolicy) String() string {
	return p.cacheKeyPart()
}
