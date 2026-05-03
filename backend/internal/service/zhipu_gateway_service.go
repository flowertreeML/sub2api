package service

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/provider/zhipu"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// ZhipuChatCompletions 直转智谱 OpenAI 兼容 Chat Completions。
// 智谱 GLM 不支持 Responses API，因此这里禁止进入 Chat→Responses 转换链路。
func (s *GatewayService) ZhipuChatCompletions(ctx context.Context, c *gin.Context, account *Account, body []byte, isStream bool) (*ForwardResult, error) {
	startTime := time.Now()
	originalModel := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	upstreamModel := originalModel
	if account != nil {
		if mapped := strings.TrimSpace(account.GetMappedModel(originalModel)); mapped != "" {
			upstreamModel = mapped
		}
	}
	if upstreamModel != originalModel {
		body = ReplaceModelInBody(body, upstreamModel)
	}

	upstreamReq, err := buildZhipuChatCompletionsRequest(ctx, account, body, isStream)
	if err != nil {
		writeGatewayCCError(c, http.StatusInternalServerError, "api_error", "Failed to build request")
		return nil, err
	}

	proxyURL := ""
	if account != nil && account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	accountID, accountConcurrency := int64(0), 0
	if account != nil {
		accountID = account.ID
		accountConcurrency = account.Concurrency
	}
	resp, err := s.httpUpstream.DoWithTLS(upstreamReq, proxyURL, accountID, accountConcurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		writeGatewayCCError(c, http.StatusBadGateway, "server_error", "Upstream request failed")
		return nil, fmt.Errorf("zhipu upstream request failed: %s", safeErr)
	}
	defer func() { _ = resp.Body.Close() }()

	requestID := resp.Header.Get("x-request-id")

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		// 智谱错误响应按上游状态码和 body 原样透传，不做 OpenAI/Responses 错误改写。
		s.writeZhipuPassthroughHeaders(c, resp)
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
		return nil, fmt.Errorf("zhipu upstream error: %d", resp.StatusCode)
	}

	if isStream {
		return s.handleZhipuStreamingResponse(resp, c, originalModel, upstreamModel, requestID, startTime)
	}
	return s.handleZhipuNonStreamingResponse(resp, c, originalModel, upstreamModel, requestID, startTime)
}

func buildZhipuChatCompletionsRequest(ctx context.Context, account *Account, body []byte, isStream bool) (*http.Request, error) {
	upstreamURL := resolveZhipuChatCompletionsURL(account)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if isStream {
		req.Header.Set("Accept", "text/event-stream")
	}
	if account != nil {
		if apiKey := strings.TrimSpace(account.GetCredential("api_key")); apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
	}
	return req, nil
}

func resolveZhipuChatCompletionsURL(account *Account) string {
	baseURL := ""
	if account != nil {
		baseURL = strings.TrimSpace(account.GetCredential("base_url"))
	}
	if baseURL == "" {
		baseURL = zhipu.DefaultBaseURL
	}
	return strings.TrimRight(baseURL, "/") + "/chat/completions"
}

func (s *GatewayService) handleZhipuNonStreamingResponse(resp *http.Response, c *gin.Context, originalModel, upstreamModel, requestID string, startTime time.Time) (*ForwardResult, error) {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeGatewayCCError(c, http.StatusBadGateway, "server_error", "Failed to read upstream response")
		return nil, err
	}

	s.writeZhipuPassthroughHeaders(c, resp)
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(resp.StatusCode, contentType, respBody)

	usage := extractZhipuUsage(respBody)
	return &ForwardResult{
		RequestID:     requestID,
		Usage:         usage,
		Model:         originalModel,
		UpstreamModel: upstreamModel,
		Stream:        false,
		Duration:      time.Since(startTime),
	}, nil
}

func (s *GatewayService) handleZhipuStreamingResponse(resp *http.Response, c *gin.Context, originalModel, upstreamModel, requestID string, startTime time.Time) (*ForwardResult, error) {
	s.writeZhipuPassthroughHeaders(c, resp)
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(resp.StatusCode)

	reader := bufio.NewReader(resp.Body)

	var usage ClaudeUsage
	var firstTokenMs *int
	firstChunk := true
	clientDisconnected := false

	for {
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			trimmedLine := strings.TrimRight(line, "\r\n")
			if firstChunk && strings.TrimSpace(trimmedLine) != "" {
				firstChunk = false
				ms := int(time.Since(startTime).Milliseconds())
				firstTokenMs = &ms
			}
			if strings.HasPrefix(trimmedLine, "data: ") {
				if parsed := extractZhipuUsageFromSSEData(trimmedLine[len("data: "):]); parsed != nil {
					usage = *parsed
				}
			}
			if _, err := fmt.Fprint(c.Writer, line); err != nil {
				clientDisconnected = true
				break
			}
			c.Writer.Flush()
		}
		if readErr != nil {
			if readErr != io.EOF && !clientDisconnected {
				return nil, readErr
			}
			break
		}
	}

	return &ForwardResult{
		RequestID:        requestID,
		Usage:            usage,
		Model:            originalModel,
		UpstreamModel:    upstreamModel,
		Stream:           true,
		Duration:         time.Since(startTime),
		FirstTokenMs:     firstTokenMs,
		ClientDisconnect: clientDisconnected,
	}, nil
}

func extractZhipuUsage(body []byte) ClaudeUsage {
	usage := gjson.GetBytes(body, "usage")
	if !usage.Exists() {
		return ClaudeUsage{}
	}
	return ClaudeUsage{
		InputTokens:  int(usage.Get("prompt_tokens").Int()),
		OutputTokens: int(usage.Get("completion_tokens").Int()),
	}
}

func extractZhipuUsageFromSSEData(payload string) *ClaudeUsage {
	payload = strings.TrimSpace(payload)
	if payload == "" || payload == "[DONE]" || !gjson.Valid(payload) {
		return nil
	}
	usage := extractZhipuUsage([]byte(payload))
	if usage.InputTokens == 0 && usage.OutputTokens == 0 && !gjson.Get(payload, "usage").Exists() {
		return nil
	}
	return &usage
}

func (s *GatewayService) writeZhipuPassthroughHeaders(c *gin.Context, resp *http.Response) {
	if resp == nil {
		return
	}
	responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
}
