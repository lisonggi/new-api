package channel

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTaskAPIRequestInheritsClientCancellation(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	requestContext, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(requestContext)

	upstream, err := newTaskAPIRequest(c, "https://provider.example/tasks", nil)
	require.NoError(t, err)
	cancel()

	require.ErrorIs(t, upstream.Context().Err(), context.Canceled)
}

func TestProcessHeaderOverride_ChannelTestSkipsPassthroughRules(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"*": "",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Empty(t, headers)
}

func TestProcessHeaderOverride_ChannelTestSkipsClientHeaderPlaceholder(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Upstream-Trace": "{client_header:X-Trace-Id}",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	_, ok := headers["x-upstream-trace"]
	require.False(t, ok)
}

func TestProcessHeaderOverride_NonTestKeepsClientHeaderPlaceholder(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Upstream-Trace": "{client_header:X-Trace-Id}",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "trace-123", headers["x-upstream-trace"])
}

func TestProcessHeaderOverride_RuntimeOverrideIsFinalHeaderMap(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{
		IsChannelTest:             false,
		UseRuntimeHeadersOverride: true,
		RuntimeHeadersOverride: map[string]any{
			"x-static":  "runtime-value",
			"x-runtime": "runtime-only",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Static": "legacy-value",
				"X-Legacy": "legacy-only",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "runtime-value", headers["x-static"])
	require.Equal(t, "runtime-only", headers["x-runtime"])
	_, exists := headers["x-legacy"]
	require.False(t, exists)
}

func TestProcessHeaderOverride_PassthroughSkipsAcceptEncoding(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")
	ctx.Request.Header.Set("Accept-Encoding", "gzip")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"*": "",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "trace-123", headers["x-trace-id"])

	_, hasAcceptEncoding := headers["accept-encoding"]
	require.False(t, hasAcceptEncoding)
}

func TestProcessHeaderOverride_PassHeadersTemplateSetsRuntimeHeaders(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	ctx.Request.Header.Set("Originator", "Codex CLI")
	ctx.Request.Header.Set("Session_id", "sess-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		RequestHeaders: map[string]string{
			"Originator": "Codex CLI",
			"Session_id": "sess-123",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ParamOverride: map[string]any{
				"operations": []any{
					map[string]any{
						"mode":  "pass_headers",
						"value": []any{"Originator", "Session_id", "X-Codex-Beta-Features"},
					},
				},
			},
			HeadersOverride: map[string]any{
				"X-Static": "legacy-value",
			},
		},
	}

	_, err := relaycommon.ApplyParamOverrideWithRelayInfo([]byte(`{"model":"gpt-4.1"}`), info)
	require.NoError(t, err)
	require.True(t, info.UseRuntimeHeadersOverride)
	require.Equal(t, "Codex CLI", info.RuntimeHeadersOverride["originator"])
	require.Equal(t, "sess-123", info.RuntimeHeadersOverride["session_id"])
	_, exists := info.RuntimeHeadersOverride["x-codex-beta-features"]
	require.False(t, exists)
	require.Equal(t, "legacy-value", info.RuntimeHeadersOverride["x-static"])

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "Codex CLI", headers["originator"])
	require.Equal(t, "sess-123", headers["session_id"])
	_, exists = headers["x-codex-beta-features"]
	require.False(t, exists)

	upstreamReq := httptest.NewRequest(http.MethodPost, "https://example.com/v1/responses", nil)
	applyHeaderOverrideToRequest(upstreamReq, headers)
	require.Equal(t, "Codex CLI", upstreamReq.Header.Get("Originator"))
	require.Equal(t, "sess-123", upstreamReq.Header.Get("Session_id"))
	require.Empty(t, upstreamReq.Header.Get("X-Codex-Beta-Features"))
}

func TestToWebSocketURL(t *testing.T) {
	for input, want := range map[string]string{
		"https://api.openai.com/v1/responses":             "wss://api.openai.com/v1/responses",
		"http://127.0.0.1:3000/v1/responses":              "ws://127.0.0.1:3000/v1/responses",
		"wss://chatgpt.com/backend-api/codex/responses":   "wss://chatgpt.com/backend-api/codex/responses",
		"ws://127.0.0.1:3000/backend-api/codex/responses": "ws://127.0.0.1:3000/backend-api/codex/responses",
	} {
		assert.Equal(t, want, toWebSocketURL(input), input)
	}
}

// hangBody blocks on Read until Close is called, simulating an upstream that
// returns headers but never sends the first body byte.
type hangBody struct {
	release chan struct{}
	once    sync.Once
}

func (h *hangBody) Read([]byte) (int, error) {
	<-h.release
	return 0, io.EOF
}

func (h *hangBody) Close() error {
	h.once.Do(func() { close(h.release) })
	return nil
}

func TestAwaitFirstByteReturnsFirstByteAndPreservesStream(t *testing.T) {
	resp := &http.Response{Body: io.NopCloser(strings.NewReader("hello"))}
	require.NoError(t, awaitFirstByte(resp, time.Second))

	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))
}

func TestAwaitFirstByteEmptyBodyIsNotTimeout(t *testing.T) {
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(""))}
	require.NoError(t, awaitFirstByte(resp, time.Second))
}

func TestAwaitFirstByteTimesOutAndClosesBody(t *testing.T) {
	body := &hangBody{release: make(chan struct{})}
	resp := &http.Response{Body: body}

	err := awaitFirstByte(resp, 20*time.Millisecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "first response byte not received")

	select {
	case <-body.release:
	default:
		t.Fatal("body was not closed on timeout")
	}
}

// An auth service (e.g. chat2api) can answer a request with a 401 challenge
// before the eventual 200. The first-byte guard must treat the 401 as the
// response of this attempt (not hang waiting for a later 200) and keep its
// body intact.
func TestAwaitFirstByteErrorResponsePreservesBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(strings.NewReader(`{"error":"unauthorized"}`)),
	}
	require.NoError(t, awaitFirstByte(resp, time.Second))

	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"error":"unauthorized"}`, string(got))
}

// A quick 401 challenge satisfies the transport's ResponseHeaderTimeout and the
// first-byte guard alike; neither should falsely fire on the 401.
func TestResponseHeaderTimeoutSatisfiedByQuick401(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="chat2api"`)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer upstream.Close()

	client := &http.Client{
		Transport: &http.Transport{ResponseHeaderTimeout: 50 * time.Millisecond},
	}
	req, err := http.NewRequest(http.MethodPost, upstream.URL, strings.NewReader(`{"model":"m"}`))
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	require.NoError(t, awaitFirstByte(resp, 50*time.Millisecond))
	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"error":"unauthorized"}`, string(got))
}

// TestAwaitFirstByteNilBodyIsNotTimeout guards against a response whose Body is
// nil (e.g. HEAD-style or a 401 with no body); the guard must return immediately.
func TestAwaitFirstByteNilBodyIsNotTimeout(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusUnauthorized, Body: http.NoBody}
	require.NoError(t, awaitFirstByte(resp, time.Second))
}

// TestResolveFirstResponseTimeoutGatesOnStreaming verifies the first-byte timeout
// is a streaming-only concept: a non-streaming request must never arm it, even
// when a per-model threshold is configured and prompt tokens are known.
func TestResolveFirstResponseTimeoutGatesOnStreaming(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	threshold := kitdto.ChannelSettings{
		ModelFirstResponseTimeout: map[string][]kitdto.FirstResponseTimeoutTier{
			"gpt-4": {{ContextTokens: 4096, TimeoutMs: 5000}},
		},
	}
	newInfo := func(isStream bool, meta *relaycommon.ChannelMeta) *relaycommon.RelayInfo {
		info := &relaycommon.RelayInfo{IsStream: isStream, OriginModelName: "gpt-4", TokenGroup: "default"}
		if meta != nil {
			info.ChannelMeta = meta
		}
		info.SetEstimatePromptTokens(1000)
		return info
	}

	// Non-streaming never arms the timeout, even with a configured threshold and
	// known prompt tokens.
	require.Zero(t, resolveFirstResponseTimeout(c, newInfo(false, &relaycommon.ChannelMeta{ChannelSetting: threshold})))

	// Streaming without channel meta → 0.
	require.Zero(t, resolveFirstResponseTimeout(c, newInfo(true, nil)))

	// Streaming without a threshold for the model → 0.
	require.Zero(t, resolveFirstResponseTimeout(c, newInfo(true, &relaycommon.ChannelMeta{})))
}

// TestUpstream401Then200OnSameConnection documents what actually happens when an
// auth service writes a 401 and then a 200 for the same request. Go's HTTP/1.1
// client returns the FIRST response (401); the trailing 200 bytes stay buffered
// for the next request on the keep-alive connection. The first-byte guard reads
// the 401's (empty) body and never hangs waiting for the 200.
func TestUpstream401Then200OnSameConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf) // drain the request line + headers
		_, _ = conn.Write([]byte("HTTP/1.1 401 Unauthorized\r\nContent-Length: 0\r\n\r\n"))
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nhello"))
	}()

	client := &http.Client{
		Transport: &http.Transport{ResponseHeaderTimeout: time.Second},
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+listener.Addr().String(), strings.NewReader(`{"model":"m"}`))
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	require.NoError(t, awaitFirstByte(resp, time.Second))
	select {
	case <-serverDone:
	case <-time.After(time.Second):
		t.Fatal("server goroutine did not finish")
	}
}
