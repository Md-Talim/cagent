package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/openai/openai-go/v3"
)

var readTool = Tool{
	Name:        "Read",
	Description: "Read and return the contents of a file",
	Parameters: openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The path to the file to read",
			},
		},
		"required": []string{"file_path"},
	},
	Run: runRead,
}

func runRead(_ context.Context, rawArgs string) (string, error) {
	var args struct {
		FilePath string `json:"file_path"`
	}

	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		return "", fmt.Errorf("parse Read arguments: %w", err)
	}

	data, err := os.ReadFile(args.FilePath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", args.FilePath, err)
	}

	return string(data), nil
}
