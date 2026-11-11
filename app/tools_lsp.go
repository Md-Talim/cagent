package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/openai/openai-go/v3"
)

// type lspArguments struct {
// 	Operation string `json:"operation"`
// 	FilePath  string `json:"filePath"`
// 	Line      int    `json:"line"`
// 	Character int    `json:"character"`
// }

type lspDocument struct {
	version int
	text    string
}

type lspSession struct {
	client *LSPClient
	docs   map[string]*lspDocument
}

func newLSPSession(client *LSPClient) *lspSession {
	return &lspSession{
		client: client,
		docs:   map[string]*lspDocument{},
	}
}

func newLSPTool(session *lspSession) Tool {
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
			return runLSP(ctx, session, rawArgs)
		},
	}
}

func runLSP(_ context.Context, session *lspSession, rawArgs string) (string, error) {
	var args struct {
		Operation string `json:"operation"`
		FilePath  string `json:"filePath"`
		Line      int    `json:"line"`
		Character int    `json:"character"`
	}

	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		return "", fmt.Errorf("parse LSP arguments: %w", err)
	}
	if args.Operation != "goToDefinition" {
		return "", fmt.Errorf("unsupported LSP operation: %q", args.Operation)
	}
	if args.Line < 1 || args.Character < 1 {
		return "", fmt.Errorf("line and character are 1-based")
	}

	path, err := filepath.Abs(args.FilePath)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", args.FilePath, err)
	}

	uri := fileURI(path)
	if err := session.open(path, uri); err != nil {
		return "", err
	}

	location, err := session.definition(uri, args.Line-1, args.Character-1)
	if err != nil {
		return "", err
	}

	rel, err := relativePath(location.URI)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"Defined in %s:%d:%d",
		rel,
		location.Range.Start.Line+1,
		location.Range.Start.Character+1,
	), nil
}

func (s *lspSession) open(path, uri string) error {
	if _, ok := s.docs[uri]; ok {
		return nil
	}

	text, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	languageID := s.client.ExtensionToLanguage[strings.ToLower(filepath.Ext(path))]

	payload := lspPayload{
		Method: "textDocument/didOpen",
		Params: map[string]any{
			"textDocument": map[string]any{
				"uri":        uri,
				"languageId": languageID,
				"version":    1,
				"text":       string(text),
			},
		},
	}

	if _, err := s.client.Call("notify", payload); err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}

	s.docs[uri] = &lspDocument{version: 1, text: string(text)}
	return nil
}

func (s *lspSession) definition(uri string, line, character int) (lspLocation, error) {
	payload := lspPayload{
		Method: "textDocument/definition",
		Params: map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"position": map[string]any{
				"line":      line,
				"character": character,
			},
		},
	}

	data, err := s.client.Call("request", payload)
	if err != nil {
		return lspLocation{}, fmt.Errorf("definition request: %w", err)
	}

	location, ok := parseLocation(data)
	if !ok {
		return lspLocation{}, fmt.Errorf("no definition found")
	}
	return location, nil
}

type lspPayload struct {
	Method string `json:"method"`
	Params any    `json:"params"`
}

type lspPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type lspRange struct {
	Start lspPosition `json:"start"`
	End   lspPosition `json:"end"`
}

type lspLocation struct {
	URI   string   `json:"uri"`
	Range lspRange `json:"range"`
}

func parseLocation(data json.RawMessage) (lspLocation, bool) {
	var location lspLocation
	if err := json.Unmarshal(data, &location); err == nil && location.URI != "" {
		return location, true
	}

	var locations []lspLocation
	if err := json.Unmarshal(data, &locations); err == nil && len(locations) > 0 {
		return locations[0], true
	}

	var response struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(data, &response); err == nil && len(response.Result) > 0 {
		return parseLocation(response.Result)
	}

	return lspLocation{}, false
}

func fileURI(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

func relativePath(uri string) (string, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("parse definition URI: %w", err)
	}
	if parsed.Scheme != "file" {
		return "", fmt.Errorf("unsupported definition URI %q", uri)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(cwd, parsed.Path)
	if err != nil {
		return "", fmt.Errorf("relative path for %s: %w", parsed.Path, err)
	}
	return filepath.ToSlash(rel), nil
}
