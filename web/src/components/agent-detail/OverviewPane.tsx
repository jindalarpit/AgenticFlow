import { useState, useCallback } from "react";
import type { TabId, Agent } from "../../lib/agent-detail-types";
import { TabBar } from "./TabBar";
import { DirtyGuard } from "./DirtyGuard";
import { ActivityTab } from "./ActivityTab";
import { TasksTab } from "./TasksTab";
import { InstructionsTab } from "./InstructionsTab";
import { SkillsTab } from "./SkillsTab";
import { ToolsTab } from "./ToolsTab";
import { EnvironmentTab } from "./EnvironmentTab";
import { CustomArgsTab } from "./CustomArgsTab";

/* ─── Props ─── */

interface OverviewPaneProps {
  agent: Agent;
  isOwner: boolean;
  onSave: (data: Partial<Agent>) => Promise<void>;
}

/* ─── Component ─── */

/**
 * Right-side tabbed content area for the agent detail page.
 * Manages tab navigation state and dirty-guard logic for editable tabs.
 *
 * Editable tabs (Instructions, Environment, Custom Args) report dirty state
 * upward. When the user tries to switch away from a dirty tab, a confirmation
 * dialog intercepts the switch.
 */
export function OverviewPane({ agent, isOwner, onSave }: OverviewPaneProps) {
  const [activeTab, setActiveTab] = useState<TabId>("activity");
  const [activeDirty, setActiveDirty] = useState(false);
  const [pendingTab, setPendingTab] = useState<TabId | null>(null);

  /**
   * Intercepts tab switches when the active tab has unsaved changes.
   * If dirty, opens the DirtyGuard dialog instead of switching immediately.
   */
  const requestTabChange = useCallback(
    (next: TabId) => {
      if (next === activeTab) return;
      if (activeDirty) {
        setPendingTab(next);
        return;
      }
      setActiveTab(next);
    },
    [activeTab, activeDirty]
  );

  /**
   * Called when the user confirms discarding unsaved changes.
   * Resets dirty state and switches to the pending tab.
   */
  const handleDiscard = useCallback(() => {
    setActiveDirty(false);
    if (pendingTab) {
      setActiveTab(pendingTab);
      setPendingTab(null);
    }
  }, [pendingTab]);

  /**
   * Called when the user cancels the dirty-guard dialog.
   * Clears the pending tab and stays on the current tab.
   */
  const handleStay = useCallback(() => {
    setPendingTab(null);
  }, []);

  return (
    <div className="flex flex-col h-full overflow-hidden">
      <TabBar activeTab={activeTab} onTabChange={requestTabChange} />

      {/* DirtyGuard confirmation dialog */}
      <DirtyGuard
        isOpen={pendingTab !== null}
        onDiscard={handleDiscard}
        onStay={handleStay}
      />

      {/* Tab content area — unmounts previous tab on switch */}
      <div
        className="flex-1 overflow-y-auto p-4"
        role="tabpanel"
        id={`tabpanel-${activeTab}`}
        aria-labelledby={`tab-${activeTab}`}
      >
        {activeTab === "activity" && <ActivityTab agent={agent} />}
        {activeTab === "tasks" && <TasksTab agent={agent} />}
        {activeTab === "instructions" && (
          <InstructionsTab
            agent={agent}
            isOwner={isOwner}
            onDirtyChange={setActiveDirty}
            onSave={onSave}
          />
        )}
        {activeTab === "skills" && <SkillsTab agent={agent} />}
        {activeTab === "tools" && (
          <ToolsTab
            agent={agent}
            isOwner={isOwner}
            onDirtyChange={setActiveDirty}
            onSave={onSave}
          />
        )}
        {activeTab === "env" && (
          <EnvironmentTab
            agent={agent}
            isOwner={isOwner}
            onDirtyChange={setActiveDirty}
            onSave={onSave}
          />
        )}
        {activeTab === "custom_args" && (
          <CustomArgsTab
            agent={agent}
            isOwner={isOwner}
            onDirtyChange={setActiveDirty}
            onSave={onSave}
          />
        )}
      </div>
    </div>
  );
}
