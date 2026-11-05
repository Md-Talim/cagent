package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/openai/openai-go/v3"
)

func writeSkill(t *testing.T, root, folder, content string) {
	t.Helper()
	dir := filepath.Join(root, folder)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill %s: %v", folder, err)
	}
}

func skillPrefix(skill Skill) string {
	return fmt.Sprintf(
		"Skill: %s (located at %s)\nPaths in the instructions below are relative to that folder.\n\n",
		skill.Name,
		filepath.ToSlash(skill.Dir),
	)
}

func TestParseSkillFile(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantMeta  skillFrontmatter
		wantBody  string
		wantError bool
	}{
		{
			name:     "normal frontmatter",
			text:     "---\nname: apple\ndescription: Deploys apple.\n---\n\nRun the deploy.\n",
			wantMeta: skillFrontmatter{Name: "apple", Description: "Deploys apple."},
			wantBody: "Run the deploy.",
		},
		{
			name:     "crlf line endings",
			text:     "---\r\nname: apple\r\ndescription: Deploys apple.\r\n---\r\n\r\nRun the deploy.\r\n",
			wantMeta: skillFrontmatter{Name: "apple", Description: "Deploys apple."},
			wantBody: "Run the deploy.",
		},
		{
			name:     "bom at start",
			text:     "\ufeff---\nname: apple\ndescription: Deploys apple.\n---\nBody\n",
			wantMeta: skillFrontmatter{Name: "apple", Description: "Deploys apple."},
			wantBody: "Body",
		},
		{
			name:     "quoted description with colon",
			text:     "---\nname: apple\ndescription: \"Deploys apple: production\"\n---\nBody\n",
			wantMeta: skillFrontmatter{Name: "apple", Description: "Deploys apple: production"},
			wantBody: "Body",
		},
		{
			name:     "folded description",
			text:     "---\nname: apple\ndescription: >-\n  Deploys apple\n  to production.\n---\nBody\n",
			wantMeta: skillFrontmatter{Name: "apple", Description: "Deploys apple to production."},
			wantBody: "Body",
		},
		{
			name:     "body keeps blank lines",
			text:     "---\nname: apple\ndescription: Deploys apple.\n---\n\nline one\n\nline two\n",
			wantMeta: skillFrontmatter{Name: "apple", Description: "Deploys apple."},
			wantBody: "line one\n\nline two",
		},
		{
			name:      "missing opening delimiter",
			text:      "name: apple\n---\nBody\n",
			wantError: true,
		},
		{
			name:      "missing closing delimiter",
			text:      "---\nname: apple\nBody\n",
			wantError: true,
		},
		{
			name:      "invalid yaml",
			text:      "---\nname: [apple\ndescription: Deploys apple.\n---\nBody\n",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, body, err := parseSkillFile(tt.text)
			if tt.wantError {
				if err == nil {
					t.Fatal("parseSkillFile() error = nil, want non-nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSkillFile() error = %v", err)
			}
			if meta != tt.wantMeta {
				t.Errorf("meta = %+v, want %+v", meta, tt.wantMeta)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestLoadSkillsMissingDirectory(t *testing.T) {
	set, err := LoadSkills(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("LoadSkills() error = %v", err)
	}
	if got := set.SystemPrompt(); got != "" {
		t.Errorf("SystemPrompt() = %q, want empty", got)
	}
	if _, ok := set.Find("apple"); ok {
		t.Error("Find(\"apple\") found a skill, want none")
	}
}

func TestLoadSkills(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "grape", "---\nname: grape\ndescription: Runs grape tests.\n---\n\nGrape body\n")
	writeSkill(t, root, "apple", "---\nname: apple\ndescription: Deploys apple.\n---\n\nApple body\n")
	writeSkill(t, root, "broken", "not a skill file")

	if err := os.WriteFile(filepath.Join(root, "README.txt"), []byte("ignore me"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	set, err := LoadSkills(root)
	if err != nil {
		t.Fatalf("LoadSkills() error = %v", err)
	}

	names := make([]string, 0, 2)
	for _, sk := range set.skills {
		names = append(names, sk.Name)
	}
	if got := strings.Join(names, ","); got != "apple,grape" {
		t.Fatalf("skill names = %q, want %q", got, "apple,grape")
	}

	apple, ok := set.Find("apple")
	if !ok {
		t.Fatal("Find(\"apple\") not found")
	}
	if apple.Description != "Deploys apple." {
		t.Errorf("apple description = %q", apple.Description)
	}
	if apple.FolderName() != "apple" {
		t.Errorf("apple folder = %q", apple.FolderName())
	}

	if _, ok := set.Find("broken"); ok {
		t.Error("Find(\"broken\") found malformed skill, want none")
	}
	if _, ok := set.Find("empty"); ok {
		t.Error("Find(\"empty\") found empty skill, want none")
	}
}

func TestLoadSkillsFallsBackToFolderName(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "mango", "---\ndescription: A mango skill.\n---\nBody\n")

	set, err := LoadSkills(root)
	if err != nil {
		t.Fatalf("LoadSkills() error = %v", err)
	}

	mango, ok := set.Find("mango")
	if !ok {
		t.Fatal("Find(\"mango\") not found")
	}
	if mango.Name != "mango" {
		t.Errorf("name = %q, want %q", mango.Name, "mango")
	}
}

func TestSystemPrompt(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "apple", "---\nname: apple\ndescription: Deploys apple.\n---\n\nApple body\n")
	writeSkill(t, root, "grape", "---\nname: grape\ndescription: Runs grape tests.\n---\n\nGrape body\n")

	set, err := LoadSkills(root)
	if err != nil {
		t.Fatalf("LoadSkills() error = %v", err)
	}

	want := "You have access to the following skills:\n\n" +
		"- apple: Deploys apple.\n" +
		"- grape: Runs grape tests."

	got := set.SystemPrompt()
	if got != want {
		t.Errorf("SystemPrompt() = %q, want %q", got, want)
	}
	if strings.Contains(got, "Apple body") || strings.Contains(got, "Grape body") {
		t.Error("SystemPrompt() contains a skill body, want frontmatter only")
	}
}

func assertExpanded(t *testing.T, set *SkillSet, prompt string, want ...string) {
	t.Helper()
	got := set.ExpandPrompt(prompt)
	if !slices.Equal(got, want) {
		t.Errorf("ExpandPrompt(%q) = %q, want %q", prompt, got, want)
	}
}

func TestExpandPrompt(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "apple", "---\nname: apple\ndescription: Deploys apple.\n---\n\nRespond with exactly one word: blueberry\n\nDo not add anything else.\n")
	writeSkill(t, root, "grape", "---\nname: grape\ndescription: Runs grape tests.\n---\n\nRespond with exactly one word: grapefruit\n")

	set, err := LoadSkills(root)
	if err != nil {
		t.Fatalf("LoadSkills() error = %v", err)
	}

	const appleBody = "Respond with exactly one word: blueberry\n\nDo not add anything else."

	apple, ok := set.Find("apple")
	if !ok {
		t.Fatal("Find(\"apple\") not found")
	}
	prefix := skillPrefix(apple)

	assertExpanded(t, set, "/apple", prefix+appleBody)
	assertExpanded(t, set, "/apple please deploy", prefix+appleBody)
	assertExpanded(t, set, "Which skill fits?", "Which skill fits?")
	assertExpanded(t, set, "/mango", "/mango")
	assertExpanded(t, set, "/", "/")
	assertExpanded(t, set, "", "")
}

type testMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func agentMessages(t *testing.T, agent *Agent) []testMessage {
	t.Helper()
	data, err := json.Marshal(agent.messages)
	if err != nil {
		t.Fatalf("marshal messages: %v", err)
	}
	var msgs []testMessage
	if err := json.Unmarshal(data, &msgs); err != nil {
		t.Fatalf("unmarshal messages: %v", err)
	}
	return msgs
}

func TestNewAgentInjectsSystemPromptAndSkillBody(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "apple", "---\nname: apple\ndescription: Deploys apple.\n---\n\nRespond with exactly one word: blueberry\n")

	set, err := LoadSkills(root)
	if err != nil {
		t.Fatalf("LoadSkills() error = %v", err)
	}

	apple, ok := set.Find("apple")
	if !ok {
		t.Fatal("Find(\"apple\") not found")
	}
	want := skillPrefix(apple) + "Respond with exactly one word: blueberry"

	agent := NewAgent(openai.Client{}, "test-model", set, "/apple")
	msgs := agentMessages(t, agent)

	if len(msgs) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("messages[0].role = %q, want system", msgs[0].Role)
	}
	if !strings.Contains(msgs[0].Content, "- apple: Deploys apple.") {
		t.Errorf("system prompt = %q, want apple in skill list", msgs[0].Content)
	}
	if msgs[1].Role != "user" {
		t.Errorf("messages[1].role = %q, want user", msgs[1].Role)
	}
	if msgs[1].Content != want {
		t.Errorf("user message = %q, want %q", msgs[1].Content, want)
	}
}

func TestNewAgentWithoutSkillsHasNoSystemMessage(t *testing.T) {
	agent := NewAgent(openai.Client{}, "test-model", &SkillSet{}, "hello")
	msgs := agentMessages(t, agent)

	if len(msgs) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hello" {
		t.Errorf("message = %+v, want user message", msgs[0])
	}
}

func TestParseSkillBody(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		rawArgs string
		want    string
	}{
		{"full arguments", "Deploy $ARGUMENTS", "staging eu-west", "Deploy staging eu-west"},
		{"positional", "Deploy to $0 in region $1", "staging eu-west", "Deploy to staging in region eu-west"},
		{"reversed", "$1 $0", "mango pear", "pear mango"},
		{"indexed", "$ARGUMENTS[0] then $ARGUMENTS[1]", "a b", "a then b"},
		{"indexed before bare", "$ARGUMENTS[0]", "a b", "a"},
		{"mixed forms", "$0 / $ARGUMENTS / $1", "a b", "a / a b / b"},
		{"repeated placeholder", "$0 $0", "a", "a a"},
		{"missing positional", "x$5y", "a b", "xy"},
		{"missing indexed", "$ARGUMENTS[9]", "a", ""},
		{"double digit index", "$10", "a b", ""},
		{"no arguments no spacing", "$0$1$ARGUMENTS", "", ""},
		{"no arguments keeps spacing", "$0 $1", "", " "},
		{"literal dollar", "cost: $", "", "cost: $"},
		{"unknown dollar name", "$foo", "", "$foo"},
		{"argument is not re-expanded", "$0", "$1", "$1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseSkillBody(tt.body, tt.rawArgs); got != tt.want {
				t.Errorf("parseSkillBody(%q, %q) = %q, want %q", tt.body, tt.rawArgs, got, tt.want)
			}
		})
	}
}

func TestExpandPromptSubstitutesArguments(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "apple", `---
name: apple
description: Deploys apple.
---

Deploy to $0 in region $1. Full request was: $ARGUMENTS
`)

	set, err := LoadSkills(root)
	if err != nil {
		t.Fatalf("LoadSkills() error = %v", err)
	}

	apple, ok := set.Find("apple")
	if !ok {
		t.Fatal("Find(\"apple\") not found")
	}

	want := skillPrefix(apple) + "Deploy to staging in region eu-west. Full request was: staging eu-west"
	assertExpanded(t, set, "/apple staging eu-west", want)
}

func TestExpandPromptWithoutArguments(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "apple", "---\nname: apple\ndescription: Deploys apple.\n---\n\nArgs: [$ARGUMENTS] [$0]\n")

	set, err := LoadSkills(root)
	if err != nil {
		t.Fatalf("LoadSkills() error = %v", err)
	}

	apple, ok := set.Find("apple")
	if !ok {
		t.Fatal("Find(\"apple\") not found")
	}

	assertExpanded(t, set, "/apple", skillPrefix(apple)+"Args: [] []")
}

func TestSkillRender(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "apple", "---\nname: apple\ndescription: Apple skill.\n---\n\nblueberry-$ARGUMENTS\n")

	set, err := LoadSkills(root)
	if err != nil {
		t.Fatalf("LoadSkills() error = %v", err)
	}

	apple, ok := set.Find("apple")
	if !ok {
		t.Fatal("Find(\"apple\") not found")
	}

	got, err := apple.Render("4127")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	want := skillPrefix(apple) + "blueberry-4127"
	if got != want {
		t.Errorf("Render() = %q, want %q", got, want)
	}
}

func TestLoadSkillsRelativeRoot(t *testing.T) {
	t.Chdir(t.TempDir())
	root := filepath.Join(".claude", "skills")
	writeSkill(t, root, "apple", "---\nname: apple\ndescription: Apple skill.\n---\n\nRun `scripts/check.sh`.\n")

	set, err := LoadSkills(root)
	if err != nil {
		t.Fatalf("LoadSkills() error = %v", err)
	}

	apple, ok := set.Find("apple")
	if !ok {
		t.Fatal("Find(\"apple\") not found")
	}

	wantDir := filepath.Join(".claude", "skills", "apple")
	if apple.Dir != wantDir {
		t.Errorf("Dir = %q, want %q", apple.Dir, wantDir)
	}

	got := set.ExpandPrompt("/apple")[0]
	want := "Skill: apple (located at " + filepath.ToSlash(wantDir) + ")\nPaths in the instructions below are relative to that folder.\n\nRun `scripts/check.sh`."
	if got != want {
		t.Errorf("ExpandPrompt() = %q, want %q", got, want)
	}
}

func TestExpandPromptStacksSkills(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "apple", "---\nname: apple\ndescription: Apple skill.\n---\n\nblueberry-$ARGUMENTS\n")
	writeSkill(t, root, "grape", "---\nname: grape\ndescription: Grape skill.\n---\n\ncherry-$ARGUMENTS\n")
	writeSkill(t, root, "pear", "---\nname: pear\ndescription: Pear skill.\n---\n\npear-$ARGUMENTS\n")

	set, err := LoadSkills(root)
	if err != nil {
		t.Fatalf("LoadSkills() error = %v", err)
	}

	apple, ok := set.Find("apple")
	if !ok {
		t.Fatal("Find(\"apple\") not found")
	}
	grape, ok := set.Find("grape")
	if !ok {
		t.Fatal("Find(\"grape\") not found")
	}
	appleMsg := func(body string) string { return skillPrefix(apple) + body }
	grapeMsg := func(body string) string { return skillPrefix(grape) + body }

	tests := []struct {
		name   string
		prompt string
		want   []string
	}{
		{
			name:   "two skills share arguments",
			prompt: "/apple /grape 4127",
			want:   []string{appleMsg("blueberry-4127"), grapeMsg("cherry-4127")},
		},
		{
			name:   "stops at first argument",
			prompt: "/apple 4127 /grape",
			want:   []string{appleMsg("blueberry-4127 /grape")},
		},
		{
			name:   "stops at unknown skill",
			prompt: "/apple /kiwi 4127",
			want:   []string{appleMsg("blueberry-/kiwi 4127")},
		},
		{
			name:   "unknown first skill",
			prompt: "/kiwi /apple 4127",
			want:   []string{"/kiwi /apple 4127"},
		},
		{
			name:   "no arguments",
			prompt: "/apple /grape",
			want:   []string{appleMsg("blueberry-"), grapeMsg("cherry-")},
		},
		{
			name:   "never invoked skill stays out",
			prompt: "/apple /grape 4127",
			want:   []string{appleMsg("blueberry-4127"), grapeMsg("cherry-4127")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertExpanded(t, set, tt.prompt, tt.want...)
		})
	}
}

func TestNewAgentStacksSkillMessages(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "apple", "---\nname: apple\ndescription: Apple skill.\n---\n\nblueberry-$ARGUMENTS\n")
	writeSkill(t, root, "grape", "---\nname: grape\ndescription: Grape skill.\n---\n\ncherry-$ARGUMENTS\n")

	set, err := LoadSkills(root)
	if err != nil {
		t.Fatalf("LoadSkills() error = %v", err)
	}

	apple, ok := set.Find("apple")
	if !ok {
		t.Fatal("Find(\"apple\") not found")
	}
	grape, ok := set.Find("grape")
	if !ok {
		t.Fatal("Find(\"grape\") not found")
	}
	want1 := skillPrefix(apple) + "blueberry-4127"
	want2 := skillPrefix(grape) + "cherry-4127"

	agent := NewAgent(openai.Client{}, "test-model", set, "/apple /grape 4127")
	msgs := agentMessages(t, agent)

	if len(msgs) != 3 {
		t.Fatalf("len(messages) = %d, want 3", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("messages[0].role = %q, want system", msgs[0].Role)
	}
	if msgs[1].Role != "user" || msgs[1].Content != want1 {
		t.Errorf("messages[1] = %+v, want %q", msgs[1], want1)
	}
	if msgs[2].Role != "user" || msgs[2].Content != want2 {
		t.Errorf("messages[2] = %+v, want %q", msgs[2], want2)
	}
}
