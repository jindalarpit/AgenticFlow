// Package skill provides skill injection, frontmatter handling, and local
// skill discovery for the AgenticFlow daemon.
package skill

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Discovery limits.
const (
	MaxDepth        = 4
	MaxFileSize     = 1 << 20 // 1 MB
	MaxBundleSize   = 8 << 20 // 8 MB
	MaxFilesPerSkill = 128
)

// DiscoveredSkill represents a skill found during local directory scanning.
type DiscoveredSkill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SourcePath  string `json:"source_path"`
	Provider    string `json:"provider"`
}

// providerDir maps a provider name to its local skills directory path
// relative to the user's home directory.
type providerDir struct {
	Provider string
	Path     string // relative to home dir
}

// localProviderDirs returns the list of provider directories to scan.
func localProviderDirs(home string) []providerDir {
	return []providerDir{
		{Provider: "claude", Path: filepath.Join(home, ".claude", "skills")},
		{Provider: "opencode", Path: filepath.Join(home, ".config", "opencode", "skills")},
		{Provider: "gemini", Path: filepath.Join(home, ".config", "gemini", "skills")},
		{Provider: "kiro", Path: filepath.Join(home, ".kiro", "skills")},
	}
}

// DiscoverLocalSkills scans provider-specific directories for skill definitions.
// It looks in:
//   - ~/.claude/skills/
//   - ~/.config/opencode/skills/
//   - ~/.config/gemini/skills/
//   - ~/.kiro/skills/
//
// For each directory that exists, it walks subdirectories looking for SKILL.md
// files. It enforces a depth limit of 4 levels, skips files > 1MB, skips
// bundles > 8MB total, and caps at 128 files per skill. Non-existent
// directories are skipped silently.
func DiscoverLocalSkills(logger *slog.Logger) ([]DiscoveredSkill, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine home directory: %w", err)
	}

	return discoverLocalSkillsIn(logger, localProviderDirs(home))
}

// discoverLocalSkillsIn is the internal implementation that accepts explicit
// provider directories, making it testable without depending on the real home dir.
func discoverLocalSkillsIn(logger *slog.Logger, dirs []providerDir) ([]DiscoveredSkill, error) {
	var skills []DiscoveredSkill

	for _, pd := range dirs {
		info, err := os.Stat(pd.Path)
		if err != nil || !info.IsDir() {
			// Skip non-existent or non-directory paths silently.
			continue
		}

		discovered, err := scanProviderDir(logger, pd.Provider, pd.Path)
		if err != nil {
			logger.Warn("error scanning provider directory",
				"provider", pd.Provider,
				"path", pd.Path,
				"error", err,
			)
			continue
		}
		skills = append(skills, discovered...)
	}

	return skills, nil
}

// scanProviderDir scans a single provider's skills directory for SKILL.md files.
func scanProviderDir(logger *slog.Logger, provider, rootDir string) ([]DiscoveredSkill, error) {
	var skills []DiscoveredSkill

	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", rootDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			// Check if SKILL.md is directly in the root (depth 0 skill).
			if strings.EqualFold(entry.Name(), "SKILL.md") {
				skill, err := processSkillFile(logger, provider, rootDir, filepath.Join(rootDir, entry.Name()))
				if err != nil {
					logger.Warn("skipping skill file",
						"path", filepath.Join(rootDir, entry.Name()),
						"error", err,
					)
					continue
				}
				if skill != nil {
					skills = append(skills, *skill)
				}
			}
			continue
		}

		// Each subdirectory is a potential skill directory.
		skillDir := filepath.Join(rootDir, entry.Name())
		skill, err := discoverSkillInDir(logger, provider, skillDir, rootDir)
		if err != nil {
			logger.Warn("skipping skill directory",
				"path", skillDir,
				"error", err,
			)
			continue
		}
		if skill != nil {
			skills = append(skills, *skill)
		}
	}

	return skills, nil
}

// discoverSkillInDir looks for a SKILL.md file within a skill directory,
// respecting depth and size limits.
func discoverSkillInDir(logger *slog.Logger, provider, skillDir, rootDir string) (*DiscoveredSkill, error) {
	var skillMDPath string
	var bundleSize int64
	var fileCount int

	err := filepath.WalkDir(skillDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip entries we can't read
		}

		// Enforce depth limit relative to rootDir.
		rel, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			return nil
		}
		depth := countPathDepth(rel)
		if depth > MaxDepth {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		// Count files per skill.
		fileCount++
		if fileCount > MaxFilesPerSkill {
			return fmt.Errorf("exceeded max files per skill (%d)", MaxFilesPerSkill)
		}

		// Check individual file size.
		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}
		if info.Size() > MaxFileSize {
			logger.Warn("skipping file exceeding size limit",
				"path", path,
				"size", info.Size(),
				"limit", MaxFileSize,
			)
			return nil
		}

		// Track bundle size.
		bundleSize += info.Size()
		if bundleSize > MaxBundleSize {
			return fmt.Errorf("skill bundle exceeds %d bytes", MaxBundleSize)
		}

		// Look for SKILL.md.
		if strings.EqualFold(filepath.Base(path), "SKILL.md") && skillMDPath == "" {
			skillMDPath = path
		}

		return nil
	})

	if err != nil {
		logger.Warn("skipping skill due to limit violation",
			"dir", skillDir,
			"error", err,
		)
		return nil, nil
	}

	if skillMDPath == "" {
		return nil, nil
	}

	return processSkillFile(logger, provider, skillDir, skillMDPath)
}

// processSkillFile reads a SKILL.md file and extracts skill metadata.
func processSkillFile(logger *slog.Logger, provider, skillDir, skillMDPath string) (*DiscoveredSkill, error) {
	info, err := os.Stat(skillMDPath)
	if err != nil {
		return nil, err
	}
	if info.Size() > MaxFileSize {
		logger.Warn("skipping SKILL.md exceeding size limit",
			"path", skillMDPath,
			"size", info.Size(),
		)
		return nil, nil
	}

	content, err := os.ReadFile(skillMDPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", skillMDPath, err)
	}

	name, description := extractSkillMetadata(string(content), skillDir)

	return &DiscoveredSkill{
		Name:        name,
		Description: description,
		SourcePath:  skillMDPath,
		Provider:    provider,
	}, nil
}

// extractSkillMetadata parses frontmatter from SKILL.md content to get
// name and description. Falls back to the directory name for the skill name.
func extractSkillMetadata(content, skillDir string) (name, description string) {
	fm := parseFrontmatterFields(content)

	name = fm["name"]
	if name == "" {
		// Fall back to directory name.
		name = filepath.Base(skillDir)
	}

	description = fm["description"]
	return name, description
}

// parseFrontmatterFields extracts key-value pairs from YAML frontmatter.
// It handles the simple case of `key: value` lines between `---` delimiters.
func parseFrontmatterFields(content string) map[string]string {
	fields := make(map[string]string)

	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return fields
	}

	// Find the closing ---
	rest := content[3:]
	// Skip the newline after opening ---
	if idx := strings.IndexByte(rest, '\n'); idx >= 0 {
		rest = rest[idx+1:]
	} else {
		return fields
	}

	endIdx := strings.Index(rest, "\n---")
	if endIdx < 0 {
		// Also check for --- at end without trailing newline
		if strings.HasSuffix(strings.TrimSpace(rest), "---") {
			endIdx = strings.LastIndex(rest, "---")
		}
		if endIdx < 0 {
			return fields
		}
	}

	fmBlock := rest[:endIdx]
	lines := strings.Split(fmBlock, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		colonIdx := strings.IndexByte(line, ':')
		if colonIdx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])
		// Remove surrounding quotes if present.
		value = strings.Trim(value, "\"'")
		fields[key] = value
	}

	return fields
}

// countPathDepth counts the number of path components in a relative path.
func countPathDepth(rel string) int {
	if rel == "." || rel == "" {
		return 0
	}
	return len(strings.Split(filepath.ToSlash(rel), "/"))
}
