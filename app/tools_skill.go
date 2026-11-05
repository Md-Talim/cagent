package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/openai/openai-go/v3"
)

func newSkillTool(skills *SkillSet) Tool {
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
			return runSkill(ctx, skills, rawArgs)
		},
	}
}

func runSkill(_ context.Context, skills *SkillSet, rawArgs string) (string, error) {
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

	return skill.Render(args.Args)
}
