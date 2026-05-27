package daemon

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/agenticflow/agenticflow/daemon/pkg/mcp"
	"github.com/agenticflow/agenticflow/daemon/pkg/skill"
	"github.com/agenticflow/agenticflow/shared/api"
)

// injectionResult holds the results of skill/MCP injection so the caller
// can perform cleanup after execution completes.
type injectionResult struct {
	// mcpConfigPath is the temp file path for MCP config (empty if not injected).
	mcpConfigPath string
}

// injectSkillsAndMCP performs pre-execution injection of skills and MCP config.
// It writes skill files to the provider-native directory (with frontmatter ensured)
// and writes MCP config to a temp file, returning additional CLI args to pass.
//
// The caller MUST call cleanupInjection after execution completes (success or failure).
func injectSkillsAndMCP(
	workspaceDir string,
	agentType string,
	agentData *TaskAgentData,
	logger *slog.Logger,
) (*injectionResult, []string, error) {
	result := &injectionResult{}
	var extraArgs []string

	if agentData == nil {
		return result, extraArgs, nil
	}

	// Inject skills if present.
	if len(agentData.Skills) > 0 {
		// Ensure frontmatter on each skill's content before injection.
		processedSkills := make([]api.TaskSkill, len(agentData.Skills))
		for i, s := range agentData.Skills {
			processedSkills[i] = api.TaskSkill{
				Name:        s.Name,
				Description: s.Description,
				Content:     skill.EnsureFrontmatter(s.Content, s.Name, s.Description),
				Files:       s.Files,
			}
		}

		skillInjector := &skill.Injector{Logger: logger}
		if err := skillInjector.InjectSkills(workspaceDir, agentType, processedSkills); err != nil {
			return result, extraArgs, err
		}

		logger.Info("injected skills", "count", len(processedSkills), "provider", agentType)
	}

	// Inject MCP config if present.
	if len(agentData.MCPConfig) > 0 && !isEmptyJSON(agentData.MCPConfig) {
		mcpInjector := &mcp.Injector{Logger: logger}
		mcpPath, err := mcpInjector.InjectMCPConfig(agentData.MCPConfig)
		if err != nil {
			return result, extraArgs, err
		}
		result.mcpConfigPath = mcpPath
		extraArgs = append(extraArgs, mcp.MCPArgs(mcpPath)...)
		logger.Info("injected MCP config", "path", mcpPath)
	}

	return result, extraArgs, nil
}

// cleanupInjection removes any temporary files created during injection.
// It is safe to call with a nil result or empty paths.
func cleanupInjection(result *injectionResult, logger *slog.Logger) {
	if result == nil || result.mcpConfigPath == "" {
		return
	}

	mcpInjector := &mcp.Injector{Logger: logger}
	if err := mcpInjector.CleanupMCPConfig(result.mcpConfigPath); err != nil {
		logger.Warn("failed to cleanup MCP config", "path", result.mcpConfigPath, "error", err)
	}
}

// isEmptyJSON checks if a JSON raw message is empty, null, or an empty object.
func isEmptyJSON(data json.RawMessage) bool {
	if len(data) == 0 {
		return true
	}
	trimmed := string(data)
	return trimmed == "null" || trimmed == "{}" || trimmed == "[]"
}

// reportLocalSkills discovers local skills and reports them to the server
// for each registered runtime.
func (d *Daemon) reportLocalSkills(ctx context.Context) {
	if d.client == nil {
		return
	}

	d.mu.RLock()
	runtimes := make(map[string]string, len(d.runtimes))
	for id, provider := range d.runtimes {
		runtimes[id] = provider
	}
	d.mu.RUnlock()

	if len(runtimes) == 0 {
		return
	}

	discovered, err := skill.DiscoverLocalSkills(d.logger)
	if err != nil {
		d.logger.Warn("local skill discovery failed", "error", err)
		return
	}

	if len(discovered) == 0 {
		d.logger.Debug("no local skills discovered")
		return
	}

	// Group discovered skills by provider and report to matching runtimes.
	for runtimeID, provider := range runtimes {
		var skills []LocalSkillReport
		for _, ds := range discovered {
			if ds.Provider == provider || ds.Provider == "" {
				skills = append(skills, LocalSkillReport{
					Name:        ds.Name,
					Description: ds.Description,
					SourcePath:  ds.SourcePath,
					Provider:    ds.Provider,
				})
			}
		}

		if len(skills) == 0 {
			continue
		}

		if err := d.client.ReportLocalSkills(ctx, runtimeID, skills); err != nil {
			d.logger.Warn("failed to report local skills",
				"runtime_id", runtimeID,
				"provider", provider,
				"count", len(skills),
				"error", err,
			)
		} else {
			d.logger.Info("reported local skills",
				"runtime_id", runtimeID,
				"provider", provider,
				"count", len(skills),
			)
		}
	}
}
