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
		tool, ok := findTool(baseTools, name)
		if !ok {
			t.Fatalf("findTool(%q) not found", name)
		}
		if tool.Name != name {
			t.Errorf("findTool(%q).Name = %q", name, tool.Name)
		}
	}

	if _, ok := findTool(baseTools, "Nope"); ok {
		t.Error("findTool(\"Nope\") found, want not found")
	}
}

func TestToolDefinitions(t *testing.T) {
	want := []string{"Read", "Write", "RunBashCommand"}
	defs := toolDefinitions(baseTools)
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

func TestRunSkill(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "apple", "---\nname: apple\ndescription: Deploys apple.\n---\n\nblueberry-$ARGUMENTS\n")

	set, err := LoadSkills(root)
	if err != nil {
		t.Fatalf("LoadSkills() error = %v", err)
	}

	apple, ok := set.Find("apple")
	if !ok {
		t.Fatal("Find(\"apple\") not found")
	}

	runner := func(context.Context, string) (string, error) {
		t.Fatal("subagent runner called for a regular skill")
		return "", nil
	}

	got, err := runSkill(context.Background(), set, runner, `{"name":"apple","args":"4127"}`)
	if err != nil {
		t.Fatalf("runSkill() error = %v", err)
	}

	want := skillPrefix(apple) + "blueberry-4127"
	if got != want {
		t.Errorf("runSkill() = %q, want %q", got, want)
	}
}

func TestRunSkillErrors(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "apple", "---\nname: apple\ndescription: Apple.\n---\n\nblueberry\n")

	set, err := LoadSkills(root)
	if err != nil {
		t.Fatalf("LoadSkills() error = %v", err)
	}

	tests := []struct {
		name string
		args string
	}{
		{"unknown skill", `{"name":"kiwi"}`},
		{"missing name", `{}`},
		{"invalid json", `{"name":`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := runSkill(context.Background(), set, nil, tt.args); err == nil {
				t.Fatal("runSkill() error = nil, want non-nil")
			}
		})
	}
}

func TestToolRegistryIncludesSkillTool(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "apple", "---\nname: apple\ndescription: Apple.\n---\n\nblueberry\n")

	set, err := LoadSkills(root)
	if err != nil {
		t.Fatalf("LoadSkills() error = %v", err)
	}

	tools := toolRegistry(set, nil)
	if _, ok := findTool(tools, "Skill"); !ok {
		t.Error("Skill tool not registered")
	}

	empty := toolRegistry(&SkillSet{}, nil)
	if _, ok := findTool(empty, "Skill"); ok {
		t.Error("Skill tool registered without skills")
	}
}

func TestRunSkillForked(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "apple", "---\nname: apple\ndescription: Handles on-call.\ncontext: fork\n---\n\nRespond with exactly one word: $ARGUMENTS\n")

	set, err := LoadSkills(root)
	if err != nil {
		t.Fatalf("LoadSkills() error = %v", err)
	}

	var gotPrompt string
	runner := func(_ context.Context, prompt string) (string, error) {
		gotPrompt = prompt
		return "blueberry", nil
	}

	got, err := runSkill(context.Background(), set, runner, `{"name":"apple","args":"blueberry"}`)
	if err != nil {
		t.Fatalf("runSkill() error = %v", err)
	}

	if gotPrompt != "Respond with exactly one word: blueberry" {
		t.Errorf("subagent prompt = %q", gotPrompt)
	}

	want := "Skill apple ran in a separate context and returned: blueberry"
	if got != want {
		t.Errorf("runSkill() = %q, want %q", got, want)
	}
}

func TestExpandPromptForkStopsStacking(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "apple", "---\nname: apple\ndescription: Apple.\ncontext: fork\n---\n\napple-$ARGUMENTS\n")
	writeSkill(t, root, "grape", "---\nname: grape\ndescription: Grape.\n---\n\ngrape-$ARGUMENTS\n")

	set, err := LoadSkills(root)
	if err != nil {
		t.Fatalf("LoadSkills() error = %v", err)
	}

	apple, _ := set.Find("apple")
	assertExpanded(t, set, "/apple /grape 4127", skillPrefix(apple)+"apple-/grape 4127")
}
