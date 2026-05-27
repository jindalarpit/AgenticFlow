// Package skill provides skill injection and discovery for the daemon.
package skill

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/agenticflow/agenticflow/shared/api"
)

// Injector writes skill files into provider-native directories.
type Injector struct {
	Logger *slog.Logger
}

// InjectSkills writes all skill files to the provider-native directory
// within the given workspace directory.
func (inj *Injector) InjectSkills(workDir, provider string, skills []api.TaskSkill) error {
	if len(skills) == 0 {
		return nil
	}

	skillsDir := ProviderSkillsDir(workDir, provider)

	for _, skill := range skills {
		dirName := SanitizeSkillName(skill.Name)
		skillDir := filepath.Join(skillsDir, dirName)

		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			return fmt.Errorf("failed to create skill directory: %w", err)
		}

		// Write SKILL.md with the skill content.
		skillMDPath := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(skillMDPath, []byte(skill.Content), 0o644); err != nil {
			return fmt.Errorf("failed to write skill file: %w", err)
		}

		if inj.Logger != nil {
			inj.Logger.Debug("wrote skill", "name", skill.Name, "path", skillMDPath)
		}

		// Write supporting files at their relative paths within the skill directory.
		for _, f := range skill.Files {
			filePath := filepath.Join(skillDir, f.Path)

			// Ensure the parent directory exists for nested paths.
			if dir := filepath.Dir(filePath); dir != skillDir {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return fmt.Errorf("failed to write skill file: %w", err)
				}
			}

			if err := os.WriteFile(filePath, []byte(f.Content), 0o644); err != nil {
				return fmt.Errorf("failed to write skill file: %w", err)
			}
		}
	}

	return nil
}

// providerDirMap maps known provider names to their skills directory prefix.
var providerDirMap = map[string]string{
	"claude":   ".claude/skills",
	"opencode": ".opencode/skills",
	"gemini":   ".gemini/skills",
	"kiro":     ".kiro/skills",
}

// ProviderSkillsDir returns the provider-specific skills directory path
// for the given workspace directory and provider name.
func ProviderSkillsDir(workDir, provider string) string {
	if dir, ok := providerDirMap[strings.ToLower(provider)]; ok {
		return filepath.Join(workDir, dir)
	}
	return filepath.Join(workDir, ".agent_context/skills")
}

// nonAlphanumHyphen matches any character that is not lowercase alphanumeric or hyphen.
var nonAlphanumHyphen = regexp.MustCompile(`[^a-z0-9-]`)

// multipleHyphens matches two or more consecutive hyphens.
var multipleHyphens = regexp.MustCompile(`-{2,}`)

// SanitizeSkillName normalizes a skill name for use as a directory name.
// It lowercases the input, replaces non-alphanumeric characters (except hyphens)
// with hyphens, collapses multiple hyphens, trims leading/trailing hyphens,
// and ensures the result starts with an alphanumeric character.
// Sanitization is idempotent.
func SanitizeSkillName(name string) string {
	// Lowercase the input.
	s := strings.ToLower(name)

	// Replace non-alphanumeric (except hyphens) with hyphens.
	s = nonAlphanumHyphen.ReplaceAllString(s, "-")

	// Collapse multiple consecutive hyphens into one.
	s = multipleHyphens.ReplaceAllString(s, "-")

	// Trim leading and trailing hyphens.
	s = strings.Trim(s, "-")

	// Ensure starts with alphanumeric — strip any leading hyphens that remain.
	// (Already handled by Trim above, but be defensive.)
	for len(s) > 0 && s[0] == '-' {
		s = s[1:]
	}

	// If the result is empty after sanitization, return a fallback.
	if s == "" {
		return "skill"
	}

	return s
}
