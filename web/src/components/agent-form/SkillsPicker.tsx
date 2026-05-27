import { useState } from "react";
import { useSkills } from "../../hooks/useSkills";

interface SkillsPickerProps {
  value: string[];
  onAdd: (skillId: string) => void;
  onRemove: (skillId: string) => void;
}

/**
 * SkillsPicker — shows attached skills with remove, and an "Add Skill" dropdown.
 *
 * Uses useSkills() to fetch all available skills.
 * Filters out already-attached skills from the dropdown.
 */
export function SkillsPicker({ value, onAdd, onRemove }: SkillsPickerProps) {
  const { data: allSkills, isLoading } = useSkills();
  const [showDropdown, setShowDropdown] = useState(false);

  // Skills currently attached
  const attachedSkills = (allSkills ?? []).filter((s) => value.includes(s.id));

  // Skills available to add (not yet attached)
  const availableSkills = (allSkills ?? []).filter((s) => !value.includes(s.id));

  const handleSelect = (skillId: string) => {
    onAdd(skillId);
    setShowDropdown(false);
  };

  return (
    <div className="space-y-2">
      <label className="block text-sm font-medium text-gray-700">Skills</label>

      {/* Attached skills */}
      {attachedSkills.length > 0 ? (
        <div className="flex flex-wrap gap-2">
          {attachedSkills.map((skill) => (
            <span
              key={skill.id}
              className="inline-flex items-center gap-1 rounded-full bg-blue-50 border border-blue-200 px-3 py-1 text-sm text-blue-700"
            >
              {skill.name}
              <button
                type="button"
                onClick={() => onRemove(skill.id)}
                className="ml-1 rounded-full p-0.5 text-blue-400 hover:text-red-600 hover:bg-red-50 focus:outline-none focus:ring-2 focus:ring-red-500"
                aria-label={`Remove skill ${skill.name}`}
              >
                <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </span>
          ))}
        </div>
      ) : (
        <p className="text-sm text-gray-500">No skills attached.</p>
      )}

      {/* Add skill dropdown */}
      <div className="relative">
        {isLoading ? (
          <div className="h-8 w-24 bg-gray-100 rounded animate-pulse" />
        ) : availableSkills.length > 0 ? (
          <>
            <button
              type="button"
              onClick={() => setShowDropdown(!showDropdown)}
              className="inline-flex items-center gap-1 rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
              Add Skill
            </button>

            {showDropdown && (
              <div className="absolute left-0 top-full z-10 mt-1 w-64 max-h-48 overflow-y-auto rounded-md border border-gray-200 bg-white py-1 shadow-lg">
                {availableSkills.map((skill) => (
                  <button
                    key={skill.id}
                    type="button"
                    onClick={() => handleSelect(skill.id)}
                    className="w-full px-4 py-2 text-left text-sm text-gray-700 hover:bg-blue-50 focus:bg-blue-50 focus:outline-none"
                  >
                    <span className="font-medium">{skill.name}</span>
                    {skill.description && (
                      <span className="block text-xs text-gray-500 truncate">
                        {skill.description}
                      </span>
                    )}
                  </button>
                ))}
              </div>
            )}
          </>
        ) : (
          <p className="text-xs text-gray-500">
            {allSkills && allSkills.length > 0
              ? "All available skills are attached."
              : "No skills available. Create skills first."}
          </p>
        )}
      </div>
    </div>
  );
}
