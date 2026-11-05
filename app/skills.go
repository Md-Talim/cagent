package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Skill struct {
	Name        string
	Description string
	Dir         string // e.g. .claude/skills/apple
	Context     string // e.g. "fork"
}

func (s *Skill) Body() (string, error) {
	data, err := os.ReadFile(s.FilePath())
	if err != nil {
		return "", err
	}
	_, body, err := parseSkillFile(string(data))
	return body, err
}

func (s *Skill) Render(args string) (string, error) {
	body, err := s.Body()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"Skill: %s (located at %s)\nPaths in the instructions below are relative to that folder.\n\n%s",
		s.Name,
		filepath.ToSlash(s.Dir),
		parseSkillBody(body, args),
	), nil
}

func (s Skill) FilePath() string {
	return filepath.Join(s.Dir, "SKILL.md")
}

func (s Skill) FolderName() string {
	return filepath.Base(s.Dir)
}

func (s Skill) IsForked() bool {
	return s.Context == "fork"
}

type SkillSet struct {
	skills []Skill
}

func (s *SkillSet) SystemPrompt() string {
	if len(s.skills) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("You have access to the following skills:\n\n")
	for _, sk := range s.skills {
		fmt.Fprintf(&b, "- %s: %s\n", sk.Name, sk.Description)
	}
	b.WriteString(
		"\nIf a skill matches the user's request, call the Skill tool with its name\n" +
			"and follow the instructions it returns.",
	)
	return b.String()
}

func (s *SkillSet) Find(name string) (Skill, bool) {
	for _, sk := range s.skills {
		if sk.FolderName() == name || sk.Name == name {
			return sk, true
		}
	}
	return Skill{}, false
}

func (s *SkillSet) ExpandPrompt(prompt string) []string {
	pos := 0
	var skills []Skill

	for {
		for pos < len(prompt) && isSpace(prompt[pos]) {
			pos++
		}
		if pos >= len(prompt) || prompt[pos] != '/' {
			break
		}

		end := pos
		for end < len(prompt) && !isSpace(prompt[end]) {
			end++
		}

		name := prompt[pos+1 : end]
		skill, found := s.Find(name)
		if !found {
			break
		}

		skills = append(skills, skill)
		pos = end
		if skill.IsForked() {
			break
		}
	}

	if len(skills) == 0 {
		return []string{prompt}
	}

	args := strings.TrimSpace(prompt[pos:])

	messages := make([]string, 0, len(skills))
	for _, skill := range skills {
		content, err := skill.Render(args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[skills] load %q: %v\n", skill.Name, err)
			continue
		}
		messages = append(messages, content)
	}

	if len(messages) == 0 {
		return []string{prompt}
	}

	return messages
}

func (s *SkillSet) Len() int {
	return len(s.skills)
}

func LoadSkills(root string) (*SkillSet, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return &SkillSet{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read skill directory: %w", err)
	}

	var skills []Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dir := filepath.Join(root, entry.Name())
		data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "[skills] skipping %s: %v\n", entry.Name(), err)
			continue
		}

		meta, _, err := parseSkillFile(string(data))
		if err != nil {
			fmt.Fprintf(os.Stderr, "[skills] skipping %s: %v\n", entry.Name(), err)
			continue
		}

		name := meta.Name
		if name == "" {
			name = entry.Name()
		}

		skills = append(skills, Skill{
			Name:        name,
			Description: meta.Description,
			Dir:         dir,
			Context:     meta.Context,
		})
	}

	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })

	return &SkillSet{skills: skills}, nil
}

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Context     string `yaml:"context"`
}

func parseSkillFile(text string) (skillFrontmatter, string, error) {
	text = strings.TrimPrefix(text, "\ufeff")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return skillFrontmatter{}, "", errors.New("missing opening ---")
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return skillFrontmatter{}, "", errors.New("missing closing ---")
	}

	var meta skillFrontmatter
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &meta); err != nil {
		return skillFrontmatter{}, "", fmt.Errorf("parse frontmatter: %w", err)
	}

	body := strings.TrimSpace(strings.Join(lines[end+1:], "\n"))
	return meta, body, nil
}

var argumentPlaceholder = regexp.MustCompile(`\$ARGUMENTS\[(\d+)\]|\$ARGUMENTS|\$(\d+)`)

func parseSkillBody(body, rawArgs string) string {
	args := strings.Fields(rawArgs)

	return argumentPlaceholder.ReplaceAllStringFunc(body, func(match string) string {
		switch {
		case strings.HasPrefix(match, "$ARGUMENTS["):
			index := strings.TrimSuffix(strings.TrimPrefix(match, "$ARGUMENTS["), "]")
			i, err := strconv.Atoi(index)
			if err != nil {
				return match
			}
			return argAt(args, i)
		case match == "$ARGUMENTS":
			return rawArgs

		default:
			i, err := strconv.Atoi(strings.TrimPrefix(match, "$"))
			if err != nil {
				return match
			}
			return argAt(args, i)
		}
	})
}

func argAt(args []string, i int) string {
	if i < 0 || i >= len(args) {
		return ""
	}
	return args[i]
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
