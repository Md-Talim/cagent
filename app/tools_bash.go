package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/openai/openai-go/v3"
)

var bashTool = Tool{
	Name:        "RunBashCommand",
	Description: "Execute a shell command",
	Parameters: openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The command to execute",
			},
		},
		"required": []string{"command"},
	},
	Run: runBash,
}

func runBash(ctx context.Context, rawArgs string) (string, error) {
	var args struct {
		Command string `json:"command"`
	}

	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		return "", fmt.Errorf("parse Bash arguments: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[agent] running: %s\n", args.Command)

	cmd := exec.CommandContext(ctx, "bash", "-c", args.Command)
	output, runErr := cmd.CombinedOutput()
	result := string(output)

	if runErr != nil {
		if result == "" {
			return fmt.Sprintf("command failed: %v", runErr), nil
		}
		return result + fmt.Sprintf("\ncommand failed: %v", runErr), nil
	}
	if result == "" {
		return "Command executed successfully (no output returned).", nil
	}
	return result, nil
}
