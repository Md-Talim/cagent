package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go/v3"
)

func newLSPTool(lsp *LSPClient) Tool {
	return Tool{
		Name:        "LSP",
		Description: "Returns the file, line, and character where a symbol is defined.",
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"operation": map[string]any{
					"type": "string",
					"enum": []string{"goToDefinition"},
				},
				"filePath": map[string]any{
					"type":        "string",
					"description": "Absolute path, or a path relative to the agent's current working directory.",
				},
				"line": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "1-based line number of the symbol.",
				},
				"character": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "1-based character offset in UTF-16 code units (same as character count for ASCII).",
				},
			},
			"required": []string{"operation", "filePath", "line", "character"},
		},
		Run: func(ctx context.Context, rawArgs string) (string, error) {
			return runLSP(ctx, lsp, rawArgs)
		},
	}
}

func runLSP(_ context.Context, _ *LSPClient, rawArgs string) (string, error) {
	var args struct {
		Operation string `json:"operation"`
		FilePath  string `json:"filePath"`
		Line      int    `json:"line"`
		Character int    `json:"character"`
	}

	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		return "", fmt.Errorf("parse LSP arguments: %w", err)
	}

	switch args.Operation {
	case "goToDefinition":
		return "LSP is not implemented yet", nil
	default:
		return "", fmt.Errorf("unsupported LSP operation %q", args.Operation)
	}
}
