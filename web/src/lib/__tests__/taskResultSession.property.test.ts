// Feature: task-result-display, Property 4: Session state persistence round-trip
// **Validates: Requirements 4.4, 4.5**

import { describe, it, expect, beforeEach } from "vitest";
import fc from "fast-check";
import {
  saveTaskResultPanelState,
  loadTaskResultPanelState,
  type TaskResultPanelSessionData,
} from "../taskResultSession";

/**
 * Property 4: Session state persistence round-trip
 *
 * For any valid TaskResultPanelSessionData object, serializing it to
 * sessionStorage via saveTaskResultPanelState and then deserializing it
 * via loadTaskResultPanelState SHALL produce an object deeply equal to
 * the original.
 */
describe("Property 4: Session state persistence round-trip", () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it("save then load produces a deeply equal object for any valid TaskResultPanelSessionData", () => {
    const arbSessionData: fc.Arbitrary<TaskResultPanelSessionData> = fc.record({
      taskId: fc.string({ minLength: 1 }).filter((s) => s.trim().length > 0),
      dismissed: fc.boolean(),
    });

    fc.assert(
      fc.property(arbSessionData, (state) => {
        saveTaskResultPanelState(state);
        const loaded = loadTaskResultPanelState();
        expect(loaded).toEqual(state);
      }),
      { numRuns: 100 }
    );
  });
});
