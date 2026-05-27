package skill

import (
	"fmt"
	"strings"
)

// EnsureFrontmatter ensures the skill content has valid YAML frontmatter
// with at minimum a `name` field. Returns the content with frontmatter.
//
// Behavior:
//  1. If content has no frontmatter → synthesize a block with name and description.
//  2. If content has frontmatter with a `name` field → return unchanged.
//  3. If content has frontmatter but no `name` field → prepend name as first key.
func EnsureFrontmatter(content, skillName, description string) string {
	if !HasFrontmatter(content) {
		// Synthesize frontmatter block before the content.
		var sb strings.Builder
		sb.WriteString("---\n")
		sb.WriteString("name: ")
		sb.WriteString(skillName)
		sb.WriteString("\n")
		if description != "" {
			sb.WriteString("description: ")
			sb.WriteString(quoteYAMLScalar(description))
			sb.WriteString("\n")
		}
		sb.WriteString("---\n")
		sb.WriteString(content)
		return sb.String()
	}

	// Content has frontmatter — check if name field exists.
	fields, _, err := ParseFrontmatter(content)
	if err != nil {
		// Malformed frontmatter — treat as no frontmatter and synthesize.
		var sb strings.Builder
		sb.WriteString("---\n")
		sb.WriteString("name: ")
		sb.WriteString(skillName)
		sb.WriteString("\n")
		if description != "" {
			sb.WriteString("description: ")
			sb.WriteString(quoteYAMLScalar(description))
			sb.WriteString("\n")
		}
		sb.WriteString("---\n")
		sb.WriteString(content)
		return sb.String()
	}

	if _, hasName := fields["name"]; hasName {
		// Already has name — return unchanged.
		return content
	}

	// Has frontmatter but no name field — prepend name as first key in existing block.
	return insertNameIntoFrontmatter(content, skillName)
}

// ParseFrontmatter extracts frontmatter fields from skill content.
// Returns the frontmatter map and the body content after frontmatter.
// Returns an error if the content has an opening --- but no closing ---.
func ParseFrontmatter(content string) (map[string]string, string, error) {
	if !HasFrontmatter(content) {
		return nil, content, nil
	}

	// Find the closing --- delimiter.
	// The opening --- is on the first line; find the next --- on its own line.
	lines := strings.SplitAfter(content, "\n")
	if len(lines) < 2 {
		return nil, content, fmt.Errorf("frontmatter block not closed")
	}

	// Skip the first line (opening ---).
	closingIdx := -1
	for i := 1; i < len(lines); i++ {
		trimmed := strings.TrimRight(lines[i], "\n\r")
		if trimmed == "---" {
			closingIdx = i
			break
		}
	}

	if closingIdx == -1 {
		return nil, content, fmt.Errorf("frontmatter block not closed")
	}

	// Parse the frontmatter lines between opening and closing ---.
	fields := make(map[string]string)
	for i := 1; i < closingIdx; i++ {
		line := strings.TrimRight(lines[i], "\n\r")
		if line == "" {
			continue
		}
		key, value := parseYAMLLine(line)
		if key != "" {
			fields[key] = value
		}
	}

	// Body is everything after the closing --- line.
	var body strings.Builder
	for i := closingIdx + 1; i < len(lines); i++ {
		body.WriteString(lines[i])
	}

	return fields, body.String(), nil
}

// HasFrontmatter checks if content starts with a YAML frontmatter block (---).
func HasFrontmatter(content string) bool {
	trimmed := strings.TrimLeft(content, " \t")
	return strings.HasPrefix(trimmed, "---\n") || strings.HasPrefix(trimmed, "---\r\n")
}

// insertNameIntoFrontmatter inserts `name: <skillName>` as the first key
// in an existing frontmatter block.
func insertNameIntoFrontmatter(content, skillName string) string {
	lines := strings.SplitAfter(content, "\n")
	var sb strings.Builder

	// Write the opening --- line.
	sb.WriteString(lines[0])
	// Insert name as first key.
	sb.WriteString("name: ")
	sb.WriteString(skillName)
	sb.WriteString("\n")
	// Write the rest of the content.
	for i := 1; i < len(lines); i++ {
		sb.WriteString(lines[i])
	}

	return sb.String()
}

// parseYAMLLine parses a simple "key: value" YAML line.
// Returns empty key for lines that don't match the pattern.
func parseYAMLLine(line string) (string, string) {
	idx := strings.Index(line, ":")
	if idx < 1 {
		return "", ""
	}
	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])
	// Remove surrounding quotes from value if present.
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
	}
	return key, value
}

// quoteYAMLScalar wraps a string in double quotes and escapes internal
// double quotes and backslashes for safe YAML serialization.
func quoteYAMLScalar(s string) string {
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	escaped = strings.ReplaceAll(escaped, "\n", `\n`)
	return `"` + escaped + `"`
}
