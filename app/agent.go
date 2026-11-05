package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/openai/openai-go/v3"
)

type Agent struct {
	client   openai.Client
	model    string
	tools    []openai.ChatCompletionToolUnionParam
	messages []openai.ChatCompletionMessageParamUnion
}

func NewAgent(client openai.Client, model string, skills *SkillSet, prompt string) *Agent {
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, 2)
	if sys := skills.SystemPrompt(); sys != "" {
		messages = append(messages, openai.SystemMessage(sys))
	}
	messages = append(messages, openai.UserMessage(skills.ExpandPrompt(prompt)))

	return &Agent{
		client:   client,
		model:    model,
		tools:    toolDefinitions(),
		messages: messages,
	}
}

func (a *Agent) Run(ctx context.Context) (string, error) {
	for {
		resp, err := a.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    a.model,
			Messages: a.messages,
			Tools:    a.tools,
		})
		if err != nil {
			return "", fmt.Errorf("chat completion: %w", err)
		}
		if len(resp.Choices) == 0 {
			return "", errors.New("no choices in response")
		}

		msg := resp.Choices[0].Message
		a.messages = append(a.messages, msg.ToParam())

		if len(msg.ToolCalls) == 0 {
			return msg.Content, nil
		}

		for _, tc := range msg.ToolCalls {
			tcResp := a.runTool(ctx, tc)
			a.messages = append(a.messages, openai.ToolMessage(tcResp, tc.ID))
		}
	}
}

func (a *Agent) runTool(ctx context.Context, tc openai.ChatCompletionMessageToolCallUnion) string {
	tool, ok := findTool(tc.Function.Name)
	if !ok {
		return fmt.Sprintf("error: unknown tool %q", tc.Function.Name)
	}

	out, err := tool.Run(ctx, tc.Function.Arguments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[tool] %s failed: %v\n", tc.Function.Name, err)
		return fmt.Sprintf("error: %v", err)
	}
	return out
}
