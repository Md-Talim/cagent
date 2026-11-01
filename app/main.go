package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func main() {
	var prompt string
	flag.StringVar(&prompt, "p", "", "Prompt to send to LLM")
	flag.Parse()

	if prompt == "" {
		panic("Prompt must not be empty")
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	baseUrl := os.Getenv("OPENROUTER_BASE_URL")
	if baseUrl == "" {
		baseUrl = "https://openrouter.ai/api/v1"
	}

	if apiKey == "" {
		panic("Env variable OPENROUTER_API_KEY not found")
	}

	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseUrl))

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(prompt),
	}

	for {
		resp, err := client.Chat.Completions.New(context.Background(),
			openai.ChatCompletionNewParams{
				Model:    "anthropic/claude-haiku-4.5",
				Messages: messages,
				Tools: []openai.ChatCompletionToolUnionParam{
					openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
						Name:        "Read",
						Description: openai.String("Read and return the contents of a file"),
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
					}),
					openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
						Name:        "Write",
						Description: openai.String("Write content to a file"),
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
					}),
					openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
						Name:        "RunBashCommand",
						Description: openai.String("Execute a shell command"),
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
					}),
				},
			},
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if len(resp.Choices) == 0 {
			panic("No choices in response")
		}

		toolCalls := resp.Choices[0].Message.ToolCalls
		messages = append(messages, resp.Choices[0].Message.ToParam())

		if len(toolCalls) == 0 {
			fmt.Print(resp.Choices[0].Message.Content)
			os.Exit(0)
		}

		for _, toolCall := range toolCalls {
			switch toolCall.Function.Name {
			case "Read":
				type ReadToolCallArgument struct {
					FilePath string `json:"file_path"`
				}

				var arguments ReadToolCallArgument

				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &arguments); err != nil {
					fmt.Fprintf(os.Stderr, "error parsing read tool call argument: %v\n", err)
				}

				data, err := os.ReadFile(arguments.FilePath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error reading file(%s): %v\n", arguments.FilePath, err)
				}

				messages = append(messages, openai.ToolMessage(string(data), toolCall.ID))
			case "Write":
				type WriteToolCallArgument struct {
					FilePath string `json:"file_path"`
					Content  string `json:"content"`
				}

				var arguments WriteToolCallArgument
				var result string

				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &arguments); err != nil {
					fmt.Fprintf(os.Stderr, "error parsing read tool call argument: %v\n", err)
				}

				err := os.WriteFile(arguments.FilePath, []byte(arguments.Content), 0644)
				if err != nil {
					result = fmt.Sprintf("Error writing file: %v", err)
					fmt.Fprintf(os.Stderr, "error writing file: %v\n", err)
				} else {
					result = "Success: File written successfully."
				}

				messages = append(messages, openai.ToolMessage(result, toolCall.ID))
			case "RunBashCommand":
				type BashToolCallArgument struct {
					Command string `json:"command"`
				}

				var arguments BashToolCallArgument
				var result string

				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &arguments); err != nil {
					fmt.Fprintf(os.Stderr, "error parsing read tool call argument: %v\n", err)
				}

				cmd := exec.Command("bash", "-c", arguments.Command)

				output, err := cmd.CombinedOutput()
				if err != nil {
					result = fmt.Sprintf("Command failed with error: %v", err)
					fmt.Fprintf(os.Stderr, "error run command: %v\n", err)
				} else {
					result = string(output)
					if result == "" {
						result = "Command executed successfully (no output returned)."
					}
				}

				messages = append(messages, openai.ToolMessage(result, toolCall.ID))

				fmt.Fprintf(os.Stderr, "[Agent Logs] Ran command: %s\n", arguments.Command)
			}
		}
	}
}
