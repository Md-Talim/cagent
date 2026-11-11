package main

import (
	"context"
	"slices"
	"testing"

	"github.com/openai/openai-go/v3"
)

func TestLSPToolDefinition(t *testing.T) {
	tool := newLSPTool(nil)

	if tool.Name != "LSP" {
		t.Errorf("Name = %q, want LSP", tool.Name)
	}

	props, ok := tool.Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties = %T", tool.Parameters["properties"])
	}
	for _, name := range []string{"operation", "filePath", "line", "character"} {
		if _, ok := props[name]; !ok {
			t.Errorf("missing property %q", name)
		}
	}

	required, ok := tool.Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required = %T", tool.Parameters["required"])
	}
	want := []string{"operation", "filePath", "line", "character"}
	if !slices.Equal(required, want) {
		t.Errorf("required = %v, want %v", required, want)
	}

	operation, _ := props["operation"].(map[string]any)
	enum, _ := operation["enum"].([]string)
	if !slices.Equal(enum, []string{"goToDefinition"}) {
		t.Errorf("operation enum = %v", enum)
	}
}

func TestRunLSPPlaceholder(t *testing.T) {
	got, err := runLSP(
		context.Background(),
		nil,
		`{"operation":"goToDefinition","filePath":"checkout.py","line":5,"character":12}`,
	)
	if err != nil {
		t.Fatalf("runLSP() error = %v", err)
	}
	if got != "LSP is not implemented yet" {
		t.Errorf("runLSP() = %q", got)
	}
}

func TestToolRegistryIncludesLSPWhenConfigured(t *testing.T) {
	if _, ok := findTool(toolRegistry(nil, nil, nil), "LSP"); ok {
		t.Error("LSP tool registered without a client")
	}
	if _, ok := findTool(toolRegistry(nil, nil, newLSPSession(&LSPClient{})), "LSP"); !ok {
		t.Error("LSP tool not registered with a client")
	}
}

func TestNewAgentCarriesLSPClient(t *testing.T) {
	agent := NewAgent(openai.Client{}, "test-model", &SkillSet{}, &LSPClient{}, "hi")
	if _, ok := findTool(agent.tools, "LSP"); !ok {
		t.Error("LSP tool missing from agent")
	}

	without := NewAgent(openai.Client{}, "test-model", &SkillSet{}, &LSPClient{}, "hi")
	if _, ok := findTool(without.tools, "LSP"); ok {
		t.Error("LSP tool present without a client")
	}
}
