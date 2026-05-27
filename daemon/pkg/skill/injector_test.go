package skill

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticflow/agenticflow/shared/api"
)

func TestProviderSkillsDir(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"claude", "/work/.claude/skills"},
		{"opencode", "/work/.opencode/skills"},
		{"gemini", "/work/.gemini/skills"},
		{"kiro", "/work/.kiro/skills"},
		{"unknown", "/work/.agent_context/skills"},
		{"", "/work/.agent_context/skills"},
		{"CLAUDE", "/work/.claude/skills"},   // case-insensitive
		{"Claude", "/work/.claude/skills"},   // case-insensitive
		{"codex", "/work/.agent_context/skills"},
		{"copilot", "/work/.agent_context/skills"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := ProviderSkillsDir("/work", tt.provider)
			if got != tt.want {
				t.Errorf("ProviderSkillsDir(/work, %q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestSanitizeSkillName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"my-skill", "my-skill"},
		{"My Skill", "my-skill"},
		{"MY_SKILL_NAME", "my-skill-name"},
		{"  hello world  ", "hello-world"},
		{"foo--bar", "foo-bar"},
		{"---leading", "leading"},
		{"trailing---", "trailing"},
		{"a!b@c#d", "a-b-c-d"},
		{"UPPERCASE", "uppercase"},
		{"already-valid", "already-valid"},
		{"123-numeric", "123-numeric"},
		{"a", "a"},
		{"---", "skill"}, // all hyphens → fallback
		{"!!!", "skill"}, // all special chars → fallback
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizeSkillName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeSkillName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeSkillName_Idempotent(t *testing.T) {
	inputs := []string{
		"my-skill", "Hello World", "foo__bar", "A!B@C", "123-test",
	}
	for _, input := range inputs {
		first := SanitizeSkillName(input)
		second := SanitizeSkillName(first)
		if first != second {
			t.Errorf("SanitizeSkillName is not idempotent for %q: first=%q, second=%q", input, first, second)
		}
	}
}

func TestInjectSkills(t *testing.T) {
	workDir := t.TempDir()

	inj := &Injector{Logger: slog.Default()}

	skills := []api.TaskSkill{
		{
			Name:        "web-design",
			Description: "Web design guidelines",
			Content:     "# Web Design\n\nFollow these guidelines.",
			Files: []api.TaskSkillFile{
				{Path: "examples/button.html", Content: "<button>Click</button>"},
				{Path: "schema.json", Content: `{"type": "object"}`},
			},
		},
		{
			Name:        "go-standards",
			Description: "Go coding standards",
			Content:     "# Go Standards\n\nUse gofmt.",
			Files:       nil,
		},
	}

	err := inj.InjectSkills(workDir, "claude", skills)
	if err != nil {
		t.Fatalf("InjectSkills failed: %v", err)
	}

	// Verify SKILL.md files.
	skillMD := filepath.Join(workDir, ".claude/skills/web-design/SKILL.md")
	content, err := os.ReadFile(skillMD)
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}
	if string(content) != "# Web Design\n\nFollow these guidelines." {
		t.Errorf("unexpected SKILL.md content: %q", string(content))
	}

	// Verify supporting files.
	buttonFile := filepath.Join(workDir, ".claude/skills/web-design/examples/button.html")
	content, err = os.ReadFile(buttonFile)
	if err != nil {
		t.Fatalf("failed to read supporting file: %v", err)
	}
	if string(content) != "<button>Click</button>" {
		t.Errorf("unexpected supporting file content: %q", string(content))
	}

	schemaFile := filepath.Join(workDir, ".claude/skills/web-design/schema.json")
	content, err = os.ReadFile(schemaFile)
	if err != nil {
		t.Fatalf("failed to read schema file: %v", err)
	}
	if string(content) != `{"type": "object"}` {
		t.Errorf("unexpected schema file content: %q", string(content))
	}

	// Verify second skill.
	goMD := filepath.Join(workDir, ".claude/skills/go-standards/SKILL.md")
	content, err = os.ReadFile(goMD)
	if err != nil {
		t.Fatalf("failed to read go-standards SKILL.md: %v", err)
	}
	if string(content) != "# Go Standards\n\nUse gofmt." {
		t.Errorf("unexpected go-standards content: %q", string(content))
	}
}

func TestInjectSkills_EmptySkills(t *testing.T) {
	workDir := t.TempDir()
	inj := &Injector{Logger: slog.Default()}

	err := inj.InjectSkills(workDir, "claude", nil)
	if err != nil {
		t.Fatalf("InjectSkills with nil skills should not error: %v", err)
	}

	err = inj.InjectSkills(workDir, "claude", []api.TaskSkill{})
	if err != nil {
		t.Fatalf("InjectSkills with empty skills should not error: %v", err)
	}
}

func TestInjectSkills_UnknownProvider(t *testing.T) {
	workDir := t.TempDir()
	inj := &Injector{Logger: slog.Default()}

	skills := []api.TaskSkill{
		{Name: "test-skill", Content: "test content"},
	}

	err := inj.InjectSkills(workDir, "unknown-provider", skills)
	if err != nil {
		t.Fatalf("InjectSkills failed: %v", err)
	}

	skillMD := filepath.Join(workDir, ".agent_context/skills/test-skill/SKILL.md")
	content, err := os.ReadFile(skillMD)
	if err != nil {
		t.Fatalf("failed to read SKILL.md for unknown provider: %v", err)
	}
	if string(content) != "test content" {
		t.Errorf("unexpected content: %q", string(content))
	}
}
