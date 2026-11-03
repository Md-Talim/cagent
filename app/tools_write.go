package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/openai/openai-go/v3"
)

var writeTool = Tool{
	Name:        "Write",
	Description: "Write content to a file",
	Parameters: openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The path of the file to write to",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The content to write to the file",
			},
		},
		"required": []string{"file_path", "content"},
	},
	Run: runWrite,
}

func runWrite(_ context.Context, rawArgs string) (string, error) {
	var args struct {
		FilePath string `json:"file_path"`
		Content  string `json:"content"`
	}

	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		return "", fmt.Errorf("parse Write arguments: %w", err)
	}

	if err := os.WriteFile(args.FilePath, []byte(args.Content), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", args.FilePath, err)
	}

	return "Success: File written successfully.", nil
}
