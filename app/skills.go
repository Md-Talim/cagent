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
	Folder      string
	Path        string
}

func (s *Skill) Body() (string, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return "", err
	}
	_, body, err := parseSkillFile(string(data))
	return body, err
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
	return strings.TrimRight(b.String(), "\n")
}

func (s *SkillSet) Find(name string) (Skill, bool) {
	for _, sk := range s.skills {
		if sk.Folder == name || sk.Name == name {
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
	}

	if len(skills) == 0 {
		return []string{prompt}
	}

	args := strings.TrimSpace(prompt[pos:])

	messages := make([]string, 0, len(skills))
	for _, skill := range skills {
		body, err := skill.Body()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[skills] load %q: %v\n", skill.Folder, err)
			continue
		}
		messages = append(messages, parseSkillBody(body, args))
	}

	if len(messages) == 0 {
		return []string{prompt}
	}

	return messages
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

		path := filepath.Join(root, entry.Name(), "SKILL.md")
		data, err := os.ReadFile(path)
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
			Folder:      entry.Name(),
			Path:        path,
		})
	}

	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })

	return &SkillSet{skills: skills}, nil
}

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
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

func parseSlashCommand(prompt string) (name, args string, ok bool) {
	if !strings.HasPrefix(prompt, "/") {
		return "", "", false
	}
	trimmed := strings.TrimPrefix(prompt, "/")
	if trimmed == "" {
		return "", "", false
	}

	idx := strings.IndexAny(trimmed, " \t\n")
	if idx == -1 {
		return trimmed, "", true
	}
	return trimmed[:idx], strings.TrimSpace(trimmed[idx+1:]), true
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
