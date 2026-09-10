// llm.go talks to agentgateway's own OpenAI-compatible LLM route (this repo's
// providers feature) -- this agent never holds a provider API key itself;
// agentgateway injects the real one server-side, matching how retail-returns'
// agents/UI already call agentgateway-hosted LLM routes rather than the
// provider directly.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function functionCall `json:"function"`
}

type functionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type toolSpec struct {
	Type     string       `json:"type"`
	Function functionSpec `json:"function"`
}

type functionSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Tools    []toolSpec    `json:"tools,omitempty"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
}

func chatCompletion(ctx context.Context, llmURL, model string, messages []chatMessage, tools []toolSpec) (chatMessage, error) {
	reqBody, err := json.Marshal(chatCompletionRequest{Model: model, Messages: messages, Tools: tools})
	if err != nil {
		return chatMessage{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, llmURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return chatMessage{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return chatMessage{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return chatMessage{}, fmt.Errorf("llm request failed: HTTP %d", resp.StatusCode)
	}

	var out chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return chatMessage{}, err
	}
	if len(out.Choices) == 0 {
		return chatMessage{}, fmt.Errorf("llm response had no choices")
	}
	return out.Choices[0].Message, nil
}
