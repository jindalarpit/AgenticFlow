import type { TabId } from "../../lib/agent-detail-types";

/* ─── Tab Configuration ─── */

const TABS: { id: TabId; label: string }[] = [
  { id: "activity", label: "Activity" },
  { id: "tasks", label: "Tasks" },
  { id: "instructions", label: "Instructions" },
  { id: "skills", label: "Skills" },
  { id: "tools", label: "Tools" },
  { id: "env", label: "Environment" },
  { id: "custom_args", label: "Custom Args" },
];

/* ─── Props ─── */

interface TabBarProps {
  activeTab: TabId;
  onTabChange: (tab: TabId) => void;
}

/* ─── Component ─── */

/**
 * Horizontal tab bar with 6 tabs for the agent detail Overview Pane.
 * Active tab has a bottom border highlight; inactive tabs are muted.
 */
export function TabBar({ activeTab, onTabChange }: TabBarProps) {
  return (
    <div
      className="flex border-b border-gray-200 overflow-x-auto"
      role="tablist"
      aria-label="Agent detail tabs"
    >
      {TABS.map((tab) => {
        const isActive = tab.id === activeTab;
        return (
          <button
            key={tab.id}
            type="button"
            role="tab"
            aria-selected={isActive}
            aria-controls={`tabpanel-${tab.id}`}
            id={`tab-${tab.id}`}
            onClick={() => onTabChange(tab.id)}
            className={`whitespace-nowrap px-4 py-2.5 text-sm font-medium transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 ${
              isActive
                ? "border-b-2 border-blue-600 text-gray-900"
                : "text-gray-500 hover:text-gray-700"
            }`}
          >
            {tab.label}
          </button>
        );
      })}
    </div>
  );
}
