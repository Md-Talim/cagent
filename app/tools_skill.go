package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/openai/openai-go/v3"
)

type SubagentRunner func(ctx context.Context, prompt string) (string, error)

func newSkillTool(skills *SkillSet, runSubagent SubagentRunner) Tool {
	return Tool{
		Name:        "Skill",
		Description: "Load a skill's instructions into the conversation",
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "The name of the skill to use",
				},
				"args": map[string]any{
					"type":        "string",
					"description": "Optional arguments for the skill",
				},
			},
			"required": []string{"name"},
		},
		Run: func(ctx context.Context, rawArgs string) (string, error) {
			return runSkill(ctx, skills, runSubagent, rawArgs)
		},
	}
}

func runSkill(ctx context.Context, skills *SkillSet, runSubagent SubagentRunner, rawArgs string) (string, error) {
	var args struct {
		Name string `json:"name"`
		Args string `json:"args"`
	}

	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		return "", fmt.Errorf("parse Read arguments: %w", err)
	}
	if args.Name == "" {
		return "", errors.New("skill name is required")
	}

	skill, ok := skills.Find(args.Name)
	if !ok {
		return "", fmt.Errorf("unknown skill %q", args.Name)
	}

	if !skill.IsForked() {
		return skill.Render(args.Args)
	}

	body, err := skill.Body()
	if err != nil {
		return "", fmt.Errorf("read skill %q: %w", skill.Name, err)
	}
	if runSubagent == nil {
		return "", errors.New("subagent runner is not configured")
	}

	answer, err := runSubagent(ctx, parseSkillBody(body, args.Args))
	if err != nil {
		return "", fmt.Errorf("run skill %q in subagent: %w", skill.Name, err)
	}

	return fmt.Sprintf(
		"Skill %s ran in a separate context and returned: %s",
		skill.Name,
		answer,
	), nil
}
