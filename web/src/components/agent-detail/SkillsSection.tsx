import type { AgentSkill } from "../../lib/agent-detail-types";

interface SkillsSectionProps {
  skills: AgentSkill[];
}

/**
 * Skills section in the sidebar inspector.
 * Displays a count of attached skills and a badge/chip per skill name.
 * Shows "0" when no skills are attached.
 */
export function SkillsSection({ skills }: SkillsSectionProps) {
  return (
    <div className="flex flex-col border-b px-5 py-4">
      <div className="mb-2 flex items-center gap-2">
        <span className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
          Skills
        </span>
        <span className="font-mono text-[10px] tabular-nums text-muted-foreground/70">
          {skills.length}
        </span>
      </div>
      <div className="flex flex-wrap gap-1">
        {skills.map((skill) => (
          <span
            key={skill.id}
            className="rounded-md bg-muted px-1.5 py-0.5 font-mono text-[10px] font-medium text-muted-foreground"
          >
            {skill.name}
          </span>
        ))}
      </div>
    </div>
  );
}
