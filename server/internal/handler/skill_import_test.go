package handler

import (
	"testing"
)

func TestNormalizeGitHubURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "github blob URL",
			input:    "https://github.com/owner/repo/blob/main/skills/my-skill/SKILL.md",
			expected: "https://raw.githubusercontent.com/owner/repo/main/skills/my-skill/SKILL.md",
		},
		{
			name:     "github blob URL with branch ref",
			input:    "https://github.com/user/project/blob/feature/branch/path/to/file.md",
			expected: "https://raw.githubusercontent.com/user/project/feature/branch/path/to/file.md",
		},
		{
			name:     "already raw URL",
			input:    "https://raw.githubusercontent.com/owner/repo/main/SKILL.md",
			expected: "https://raw.githubusercontent.com/owner/repo/main/SKILL.md",
		},
		{
			name:     "non-github URL",
			input:    "https://example.com/skills/my-skill.md",
			expected: "https://example.com/skills/my-skill.md",
		},
		{
			name:     "github non-blob URL",
			input:    "https://github.com/owner/repo/tree/main/skills",
			expected: "https://github.com/owner/repo/tree/main/skills",
		},
		{
			name:     "http github blob URL",
			input:    "http://github.com/owner/repo/blob/main/SKILL.md",
			expected: "https://raw.githubusercontent.com/owner/repo/main/SKILL.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeGitHubURL(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeGitHubURL(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseFrontmatterFields(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantName    string
		wantDesc    string
	}{
		{
			name: "valid frontmatter with name and description",
			content: `---
name: my-skill
description: A useful skill
---
# Content here`,
			wantName: "my-skill",
			wantDesc: "A useful skill",
		},
		{
			name: "frontmatter with quoted description",
			content: `---
name: test-skill
description: "A skill with: special chars"
---
Body`,
			wantName: "test-skill",
			wantDesc: "A skill with: special chars",
		},
		{
			name:     "no frontmatter",
			content:  "# Just a heading\n\nSome content",
			wantName: "",
			wantDesc: "",
		},
		{
			name: "frontmatter with only name",
			content: `---
name: only-name
---
Content`,
			wantName: "only-name",
			wantDesc: "",
		},
		{
			name:     "empty content",
			content:  "",
			wantName: "",
			wantDesc: "",
		},
		{
			name: "frontmatter with single-quoted values",
			content: `---
name: 'quoted-name'
description: 'quoted desc'
---
Body`,
			wantName: "quoted-name",
			wantDesc: "quoted desc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, desc := parseFrontmatterFields(tt.content)
			if name != tt.wantName {
				t.Errorf("parseFrontmatterFields() name = %q, want %q", name, tt.wantName)
			}
			if desc != tt.wantDesc {
				t.Errorf("parseFrontmatterFields() desc = %q, want %q", desc, tt.wantDesc)
			}
		})
	}
}

func TestDeriveNameFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "simple markdown file",
			url:      "https://example.com/skills/my-skill.md",
			expected: "my-skill",
		},
		{
			name:     "SKILL.md file",
			url:      "https://github.com/owner/repo/blob/main/skills/web-design/SKILL.md",
			expected: "skill",
		},
		{
			name:     "file with underscores",
			url:      "https://example.com/my_cool_skill.md",
			expected: "my-cool-skill",
		},
		{
			name:     "file with uppercase",
			url:      "https://example.com/MySkill.md",
			expected: "myskill",
		},
		{
			name:     "raw githubusercontent URL",
			url:      "https://raw.githubusercontent.com/owner/repo/main/coding-standards.md",
			expected: "coding-standards",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveNameFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("deriveNameFromURL(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestSanitizeToSkillName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "already valid",
			input:    "my-skill",
			expected: "my-skill",
		},
		{
			name:     "uppercase",
			input:    "MySkill",
			expected: "myskill",
		},
		{
			name:     "underscores",
			input:    "my_cool_skill",
			expected: "my-cool-skill",
		},
		{
			name:     "leading hyphens",
			input:    "--leading",
			expected: "leading",
		},
		{
			name:     "special characters",
			input:    "skill@v2!",
			expected: "skillv2",
		},
		{
			name:     "spaces",
			input:    "my skill name",
			expected: "my-skill-name",
		},
		{
			name:     "multiple hyphens",
			input:    "a---b",
			expected: "a-b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeToSkillName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeToSkillName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
