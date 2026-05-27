package daemon

import (
	"fmt"
	"strings"
)

var markdownReplacer = strings.NewReplacer(
	"*", `\*`,
	"_", `\_`,
	"`", "\\`",
	"[", `\[`,
	"]", `\]`,
)

func sanitizeName(name string) string {
	return markdownReplacer.Replace(name)
}

// BuildRuntimeBrief constructs the markdown document injected into the CLI.
// Returns empty string if instructions are empty.
func BuildRuntimeBrief(agentName, instructions, workspaceContext string) string {
	if instructions == "" {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Agent Identity\n\n")
	b.WriteString(fmt.Sprintf("You are **%s**.\n\n", sanitizeName(agentName)))
	b.WriteString("## Instructions\n\n")
	b.WriteString(instructions)
	b.WriteString("\n\n")
	if workspaceContext != "" {
		b.WriteString("## Workspace Context\n\n")
		b.WriteString(workspaceContext)
		b.WriteString("\n\n")
	}
	return b.String()
}
