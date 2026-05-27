package skill

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverLocalSkills_NonExistentDirs(t *testing.T) {
	logger := slog.Default()
	dirs := []providerDir{
		{Provider: "claude", Path: "/nonexistent/path/skills"},
		{Provider: "opencode", Path: "/another/nonexistent/path"},
	}

	skills, err := discoverLocalSkillsIn(logger, dirs)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills, got %d", len(skills))
	}
}

func TestDiscoverLocalSkills_FindsSkillWithFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `---
name: my-skill
description: A test skill
---
# My Skill

Instructions here.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	logger := slog.Default()
	dirs := []providerDir{
		{Provider: "claude", Path: tmp},
	}

	skills, err := discoverLocalSkillsIn(logger, dirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "my-skill" {
		t.Errorf("expected name 'my-skill', got %q", skills[0].Name)
	}
	if skills[0].Description != "A test skill" {
		t.Errorf("expected description 'A test skill', got %q", skills[0].Description)
	}
	if skills[0].Provider != "claude" {
		t.Errorf("expected provider 'claude', got %q", skills[0].Provider)
	}
}

func TestDiscoverLocalSkills_FallsBackToDirName(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "web-design")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// No frontmatter, just markdown content.
	content := `# Web Design Guidelines

Follow these rules...
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	logger := slog.Default()
	dirs := []providerDir{
		{Provider: "kiro", Path: tmp},
	}

	skills, err := discoverLocalSkillsIn(logger, dirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "web-design" {
		t.Errorf("expected name 'web-design', got %q", skills[0].Name)
	}
}

func TestDiscoverLocalSkills_DepthLimit(t *testing.T) {
	tmp := t.TempDir()

	// Create a skill at depth 2 (within limit: root/subdir/skill-dir/SKILL.md = depth 3 from root).
	shallowDir := filepath.Join(tmp, "level1", "shallow-skill")
	if err := os.MkdirAll(shallowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shallowDir, "SKILL.md"), []byte("---\nname: shallow\n---\n# Shallow"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a skill at depth 5 (beyond limit).
	deepDir := filepath.Join(tmp, "a", "b", "c", "d", "deep-skill")
	if err := os.MkdirAll(deepDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deepDir, "SKILL.md"), []byte("---\nname: deep\n---\n# Deep"), 0o644); err != nil {
		t.Fatal(err)
	}

	logger := slog.Default()
	dirs := []providerDir{
		{Provider: "claude", Path: tmp},
	}

	skills, err := discoverLocalSkillsIn(logger, dirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only the shallow skill should be found (within depth 4).
	// The deep skill is at depth > 4 from root and should be skipped.
	for _, s := range skills {
		if s.Name == "deep" {
			t.Errorf("deep skill should not be discovered (exceeds depth limit)")
		}
	}
}

func TestDiscoverLocalSkills_SkipsLargeFiles(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "big-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a file larger than 1MB.
	bigContent := make([]byte, MaxFileSize+1)
	for i := range bigContent {
		bigContent[i] = 'x'
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), bigContent, 0o644); err != nil {
		t.Fatal(err)
	}

	logger := slog.Default()
	dirs := []providerDir{
		{Provider: "claude", Path: tmp},
	}

	skills, err := discoverLocalSkillsIn(logger, dirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills (large file should be skipped), got %d", len(skills))
	}
}

func TestDiscoverLocalSkills_MultipleProviders(t *testing.T) {
	tmp1 := t.TempDir()
	tmp2 := t.TempDir()

	// Provider 1 skill.
	skillDir1 := filepath.Join(tmp1, "skill-a")
	if err := os.MkdirAll(skillDir1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir1, "SKILL.md"), []byte("---\nname: skill-a\n---\n# A"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Provider 2 skill.
	skillDir2 := filepath.Join(tmp2, "skill-b")
	if err := os.MkdirAll(skillDir2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir2, "SKILL.md"), []byte("---\nname: skill-b\n---\n# B"), 0o644); err != nil {
		t.Fatal(err)
	}

	logger := slog.Default()
	dirs := []providerDir{
		{Provider: "claude", Path: tmp1},
		{Provider: "opencode", Path: tmp2},
	}

	skills, err := discoverLocalSkillsIn(logger, dirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	providers := map[string]bool{}
	for _, s := range skills {
		providers[s.Provider] = true
	}
	if !providers["claude"] || !providers["opencode"] {
		t.Errorf("expected skills from both providers, got: %v", providers)
	}
}

func TestCountPathDepth(t *testing.T) {
	tests := []struct {
		rel   string
		depth int
	}{
		{".", 0},
		{"", 0},
		{"a", 1},
		{"a/b", 2},
		{"a/b/c", 3},
		{"a/b/c/d", 4},
		{"a/b/c/d/e", 5},
	}

	for _, tt := range tests {
		got := countPathDepth(tt.rel)
		if got != tt.depth {
			t.Errorf("countPathDepth(%q) = %d, want %d", tt.rel, got, tt.depth)
		}
	}
}

func TestParseFrontmatterFields(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    map[string]string
	}{
		{
			name:    "valid frontmatter",
			content: "---\nname: my-skill\ndescription: A cool skill\n---\n# Body",
			want:    map[string]string{"name": "my-skill", "description": "A cool skill"},
		},
		{
			name:    "quoted values",
			content: "---\nname: \"quoted-name\"\ndescription: \"has: colons\"\n---\n# Body",
			want:    map[string]string{"name": "quoted-name", "description": "has: colons"},
		},
		{
			name:    "no frontmatter",
			content: "# Just a heading\nSome content",
			want:    map[string]string{},
		},
		{
			name:    "empty frontmatter",
			content: "---\n---\n# Body",
			want:    map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFrontmatterFields(tt.content)
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("field %q = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}
