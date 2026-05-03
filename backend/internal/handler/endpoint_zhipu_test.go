package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestDeriveUpstreamEndpoint_Zhipu_ChatCompletions(t *testing.T) {
	got := DeriveUpstreamEndpoint(EndpointChatCompletions, "/v1/chat/completions", service.PlatformZhipu)
	if got != EndpointChatCompletions {
		t.Errorf("DeriveUpstreamEndpoint(zhipu) = %q; want %q", got, EndpointChatCompletions)
	}
}

func TestDeriveUpstreamEndpoint_Zhipu_FromMessages(t *testing.T) {
	got := DeriveUpstreamEndpoint(EndpointMessages, "/v1/messages", service.PlatformZhipu)
	if got != EndpointChatCompletions {
		t.Errorf("DeriveUpstreamEndpoint(zhipu, messages-inbound) = %q; want %q",
			got, EndpointChatCompletions)
	}
}
