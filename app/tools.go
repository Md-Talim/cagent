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

var baseTools = []Tool{readTool, writeTool, bashTool}

func toolRegistry(skills *SkillSet, runSubagent SubagentRunner) []Tool {
	tools := make([]Tool, 0, len(baseTools)+1)
	tools = append(tools, baseTools...)
	if skills != nil && skills.Len() > 0 {
		tools = append(tools, newSkillTool(skills, runSubagent))
	}
	return tools
}

func toolDefinitions(tools []Tool) []openai.ChatCompletionToolUnionParam {
	defs := make([]openai.ChatCompletionToolUnionParam, len(tools))
	for i, t := range tools {
		defs[i] = t.Definition()
	}
	return defs
}

func findTool(tools []Tool, name string) (Tool, bool) {
	for _, t := range tools {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}
