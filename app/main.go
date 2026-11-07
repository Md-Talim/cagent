package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

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

	skills, err := LoadSkills(filepath.Join(".claude", "skills"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading skills: %v", err)
		os.Exit(1)
	}

	lsp, err := LSPClientFromEnvironment()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error initializing LSP client: %v\n", err)
		os.Exit(1)
	}
	if lsp != nil {
		fmt.Fprintln(os.Stderr, "LSP client connected")
	}

	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseUrl))

	agent := NewAgent(client, "anthropic/claude-haiku-4.5", skills, lsp, prompt)
	out, err := agent.Run(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(out)
}
