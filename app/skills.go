package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

func (s *SkillSet) ExpandPrompt(prompt string) string {
	name, _, ok := parseSlashCommand(prompt)
	if !ok {
		return prompt
	}

	skill, found := s.Find(name)
	if !found {
		fmt.Fprintf(os.Stderr, "[skills] unknown skill %q\n", name)
		return prompt
	}

	body, err := skill.Body()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[skills] load %q: %v\n", name, err)
		return prompt
	}

	return body
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
