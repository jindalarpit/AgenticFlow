/**
 * DeliverableSelector — Checkbox group for selecting task workflow deliverables.
 *
 * Displays checkboxes for: plan, design, tasks, execution (in canonical order).
 * Defaults to ["execution"] when nothing is selected.
 *
 * Validates: Requirements 1.4, 1.5
 */

/** Valid deliverable values in canonical execution order. */
export const DELIVERABLE_OPTIONS = [
  { value: "plan", label: "Plan" },
  { value: "design", label: "Design" },
  { value: "tasks", label: "Tasks" },
  { value: "execution", label: "Execution" },
] as const;

export type Deliverable = (typeof DELIVERABLE_OPTIONS)[number]["value"];

export const DEFAULT_DELIVERABLES: Deliverable[] = ["execution"];

export interface DeliverableSelectorProps {
  /** Currently selected deliverables. */
  value: Deliverable[];
  /** Called when the selection changes. Receives the new array of selected deliverables. */
  onChange: (deliverables: Deliverable[]) => void;
}

export function DeliverableSelector({ value, onChange }: DeliverableSelectorProps) {
  // If value is empty, treat as default (execution checked)
  const selected = value.length > 0 ? value : DEFAULT_DELIVERABLES;

  function handleToggle(deliverable: Deliverable) {
    const isSelected = selected.includes(deliverable);

    let next: Deliverable[];
    if (isSelected) {
      // Remove it — but if this would leave nothing selected, default to execution
      next = selected.filter((d) => d !== deliverable);
      if (next.length === 0) {
        next = DEFAULT_DELIVERABLES;
      }
    } else {
      // Add it, maintaining canonical order
      next = DELIVERABLE_OPTIONS
        .map((opt) => opt.value)
        .filter((d) => selected.includes(d) || d === deliverable);
    }

    onChange(next);
  }

  return (
    <fieldset>
      <legend className="block text-sm font-medium text-gray-700 mb-2">
        Deliverables
      </legend>
      <div className="flex flex-wrap gap-4">
        {DELIVERABLE_OPTIONS.map((option) => (
          <label
            key={option.value}
            className="inline-flex items-center gap-2 cursor-pointer select-none"
          >
            <input
              type="checkbox"
              checked={selected.includes(option.value)}
              onChange={() => handleToggle(option.value)}
              className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-2 focus:ring-blue-200"
              aria-label={`Select ${option.label} deliverable`}
            />
            <span className="text-sm text-gray-700">{option.label}</span>
          </label>
        ))}
      </div>
      <p className="mt-1.5 text-xs text-gray-500">
        Select which outputs the workflow should produce. Stages execute in order: Plan → Design → Tasks → Execution.
      </p>
    </fieldset>
  );
}
