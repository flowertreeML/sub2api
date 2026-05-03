package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/provider/zhipu"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type zhipuHTTPUpstreamRecorder struct {
	lastReq  *http.Request
	lastBody []byte
	resp     *http.Response
}

func (u *zhipuHTTPUpstreamRecorder) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return u.DoWithTLS(req, "", 0, 0, nil)
}

func (u *zhipuHTTPUpstreamRecorder) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.lastReq = req
	if req != nil && req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		u.lastBody = b
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(b))
	}
	return u.resp, nil
}

type zhipuRoundTripUpstream struct {
	lastReq *http.Request
}

func (u *zhipuRoundTripUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return u.DoWithTLS(req, "", 0, 0, nil)
}

func (u *zhipuRoundTripUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.lastReq = req
	return http.DefaultTransport.RoundTrip(req)
}

func newZhipuTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c, w
}

func newZhipuTestAccount(baseURL string) *Account {
	credentials := map[string]any{"api_key": "test-zhipu-key"}
	if baseURL != "" {
		credentials["base_url"] = baseURL
	}
	return &Account{
		ID:          101,
		Platform:    PlatformZhipu,
		Type:        AccountTypeAPIKey,
		Credentials: credentials,
		Concurrency: 1,
	}
}

func TestZhipuChatCompletions_NonStream_BaseURLDefault(t *testing.T) {
	upstream := &zhipuHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"chatcmpl-default",
			"model":"glm-4.6",
			"usage":{"prompt_tokens":3,"completion_tokens":5,"total_tokens":8}
		}`)),
	}}
	svc := &GatewayService{httpUpstream: upstream}
	c, w := newZhipuTestContext()

	result, err := svc.ZhipuChatCompletions(context.Background(), c, newZhipuTestAccount(""), []byte(`{"model":"glm-4.6"}`), false)

	require.NoError(t, err)
	require.Equal(t, zhipu.DefaultBaseURL+"/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 3, result.Usage.InputTokens)
	require.Equal(t, 5, result.Usage.OutputTokens)
}

func TestZhipuChatCompletions_NonStream_BaseURLOverride(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-override","model":"glm-4.6","usage":{"prompt_tokens":1,"completion_tokens":2}}`))
	}))
	defer server.Close()
	upstream := &zhipuRoundTripUpstream{}
	svc := &GatewayService{httpUpstream: upstream}
	c, w := newZhipuTestContext()

	result, err := svc.ZhipuChatCompletions(context.Background(), c, newZhipuTestAccount(server.URL), []byte(`{"model":"glm-4.6"}`), false)

	require.NoError(t, err)
	require.Equal(t, "/chat/completions", gotPath)
	require.Equal(t, server.URL+"/chat/completions", upstream.lastReq.URL.String())
	require.JSONEq(t, `{"id":"chatcmpl-override","model":"glm-4.6","usage":{"prompt_tokens":1,"completion_tokens":2}}`, w.Body.String())
	require.Equal(t, 1, result.Usage.InputTokens)
	require.Equal(t, 2, result.Usage.OutputTokens)
}

func TestZhipuChatCompletions_Stream_PassthroughChunks(t *testing.T) {
	chunks := "data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\"你\"}}]}\n\n" +
		"data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\n" +
		"data: [DONE]\n\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(chunks))
	}))
	defer server.Close()
	svc := &GatewayService{httpUpstream: &zhipuRoundTripUpstream{}}
	c, w := newZhipuTestContext()

	result, err := svc.ZhipuChatCompletions(context.Background(), c, newZhipuTestAccount(server.URL), []byte(`{"model":"glm-4.6","stream":true}`), true)

	require.NoError(t, err)
	require.True(t, result.Stream)
	require.Equal(t, chunks, w.Body.String())
}

func TestZhipuChatCompletions_Stream_ExtractUsage(t *testing.T) {
	chunks := "data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n" +
		"data: {\"id\":\"1\",\"choices\":[],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":11,\"total_tokens\":18}}\n\n" +
		"data: [DONE]\n\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(chunks))
	}))
	defer server.Close()
	svc := &GatewayService{httpUpstream: &zhipuRoundTripUpstream{}}
	c, _ := newZhipuTestContext()

	result, err := svc.ZhipuChatCompletions(context.Background(), c, newZhipuTestAccount(server.URL), []byte(`{"model":"glm-4.6","stream":true}`), true)

	require.NoError(t, err)
	require.Equal(t, 7, result.Usage.InputTokens)
	require.Equal(t, 11, result.Usage.OutputTokens)
}

func TestZhipuChatCompletions_AuthHeader(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-auth","model":"glm-4.6","usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	}))
	defer server.Close()
	svc := &GatewayService{httpUpstream: &zhipuRoundTripUpstream{}}
	c, _ := newZhipuTestContext()

	_, err := svc.ZhipuChatCompletions(context.Background(), c, newZhipuTestAccount(server.URL), []byte(`{"model":"glm-4.6"}`), false)

	require.NoError(t, err)
	require.Equal(t, "Bearer test-zhipu-key", gotAuth)
}
