package main

import (
	"context"

	"github.com/openai/openai-go/v3"
)

type Tool struct {
	Name        string
	Description string
	Parameters  openai.FunctionParameters
	Run         func(ctx context.Context, rawArgs string) (string, error)
}

func (t Tool) Definition() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        t.Name,
		Description: openai.String(t.Description),
		Parameters:  t.Parameters,
	})
}

var tools = []Tool{readTool, writeTool, bashTool}

func toolDefinitions() []openai.ChatCompletionToolUnionParam {
	defs := make([]openai.ChatCompletionToolUnionParam, len(tools))
	for i, t := range tools {
		defs[i] = t.Definition()
	}
	return defs
}

func findTool(name string) (Tool, bool) {
	for _, t := range tools {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}
