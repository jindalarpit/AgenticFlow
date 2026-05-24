import type { Agent } from "../../lib/agent-detail-types";

interface SkillsTabProps {
  agent: Agent;
}

/**
 * Skills tab in the Overview Pane.
 * Displays a read-only list of skills attached to the agent.
 * Shows a placeholder message when no skills are configured.
 *
 * Validates: Requirements 13.1, 13.2, 13.3
 */
export function SkillsTab({ agent }: SkillsTabProps) {
  const { skills } = agent;

  if (!skills || skills.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <p className="text-sm text-muted-foreground">No skills configured</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-2">
      <h3 className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
        Attached Skills
      </h3>
      <ul className="flex flex-col gap-1" role="list">
        {skills.map((skill) => (
          <li
            key={skill.id}
            className="flex items-center gap-2 rounded-md border px-3 py-2 text-sm"
          >
            <span className="h-2 w-2 rounded-full bg-emerald-500" aria-hidden="true" />
            <span>{skill.name}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
