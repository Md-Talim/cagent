package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

const (
	toolCallResponse = `{
		"id": "chatcmpl-1",
		"object": "chat.completion",
		"created": 1700000000,
		"model": "test-model",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": null,
				"tool_calls": [{
					"id": "call_1",
					"type": "function",
					"function": {"name": "RunBashCommand", "arguments": "{\"command\":\"echo hi\"}"}
				}]
			},
			"finish_reason": "tool_calls"
		}]
	}`

	finalResponse = `{
		"id": "chatcmpl-2",
		"object": "chat.completion",
		"created": 1700000000,
		"model": "test-model",
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "done"},
			"finish_reason": "stop"
		}]
	}`
)

type chatRequest struct {
	Messages []struct {
		Role       string `json:"role"`
		Content    string `json:"content"`
		ToolCallID string `json:"tool_call_id"`
	} `json:"messages"`
	Tools []struct {
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	} `json:"tools"`
}

func newTestAgent(t *testing.T, handler http.Handler) *Agent {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := openai.NewClient(option.WithAPIKey("test-key"), option.WithBaseURL(srv.URL))
	return NewAgent(client, "test-model", "say hi")
}

func TestAgentRunExecutesToolAndReturnsFinalMessage(t *testing.T) {
	var calls atomic.Int32
	var sawTools, sawToolResult atomic.Bool

	agent := newTestAgent(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch calls.Add(1) {
		case 1:
			w.Header().Set("Content-Type", "application/json")
			var req chatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode request: %v", err)
			}
			if len(req.Tools) == len(tools) {
				sawTools.Store(true)
			}
			io.WriteString(w, toolCallResponse)
		case 2:
			w.Header().Set("Content-Type", "application/json")
			var req chatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode request: %v", err)
			}
			for _, m := range req.Messages {
				if m.Role == "tool" && m.ToolCallID == "call_1" && m.Content == "hi\n" {
					sawToolResult.Store(true)
				}
			}
			io.WriteString(w, finalResponse)
		default:
			t.Errorf("unexpected request %d", calls.Load())
		}
	}))

	got, err := agent.Run(context.Background())
	if err != nil {
		t.Fatalf("Agent.Run() error = %v", err)
	}
	if got != "done" {
		t.Errorf("Agent.Run() = %q, want %q", got, "done")
	}
	if !sawTools.Load() {
		t.Error("request did not advertise all tools")
	}
	if !sawToolResult.Load() {
		t.Error("second request did not contain the tool result")
	}
}

func TestAgentRunNoChoices(t *testing.T) {
	agent := newTestAgent(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"chatcmpl-1","object":"chat.completion","created":0,"model":"test-model","choices":[]}`)
	}))

	if _, err := agent.Run(context.Background()); err == nil {
		t.Fatal("Agent.Run() error = nil, want non-nil")
	}
}
