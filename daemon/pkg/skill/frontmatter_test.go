package skill

import (
	"testing"
)

func TestHasFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "content with frontmatter",
			content: "---\nname: my-skill\n---\nBody content",
			want:    true,
		},
		{
			name:    "content without frontmatter",
			content: "# My Skill\n\nSome content here.",
			want:    false,
		},
		{
			name:    "empty content",
			content: "",
			want:    false,
		},
		{
			name:    "only opening delimiter",
			content: "---\nname: test\n",
			want:    true,
		},
		{
			name:    "dashes in middle of content",
			content: "Some text\n---\nMore text",
			want:    false,
		},
		{
			name:    "frontmatter with CRLF",
			content: "---\r\nname: test\r\n---\r\nBody",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasFrontmatter(tt.content)
			if got != tt.want {
				t.Errorf("HasFrontmatter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantFields map[string]string
		wantBody   string
		wantErr    bool
	}{
		{
			name:    "valid frontmatter with body",
			content: "---\nname: my-skill\ndescription: \"A test skill\"\n---\n# Body\n\nContent here.",
			wantFields: map[string]string{
				"name":        "my-skill",
				"description": "A test skill",
			},
			wantBody: "# Body\n\nContent here.",
			wantErr:  false,
		},
		{
			name:       "no frontmatter",
			content:    "# Just a heading\n\nSome content.",
			wantFields: nil,
			wantBody:   "# Just a heading\n\nSome content.",
			wantErr:    false,
		},
		{
			name:    "frontmatter without closing delimiter",
			content: "---\nname: broken\n",
			wantFields: nil,
			wantBody:   "---\nname: broken\n",
			wantErr:    true,
		},
		{
			name:    "empty frontmatter",
			content: "---\n---\nBody only.",
			wantFields: map[string]string{},
			wantBody:   "Body only.",
			wantErr:    false,
		},
		{
			name:    "frontmatter with unquoted value",
			content: "---\nname: test-skill\ndescription: Simple description\n---\nBody",
			wantFields: map[string]string{
				"name":        "test-skill",
				"description": "Simple description",
			},
			wantBody: "Body",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, body, err := ParseFrontmatter(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFrontmatter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if tt.wantFields == nil && fields != nil {
				t.Errorf("ParseFrontmatter() fields = %v, want nil", fields)
				return
			}
			if tt.wantFields != nil {
				if len(fields) != len(tt.wantFields) {
					t.Errorf("ParseFrontmatter() fields count = %d, want %d", len(fields), len(tt.wantFields))
					return
				}
				for k, v := range tt.wantFields {
					if fields[k] != v {
						t.Errorf("ParseFrontmatter() fields[%q] = %q, want %q", k, fields[k], v)
					}
				}
			}
			if body != tt.wantBody {
				t.Errorf("ParseFrontmatter() body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestEnsureFrontmatter(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		skillName   string
		description string
		want        string
	}{
		{
			name:        "no frontmatter - synthesize",
			content:     "# My Skill\n\nDo things.",
			skillName:   "my-skill",
			description: "A helpful skill",
			want:        "---\nname: my-skill\ndescription: \"A helpful skill\"\n---\n# My Skill\n\nDo things.",
		},
		{
			name:        "no frontmatter - empty description",
			content:     "# My Skill\n\nDo things.",
			skillName:   "my-skill",
			description: "",
			want:        "---\nname: my-skill\n---\n# My Skill\n\nDo things.",
		},
		{
			name:        "has frontmatter with name - unchanged",
			content:     "---\nname: existing-name\ndescription: \"Existing\"\n---\n# Body",
			skillName:   "new-name",
			description: "New description",
			want:        "---\nname: existing-name\ndescription: \"Existing\"\n---\n# Body",
		},
		{
			name:        "has frontmatter without name - augment",
			content:     "---\ndescription: \"Some desc\"\n---\n# Body",
			skillName:   "my-skill",
			description: "Ignored",
			want:        "---\nname: my-skill\ndescription: \"Some desc\"\n---\n# Body",
		},
		{
			name:        "description with special characters",
			content:     "# Content",
			skillName:   "my-skill",
			description: `She said "hello" and\n left`,
			want:        "---\nname: my-skill\ndescription: \"She said \\\"hello\\\" and\\\\n left\"\n---\n# Content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EnsureFrontmatter(tt.content, tt.skillName, tt.description)
			if got != tt.want {
				t.Errorf("EnsureFrontmatter() =\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

func TestQuoteYAMLScalar(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple string",
			input: "hello world",
			want:  `"hello world"`,
		},
		{
			name:  "string with double quotes",
			input: `say "hi"`,
			want:  `"say \"hi\""`,
		},
		{
			name:  "string with backslash",
			input: `path\to\file`,
			want:  `"path\\to\\file"`,
		},
		{
			name:  "string with newline",
			input: "line1\nline2",
			want:  `"line1\nline2"`,
		},
		{
			name:  "empty string",
			input: "",
			want:  `""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quoteYAMLScalar(tt.input)
			if got != tt.want {
				t.Errorf("quoteYAMLScalar(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
