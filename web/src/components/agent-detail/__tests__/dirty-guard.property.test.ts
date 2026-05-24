// Feature: agent-detail-ui, Property 1: Dirty-guard state machine correctness
// **Validates: Requirements 7.5, 7.6, 7.7, 12.5, 14.9, 15.7**

import { describe, it, expect } from "vitest";
import fc from "fast-check";
import type { TabId } from "../../../lib/agent-detail-types";

/* ─── Pure state machine (mirrors OverviewPane.tsx logic) ─── */

const ALL_TABS: TabId[] = ["activity", "tasks", "instructions", "skills", "env", "custom_args"];

interface TabState {
  activeTab: TabId;
  activeDirty: boolean;
  pendingTab: TabId | null;
}

type Action =
  | { type: "switch"; tab: TabId }
  | { type: "setDirty"; dirty: boolean }
  | { type: "confirm" }
  | { type: "cancel" };

/**
 * Pure reducer implementing the dirty-guard state machine.
 * Extracted from OverviewPane's requestTabChange / handleDiscard / handleStay logic.
 */
function dirtyGuardReducer(state: TabState, action: Action): TabState {
  switch (action.type) {
    case "switch": {
      // requestTabChange logic
      if (action.tab === state.activeTab) return state;
      if (state.activeDirty) {
        return { ...state, pendingTab: action.tab };
      }
      return { ...state, activeTab: action.tab, pendingTab: null };
    }
    case "setDirty": {
      return { ...state, activeDirty: action.dirty };
    }
    case "confirm": {
      // handleDiscard logic
      if (state.pendingTab === null) return state;
      return {
        activeTab: state.pendingTab,
        activeDirty: false,
        pendingTab: null,
      };
    }
    case "cancel": {
      // handleStay logic
      return { ...state, pendingTab: null };
    }
  }
}

/* ─── Generators ─── */

const tabArb = fc.constantFrom(...ALL_TABS);

const actionArb: fc.Arbitrary<Action> = fc.oneof(
  fc.record({ type: fc.constant("switch" as const), tab: tabArb }),
  fc.record({ type: fc.constant("setDirty" as const), dirty: fc.boolean() }),
  fc.record({ type: fc.constant("confirm" as const) }),
  fc.record({ type: fc.constant("cancel" as const) })
);

const actionsArb = fc.array(actionArb, { minLength: 1, maxLength: 30 });

const initialState: TabState = {
  activeTab: "activity",
  activeDirty: false,
  pendingTab: null,
};

/* ─── Property Tests ─── */

describe("Dirty-guard state machine — Property 1: Dirty-guard state machine correctness", () => {
  it("dialog appears (pendingTab !== null) iff activeDirty === true AND user requests a different tab", () => {
    fc.assert(
      fc.property(
        fc.record({
          activeTab: tabArb,
          activeDirty: fc.boolean(),
          pendingTab: fc.constant(null as TabId | null),
        }),
        tabArb,
        (state, nextTab) => {
          const result = dirtyGuardReducer(state, { type: "switch", tab: nextTab });

          if (nextTab === state.activeTab) {
            // Same tab → no-op, no dialog
            expect(result.pendingTab).toBeNull();
          } else if (state.activeDirty) {
            // Different tab + dirty → dialog opens (pendingTab set)
            expect(result.pendingTab).toBe(nextTab);
          } else {
            // Different tab + not dirty → immediate switch, no dialog
            expect(result.pendingTab).toBeNull();
            expect(result.activeTab).toBe(nextTab);
          }
        }
      ),
      { numRuns: 100 }
    );
  });

  it("after confirm (discard): activeTab === requested tab, activeDirty === false, pendingTab === null", () => {
    fc.assert(
      fc.property(
        tabArb,
        tabArb.filter((t) => t !== "activity"),
        (activeTab, pendingTab) => {
          // Set up state where dialog is open (pendingTab is set)
          fc.pre(pendingTab !== activeTab);
          const state: TabState = {
            activeTab,
            activeDirty: true,
            pendingTab,
          };

          const result = dirtyGuardReducer(state, { type: "confirm" });

          expect(result.activeTab).toBe(pendingTab);
          expect(result.activeDirty).toBe(false);
          expect(result.pendingTab).toBeNull();
        }
      ),
      { numRuns: 100 }
    );
  });

  it("after cancel (stay): activeTab unchanged, activeDirty unchanged (still true), pendingTab === null", () => {
    fc.assert(
      fc.property(
        tabArb,
        tabArb,
        (activeTab, pendingTab) => {
          fc.pre(pendingTab !== activeTab);
          const state: TabState = {
            activeTab,
            activeDirty: true,
            pendingTab,
          };

          const result = dirtyGuardReducer(state, { type: "cancel" });

          expect(result.activeTab).toBe(activeTab);
          expect(result.activeDirty).toBe(true);
          expect(result.pendingTab).toBeNull();
        }
      ),
      { numRuns: 100 }
    );
  });

  it("requesting the same tab as active → no state change", () => {
    fc.assert(
      fc.property(
        fc.record({
          activeTab: tabArb,
          activeDirty: fc.boolean(),
          pendingTab: fc.oneof(fc.constant(null as TabId | null), tabArb),
        }),
        (state) => {
          const result = dirtyGuardReducer(state, { type: "switch", tab: state.activeTab });
          expect(result).toEqual(state);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("when not dirty, tab switch happens immediately (activeTab changes, no pendingTab)", () => {
    fc.assert(
      fc.property(
        tabArb,
        tabArb,
        (activeTab, nextTab) => {
          fc.pre(nextTab !== activeTab);
          const state: TabState = {
            activeTab,
            activeDirty: false,
            pendingTab: null,
          };

          const result = dirtyGuardReducer(state, { type: "switch", tab: nextTab });

          expect(result.activeTab).toBe(nextTab);
          expect(result.pendingTab).toBeNull();
        }
      ),
      { numRuns: 100 }
    );
  });

  it("for any sequence of actions, state invariants hold: pendingTab is only set when dirty + different tab requested", () => {
    fc.assert(
      fc.property(actionsArb, (actions) => {
        let state = { ...initialState };

        for (const action of actions) {
          const prevState = { ...state };
          state = dirtyGuardReducer(state, action);

          // Invariant: pendingTab can only become non-null via a "switch" action
          // when activeDirty was true and the requested tab was different
          if (action.type === "switch") {
            if (action.tab === prevState.activeTab) {
              // Same tab → state unchanged
              expect(state).toEqual(prevState);
            } else if (prevState.activeDirty) {
              // Dirty + different tab → pendingTab set
              expect(state.pendingTab).toBe(action.tab);
              expect(state.activeTab).toBe(prevState.activeTab);
            } else {
              // Not dirty + different tab → immediate switch
              expect(state.activeTab).toBe(action.tab);
              expect(state.pendingTab).toBeNull();
            }
          }

          if (action.type === "confirm" && prevState.pendingTab !== null) {
            expect(state.activeTab).toBe(prevState.pendingTab);
            expect(state.activeDirty).toBe(false);
            expect(state.pendingTab).toBeNull();
          }

          if (action.type === "cancel") {
            expect(state.pendingTab).toBeNull();
            expect(state.activeTab).toBe(prevState.activeTab);
            expect(state.activeDirty).toBe(prevState.activeDirty);
          }
        }
      }),
      { numRuns: 100 }
    );
  });
});
