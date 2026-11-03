package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRunRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hello.txt")
	want := "hello world\n"
	if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := runRead(context.Background(), fmt.Sprintf(`{"file_path":%q}`, path))
	if err != nil {
		t.Fatalf("runRead() error = %v", err)
	}
	if got != want {
		t.Errorf("runRead() = %q, want %q", got, want)
	}
}

func TestRunReadErrors(t *testing.T) {
	tests := []struct {
		name string
		args string
	}{
		{"invalid json", `{"file_path":`},
		{"missing file", fmt.Sprintf(`{"file_path":%q}`, filepath.Join(t.TempDir(), "nope.txt"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := runRead(context.Background(), tt.args); err == nil {
				t.Fatal("runRead() error = nil, want non-nil")
			}
		})
	}
}

func TestRunWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.txt")
	content := "line one\nline two\n"

	got, err := runWrite(context.Background(), fmt.Sprintf(`{"file_path":%q,"content":%q}`, path, content))
	if err != nil {
		t.Fatalf("runWrite() error = %v", err)
	}
	if got != "Success: File written successfully." {
		t.Errorf("runWrite() = %q, want success message", got)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Errorf("file content = %q, want %q", data, content)
	}
}

func TestRunWriteOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.txt")
	if err := os.WriteFile(path, []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := runWrite(context.Background(), fmt.Sprintf(`{"file_path":%q,"content":"new"}`, path)); err != nil {
		t.Fatalf("runWrite() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Errorf("file content = %q, want %q", data, "new")
	}
}

func TestRunWriteErrors(t *testing.T) {
	tests := []struct {
		name string
		args string
	}{
		{"invalid json", `{"file_path":`},
		{"missing parent dir", fmt.Sprintf(`{"file_path":%q,"content":"x"}`, filepath.Join(t.TempDir(), "missing", "file.txt"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := runWrite(context.Background(), tt.args); err == nil {
				t.Fatal("runWrite() error = nil, want non-nil")
			}
		})
	}
}

func TestRunBash(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{"stdout", "echo hello", "hello\n"},
		{"stderr is captured", "echo oops >&2", "oops\n"},
		{"no output", "true", "Command executed successfully (no output returned)."},
		{"failure with output", "echo before; exit 3", "before\n\ncommand failed: exit status 3"},
		{"failure without output", "exit 2", "command failed: exit status 2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runBash(context.Background(), fmt.Sprintf(`{"command":%q}`, tt.command))
			if err != nil {
				t.Fatalf("runBash() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("runBash() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunBashInvalidArgs(t *testing.T) {
	if _, err := runBash(context.Background(), `{"command":`); err == nil {
		t.Fatal("runBash() error = nil, want non-nil")
	}
}

func TestFindTool(t *testing.T) {
	for _, name := range []string{"Read", "Write", "RunBashCommand"} {
		tool, ok := findTool(name)
		if !ok {
			t.Fatalf("findTool(%q) not found", name)
		}
		if tool.Name != name {
			t.Errorf("findTool(%q).Name = %q", name, tool.Name)
		}
	}

	if _, ok := findTool("Nope"); ok {
		t.Error("findTool(\"Nope\") found, want not found")
	}
}

func TestToolDefinitions(t *testing.T) {
	want := []string{"Read", "Write", "RunBashCommand"}
	defs := toolDefinitions()
	if len(defs) != len(want) {
		t.Fatalf("len(toolDefinitions()) = %d, want %d", len(defs), len(want))
	}

	for i, name := range want {
		fn := defs[i].GetFunction()
		if fn == nil {
			t.Fatalf("definition %d has no function", i)
		}
		if fn.Name != name {
			t.Errorf("definition %d name = %q, want %q", i, fn.Name, name)
		}
		if fn.Description.Value == "" {
			t.Errorf("definition %d has empty description", i)
		}
	}
}
