import type { Agent } from "../../lib/agent-detail-types";
import { IdentitySection } from "./IdentitySection";
import { PropertiesSection } from "./PropertiesSection";
import { DetailsSection } from "./DetailsSection";
import { SkillsSection } from "./SkillsSection";

/* ─── Props ─── */

interface SidebarInspectorProps {
  agent: Agent;
  isOwner: boolean;
  onUpdate: (data: Partial<Agent>) => Promise<void>;
}

/* ─── Component ─── */

/**
 * SidebarInspector is the fixed-width left column of the agent detail page.
 *
 * Composes four sections:
 * 1. IdentitySection — avatar, name, description, status
 * 2. PropertiesSection — runtime, model, visibility, concurrency
 * 3. DetailsSection — owner, created, updated
 * 4. SkillsSection — skills count + badges
 *
 * On desktop (≥768px): 320px wide with a right border.
 * On mobile (<768px): full width, no border.
 */
export function SidebarInspector({ agent, isOwner, onUpdate }: SidebarInspectorProps) {
  return (
    <aside className="w-full md:w-[320px] md:border-r border-gray-200 overflow-y-auto">
      <IdentitySection agent={agent} isOwner={isOwner} onUpdate={onUpdate} />
      <PropertiesSection agent={agent} isOwner={isOwner} onUpdate={onUpdate} />
      <DetailsSection agent={agent} />
      <SkillsSection skills={agent.skills ?? []} />
    </aside>
  );
}
