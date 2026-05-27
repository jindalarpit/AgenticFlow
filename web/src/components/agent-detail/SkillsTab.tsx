import { useState, useCallback, useMemo } from "react";
import type { Agent } from "../../lib/agent-detail-types";
import { useAgentSkills, useSetAgentSkills } from "../../hooks/useAgentSkills";
import { useSkills } from "../../hooks/useSkills";
import { useToast } from "../Toast";

/* ─── Props ─── */

interface SkillsTabProps {
  agent: Agent;
}

/* ─── Component ─── */

/**
 * Skills tab in the Overview Pane.
 * Displays assigned skills with name, description, and remove button.
 * Includes an "Add Skill" button that opens a picker of available unassigned skills.
 * Sends PUT request on add/remove to update associations, reflects changes immediately.
 *
 * Validates: Requirements 10.1, 10.2, 10.3, 10.4, 10.5, 10.6
 */
export function SkillsTab({ agent }: SkillsTabProps) {
  const { showToast } = useToast();
  const [pickerOpen, setPickerOpen] = useState(false);

  // Fetch assigned skills for this agent
  const {
    data: assignedSkills = [],
    isLoading: loadingAssigned,
  } = useAgentSkills(agent.id);

  // Fetch all available skills for the picker
  const { data: allSkills = [], isLoading: loadingAll } = useSkills();

  // Mutation to update agent-skill associations
  const setAgentSkills = useSetAgentSkills(agent.id);

  // Compute unassigned skills (available for adding)
  const unassignedSkills = useMemo(() => {
    const assignedIds = new Set(assignedSkills.map((s) => s.id));
    return allSkills.filter((s) => !assignedIds.has(s.id));
  }, [allSkills, assignedSkills]);

  // Add a skill to the agent
  const handleAddSkill = useCallback(
    (skillId: string) => {
      const currentIds = assignedSkills.map((s) => s.id);
      const newIds = [...currentIds, skillId];
      setAgentSkills.mutate(
        { skill_ids: newIds },
        {
          onSuccess: () => {
            setPickerOpen(false);
          },
          onError: () => {
            showToast("Failed to add skill", "error");
          },
        }
      );
    },
    [assignedSkills, setAgentSkills, showToast]
  );

  // Remove a skill from the agent
  const handleRemoveSkill = useCallback(
    (skillId: string) => {
      const newIds = assignedSkills
        .map((s) => s.id)
        .filter((id) => id !== skillId);
      setAgentSkills.mutate(
        { skill_ids: newIds },
        {
          onError: () => {
            showToast("Failed to remove skill", "error");
          },
        }
      );
    },
    [assignedSkills, setAgentSkills, showToast]
  );

  if (loadingAssigned) {
    return (
      <div className="flex items-center justify-center py-12">
        <p className="text-sm text-muted-foreground">Loading skills…</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      {/* Header with Add Skill button */}
      <div className="flex items-center justify-between">
        <h3 className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
          Assigned Skills
        </h3>
        <div className="relative">
          <button
            type="button"
            onClick={() => setPickerOpen(!pickerOpen)}
            className="rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-blue-700"
          >
            Add Skill
          </button>

          {/* Skill picker dropdown */}
          {pickerOpen && (
            <div className="absolute right-0 top-full z-10 mt-1 w-72 rounded-md border border-gray-200 bg-white shadow-lg">
              <div className="p-2">
                {loadingAll ? (
                  <p className="px-2 py-3 text-center text-xs text-gray-500">
                    Loading skills…
                  </p>
                ) : unassignedSkills.length === 0 ? (
                  <p className="px-2 py-3 text-center text-xs text-gray-500">
                    No available skills to add
                  </p>
                ) : (
                  <ul className="max-h-60 overflow-y-auto" role="listbox">
                    {unassignedSkills.map((skill) => (
                      <li key={skill.id}>
                        <button
                          type="button"
                          onClick={() => handleAddSkill(skill.id)}
                          disabled={setAgentSkills.isPending}
                          className="flex w-full flex-col gap-0.5 rounded-md px-2 py-2 text-left transition-colors hover:bg-gray-100 disabled:opacity-50"
                          role="option"
                          aria-selected={false}
                        >
                          <span className="text-sm font-medium text-gray-900">
                            {skill.name}
                          </span>
                          {skill.description && (
                            <span className="text-xs text-gray-500 line-clamp-2">
                              {skill.description}
                            </span>
                          )}
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Assigned skills list or empty state */}
      {assignedSkills.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 text-center">
          <p className="text-sm text-muted-foreground">
            No skills assigned. Add skills to give this agent domain knowledge.
          </p>
        </div>
      ) : (
        <ul className="flex flex-col gap-2" role="list">
          {assignedSkills.map((skill) => (
            <li
              key={skill.id}
              className="flex items-start justify-between gap-3 rounded-md border border-gray-200 px-4 py-3"
            >
              <div className="flex flex-col gap-0.5 min-w-0">
                <span className="text-sm font-medium text-gray-900">
                  {skill.name}
                </span>
                {skill.description && (
                  <span className="text-xs text-gray-500 line-clamp-2">
                    {skill.description}
                  </span>
                )}
              </div>
              <button
                type="button"
                onClick={() => handleRemoveSkill(skill.id)}
                disabled={setAgentSkills.isPending}
                className="flex-shrink-0 rounded-md px-2 py-1 text-xs font-medium text-red-600 transition-colors hover:bg-red-50 hover:text-red-700 disabled:opacity-50"
                aria-label={`Remove skill ${skill.name}`}
              >
                Remove
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
