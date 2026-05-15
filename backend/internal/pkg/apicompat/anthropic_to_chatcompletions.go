package apicompat

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Request: AnthropicRequest → ChatCompletionsRequest
// ---------------------------------------------------------------------------

// AnthropicToChatCompletions converts an Anthropic Messages request into
// an OpenAI Chat Completions request.
func AnthropicToChatCompletions(req *AnthropicRequest) (*ChatCompletionsRequest, error) {
	messages, err := convertAnthropicToChatMessages(req.System, req.Messages)
	if err != nil {
		return nil, err
	}

	out := &ChatCompletionsRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   req.Stream,
	}

	if req.MaxTokens > 0 {
		out.MaxTokens = &req.MaxTokens
	}

	if req.Temperature != nil {
		out.Temperature = req.Temperature
	}
	if req.TopP != nil {
		out.TopP = req.TopP
	}

	if len(req.Tools) > 0 {
		out.Tools = convertAnthropicToolsToChat(req.Tools)
	}

	if len(req.ToolChoice) > 0 {
		tc, err := convertAnthropicToolChoiceToChat(req.ToolChoice)
		if err != nil {
			return nil, fmt.Errorf("convert tool_choice: %w", err)
		}
		out.ToolChoice = tc
	}

	effort := resolveAnthropicReasoningEffortForChat(req.Thinking, req.OutputConfig)
	if effort != "" {
		out.ReasoningEffort = effort
	}

	return out, nil
}

// resolveAnthropicReasoningEffortForChat maps Anthropic thinking/effort to
// Chat Completions reasoning_effort. Returns "" when reasoning should be disabled.
func resolveAnthropicReasoningEffortForChat(thinking *AnthropicThinking, outputCfg *AnthropicOutputConfig) string {
	if thinking != nil && thinking.Type == "disabled" {
		return ""
	}
	if thinking != nil && thinking.BudgetTokens > 0 {
		return budgetTokensToChatEffort(thinking.BudgetTokens)
	}
	if outputCfg != nil && outputCfg.Effort != "" {
		return mapAnthropicEffortToChatEffort(outputCfg.Effort)
	}
	return ""
}

// budgetTokensToChatEffort maps Anthropic thinking budget_tokens to Chat Completions
// reasoning_effort level. Values are approximate.
func budgetTokensToChatEffort(tokens int) string {
	switch {
	case tokens <= 1024:
		return "low"
	case tokens <= 8192:
		return "medium"
	case tokens <= 32000:
		return "high"
	default:
		return "high"
	}
}

// mapAnthropicEffortToChatEffort converts Anthropic effort levels to Chat Completions
// reasoning_effort levels.
//
//	low    → low
//	medium → medium
//	high   → high
//	max    → high  (Chat Completions has no "xhigh" equivalent)
func mapAnthropicEffortToChatEffort(effort string) string {
	switch effort {
	case "low", "medium", "high":
		return effort
	case "max":
		return "high"
	default:
		return effort
	}
}

// convertAnthropicToChatMessages converts the Anthropic system field and message
// list into a Chat Completions messages array.
func convertAnthropicToChatMessages(system json.RawMessage, msgs []AnthropicMessage) ([]ChatMessage, error) {
	var out []ChatMessage

	if len(system) > 0 {
		sysText, err := parseAnthropicSystemPrompt(system)
		if err != nil {
			return nil, err
		}
		if sysText != "" {
			content, _ := json.Marshal(sysText)
			out = append(out, ChatMessage{
				Role:    "system",
				Content: content,
			})
		}
	}

	for _, m := range msgs {
		chatMsgs, err := anthropicMsgToChatMessages(m)
		if err != nil {
			return nil, err
		}
		out = append(out, chatMsgs...)
	}
	return out, nil
}

// anthropicMsgToChatMessages converts a single Anthropic message into one or more
// Chat Completions messages.
func anthropicMsgToChatMessages(m AnthropicMessage) ([]ChatMessage, error) {
	switch m.Role {
	case "user":
		return chatUserFromAnthropic(m.Content)
	case "assistant":
		return chatAssistantFromAnthropic(m.Content)
	default:
		return chatUserFromAnthropic(m.Content)
	}
}

// chatUserFromAnthropic converts an Anthropic user message to Chat Completions format.
func chatUserFromAnthropic(raw json.RawMessage) ([]ChatMessage, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		content, _ := json.Marshal(s)
		return []ChatMessage{{Role: "user", Content: content}}, nil
	}

	var blocks []AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, err
	}

	var parts []ChatContentPart
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				parts = append(parts, ChatContentPart{Type: "text", Text: b.Text})
			}
		case "image":
			if uri := anthropicImageToDataURI(b.Source); uri != "" {
				parts = append(parts, ChatContentPart{
					Type: "image_url",
					ImageURL: &ChatImageURL{URL: uri},
				})
			}
		}
	}

	if len(parts) == 0 {
		return []ChatMessage{{Role: "user", Content: nil}}, nil
	}

	content, err := json.Marshal(parts)
	if err != nil {
		return nil, err
	}
	return []ChatMessage{{Role: "user", Content: content}}, nil
}

// chatAssistantFromAnthropic converts an Anthropic assistant message to Chat Completions format.
func chatAssistantFromAnthropic(raw json.RawMessage) ([]ChatMessage, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		content, _ := json.Marshal(s)
		return []ChatMessage{{Role: "assistant", Content: content}}, nil
	}

	var blocks []AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, err
	}

	msg := ChatMessage{Role: "assistant"}

	// Collect text content and tool calls.
	var textParts []string
	var toolCalls []ChatToolCall

	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				textParts = append(textParts, b.Text)
			}
		case "tool_use":
			args := "{}"
			if len(b.Input) > 0 {
				args = string(b.Input)
			}
			toolCalls = append(toolCalls, ChatToolCall{
				ID:   b.ID,
				Type: "function",
				Function: ChatFunctionCall{
					Name:      b.Name,
					Arguments: args,
				},
			})
		}
	}

	if len(textParts) > 0 {
		content, _ := json.Marshal(strings.Join(textParts, "\n\n"))
		msg.Content = content
	}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}

	return []ChatMessage{msg}, nil
}

// convertAnthropicToolChoiceToChat maps Anthropic tool_choice to Chat Completions format.
//
//	{"type":"auto"}            → "auto"
//	{"type":"any"}             → "required"
//	{"type":"none"}            → "none"
//	{"type":"tool","name":"X"} → {"type":"function","name":"X"}
func convertAnthropicToolChoiceToChat(raw json.RawMessage) (json.RawMessage, error) {
	var tc struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &tc); err != nil {
		return nil, err
	}

	switch tc.Type {
	case "auto":
		return json.Marshal("auto")
	case "any":
		return json.Marshal("required")
	case "none":
		return json.Marshal("none")
	case "tool":
		return json.Marshal(map[string]any{
			"type": "function",
			"name": tc.Name,
		})
	default:
		return raw, nil
	}
}

// convertAnthropicToolsToChat maps Anthropic tool definitions to Chat Completions tools.
// Server-side tools (web_search) are skipped as Chat Completions doesn't support them.
func convertAnthropicToolsToChat(tools []AnthropicTool) []ChatTool {
	var out []ChatTool
	for _, t := range tools {
		if strings.HasPrefix(t.Type, "web_search") {
			continue
		}
		out = append(out, ChatTool{
			Type: "function",
			Function: &ChatFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  normalizeToolParameters(t.InputSchema),
			},
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// Non-streaming response: ChatCompletionsResponse → AnthropicResponse
// ---------------------------------------------------------------------------

// ChatCompletionsResponseToAnthropic converts a Chat Completions response into
// an Anthropic Messages response.
func ChatCompletionsResponseToAnthropic(resp *ChatCompletionsResponse, model string) *AnthropicResponse {
	out := &AnthropicResponse{
		ID:     resp.ID,
		Type:   "message",
		Role:   "assistant",
		Model:  model,
	}

	if len(resp.Choices) == 0 {
		out.Content = []AnthropicContentBlock{{Type: "text", Text: ""}}
		out.StopReason = "end_turn"
		return out
	}

	choice := resp.Choices[0]
	var blocks []AnthropicContentBlock

	if len(choice.Message.Content) > 0 {
		var contentText string
		// Try parsing as plain string first
		if err := json.Unmarshal(choice.Message.Content, &contentText); err == nil {
			if contentText != "" {
				blocks = append(blocks, AnthropicContentBlock{
					Type: "text",
					Text: contentText,
				})
			}
		} else {
			// Otherwise try as content parts array
			var parts []ChatContentPart
			if err := json.Unmarshal(choice.Message.Content, &parts); err == nil {
				for _, p := range parts {
					if p.Type == "text" && p.Text != "" {
						blocks = append(blocks, AnthropicContentBlock{
							Type: "text",
							Text: p.Text,
						})
					}
				}
			}
		}
	}

	if choice.Message.ReasoningContent != "" {
		blocks = append(blocks, AnthropicContentBlock{
			Type:     "thinking",
			Thinking: choice.Message.ReasoningContent,
		})
	}

	for _, tc := range choice.Message.ToolCalls {
		blocks = append(blocks, AnthropicContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}

	if len(blocks) == 0 {
		blocks = append(blocks, AnthropicContentBlock{Type: "text", Text: ""})
	}
	out.Content = blocks
	out.StopReason = chatFinishReasonToAnthropicStopReason(choice.FinishReason, len(choice.Message.ToolCalls) > 0)

	if resp.Usage != nil {
		out.Usage = AnthropicUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		}
		if resp.Usage.PromptTokensDetails != nil {
			out.Usage.CacheReadInputTokens = resp.Usage.PromptTokensDetails.CachedTokens
		}
	}

	return out
}

func chatFinishReasonToAnthropicStopReason(reason string, hasToolCalls bool) string {
	switch reason {
	case "stop":
		if hasToolCalls {
			return "tool_use"
		}
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	default:
		return "end_turn"
	}
}

// ---------------------------------------------------------------------------
// Streaming: ChatCompletionsChunk → []AnthropicStreamEvent (stateful converter)
// ---------------------------------------------------------------------------

// ChatCompletionsEventToAnthropicState tracks state for converting a sequence
// of Chat Completions SSE chunks into Anthropic SSE events.
type ChatCompletionsEventToAnthropicState struct {
	MessageStartSent bool
	MessageStopSent  bool

	ContentBlockIndex int
	ContentBlockOpen bool
	CurrentBlockType string // "text" | "thinking" | "tool_use"
	CurrentToolName  string
	CurrentToolArgs  string

	InputTokens          int
	OutputTokens         int
	CacheReadInputTokens int

	ResponseID string
	Model     string
	Created   int64
}

// NewChatCompletionsEventToAnthropicState returns an initialised stream state.
func NewChatCompletionsEventToAnthropicState() *ChatCompletionsEventToAnthropicState {
	return &ChatCompletionsEventToAnthropicState{
		Created: time.Now().Unix(),
	}
}

// ChatCompletionsEventToAnthropicEvents converts a single Chat Completions chunk
// into zero or more Anthropic SSE events, updating state as it goes.
func ChatCompletionsEventToAnthropicEvents(
	chunk *ChatCompletionsChunk,
	state *ChatCompletionsEventToAnthropicState,
) []AnthropicStreamEvent {
	if len(chunk.Choices) == 0 {
		// Handle usage-only or [DONE] chunk
		if chunk.Usage != nil {
			state.InputTokens = chunk.Usage.PromptTokens
			state.OutputTokens = chunk.Usage.CompletionTokens
			if chunk.Usage.PromptTokensDetails != nil {
				state.CacheReadInputTokens = chunk.Usage.PromptTokensDetails.CachedTokens
			}
		}
		return nil
	}

	choice := chunk.Choices[0]

	if state.ResponseID == "" {
		state.ResponseID = chunk.ID
	}
	if state.Model == "" {
		state.Model = chunk.Model
	}
	state.Created = chunk.Created

	var events []AnthropicStreamEvent

	// message_start
	if !state.MessageStartSent && choice.Delta.Role != "" {
		state.MessageStartSent = true
		events = append(events, AnthropicStreamEvent{
			Type: "message_start",
			Message: &AnthropicResponse{
				ID:      state.ResponseID,
				Type:    "message",
				Role:    "assistant",
				Content: []AnthropicContentBlock{},
				Model:   state.Model,
				Usage: AnthropicUsage{
					InputTokens:  0,
					OutputTokens: 0,
				},
			},
		})
	}

	// content delta
	if choice.Delta.Content != nil && *choice.Delta.Content != "" {
		if !state.ContentBlockOpen || state.CurrentBlockType != "text" {
			events = append(events, closeCurrentChatBlock(state)...)
			idx := state.ContentBlockIndex
			state.ContentBlockOpen = true
			state.CurrentBlockType = "text"
			events = append(events, AnthropicStreamEvent{
				Type:  "content_block_start",
				Index: &idx,
				ContentBlock: &AnthropicContentBlock{
					Type: "text",
					Text: "",
				},
			})
		}
		idx := state.ContentBlockIndex
		text := *choice.Delta.Content
		events = append(events, AnthropicStreamEvent{
			Type:  "content_block_delta",
			Index: &idx,
			Delta: &AnthropicDelta{
				Type: "text_delta",
				Text: text,
			},
		})
	}

	// reasoning_content delta
	if choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
		if !state.ContentBlockOpen || state.CurrentBlockType != "thinking" {
			events = append(events, closeCurrentChatBlock(state)...)
			idx := state.ContentBlockIndex
			state.ContentBlockOpen = true
			state.CurrentBlockType = "thinking"
			events = append(events, AnthropicStreamEvent{
				Type:  "content_block_start",
				Index: &idx,
				ContentBlock: &AnthropicContentBlock{
					Type:     "thinking",
					Thinking: "",
				},
			})
		}
		idx := state.ContentBlockIndex
		reasoning := *choice.Delta.ReasoningContent
		events = append(events, AnthropicStreamEvent{
			Type:  "content_block_delta",
			Index: &idx,
			Delta: &AnthropicDelta{
				Type:     "thinking_delta",
				Thinking: reasoning,
			},
		})
	}

	// tool_call delta
	for _, tc := range choice.Delta.ToolCalls {
		idx := 0
		if tc.Index != nil {
			idx = *tc.Index
		}

		// Tool call start: id + name without arguments
		if tc.ID != "" && tc.Function.Name != "" {
			events = append(events, closeCurrentChatBlock(state)...)
			state.ContentBlockOpen = true
			state.CurrentBlockType = "tool_use"
			state.CurrentToolName = tc.Function.Name
			state.CurrentToolArgs = ""
			events = append(events, AnthropicStreamEvent{
				Type:  "content_block_start",
				Index: &idx,
				ContentBlock: &AnthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: json.RawMessage("{}"),
				},
			})
		}

		// Tool call arguments delta
		if tc.Function.Arguments != "" {
			if !state.ContentBlockOpen || state.CurrentBlockType != "tool_use" {
				continue
			}
			state.CurrentToolArgs += tc.Function.Arguments
			events = append(events, AnthropicStreamEvent{
				Type:  "content_block_delta",
				Index: &idx,
				Delta: &AnthropicDelta{
					Type:        "input_json_delta",
					PartialJSON: tc.Function.Arguments,
				},
			})
		}
	}

	// Handle finish
	if choice.FinishReason != nil {
		events = append(events, closeCurrentChatBlock(state)...)
		stopReason := chatFinishReasonToAnthropicStopReason(*choice.FinishReason, state.CurrentBlockType == "tool_use")
		events = append(events,
			AnthropicStreamEvent{
				Type: "message_delta",
				Delta: &AnthropicDelta{
					StopReason: stopReason,
				},
				Usage: &AnthropicUsage{
					InputTokens:          state.InputTokens,
					OutputTokens:         state.OutputTokens,
					CacheReadInputTokens: state.CacheReadInputTokens,
				},
			},
			AnthropicStreamEvent{Type: "message_stop"},
		)
		state.MessageStopSent = true
	}

	return events
}

// FinalizeChatCompletionsAnthropicStream emits synthetic termination events if the
// stream ended without a proper completion chunk.
func FinalizeChatCompletionsAnthropicStream(state *ChatCompletionsEventToAnthropicState) []AnthropicStreamEvent {
	if !state.MessageStartSent || state.MessageStopSent {
		return nil
	}

	var events []AnthropicStreamEvent
	events = append(events, closeCurrentChatBlock(state)...)
	events = append(events,
		AnthropicStreamEvent{
			Type: "message_delta",
			Delta: &AnthropicDelta{
				StopReason: "end_turn",
			},
			Usage: &AnthropicUsage{
				InputTokens:          state.InputTokens,
				OutputTokens:         state.OutputTokens,
				CacheReadInputTokens: state.CacheReadInputTokens,
			},
		},
		AnthropicStreamEvent{Type: "message_stop"},
	)
	state.MessageStopSent = true
	return events
}

// ChatCompletionsAnthropicEventToSSE formats an AnthropicStreamEvent as an SSE pair.
func ChatCompletionsAnthropicEventToSSE(evt AnthropicStreamEvent) (string, error) {
	data, err := json.Marshal(evt)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("event: %s\ndata: %s\n\n", evt.Type, data), nil
}

// closeCurrentChatBlock emits a content_block_stop event and resets block state.
func closeCurrentChatBlock(state *ChatCompletionsEventToAnthropicState) []AnthropicStreamEvent {
	if !state.ContentBlockOpen {
		return nil
	}
	idx := state.ContentBlockIndex
	state.ContentBlockOpen = false
	state.ContentBlockIndex++
	state.CurrentToolName = ""
	state.CurrentToolArgs = ""
	state.CurrentBlockType = ""
	return []AnthropicStreamEvent{{
		Type:  "content_block_stop",
		Index: &idx,
	}}
}
