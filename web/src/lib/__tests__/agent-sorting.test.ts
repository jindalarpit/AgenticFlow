import { describe, it, expect } from "vitest";
import { sortAgents } from "../agent-sorting";
import type { AgentActivity, SortableAgent } from "../agent-sorting";

function makeAgent(overrides: Partial<SortableAgent> & { id: string }): SortableAgent {
  return {
    name: "Agent",
    created_at: "2025-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("sortAgents", () => {
  const agents: SortableAgent[] = [
    makeAgent({ id: "a", name: "Charlie", created_at: "2025-01-03T00:00:00Z" }),
    makeAgent({ id: "b", name: "Alice", created_at: "2025-01-01T00:00:00Z" }),
    makeAgent({ id: "c", name: "Bob", created_at: "2025-01-02T00:00:00Z" }),
  ];

  const emptyActivity = new Map<string, AgentActivity>();
  const emptyRuns = new Map<string, number>();

  it("does not mutate the input array", () => {
    const original = [...agents];
    sortAgents(agents, "name", emptyActivity, emptyRuns);
    expect(agents).toEqual(original);
  });

  describe("sort by name", () => {
    it("sorts alphabetically ascending", () => {
      const result = sortAgents(agents, "name", emptyActivity, emptyRuns);
      expect(result.map((a) => a.name)).toEqual(["Alice", "Bob", "Charlie"]);
    });
  });

  describe("sort by runs", () => {
    it("sorts by run count descending", () => {
      const runCountMap = new Map([
        ["a", 5],
        ["b", 20],
        ["c", 10],
      ]);
      const result = sortAgents(agents, "runs", emptyActivity, runCountMap);
      expect(result.map((a) => a.id)).toEqual(["b", "c", "a"]);
    });

    it("treats missing run counts as 0", () => {
      const runCountMap = new Map([["a", 3]]);
      const result = sortAgents(agents, "runs", emptyActivity, runCountMap);
      expect(result[0]!.id).toBe("a");
    });
  });

  describe("sort by created", () => {
    it("sorts by creation date newest first", () => {
      const result = sortAgents(agents, "created", emptyActivity, emptyRuns);
      expect(result.map((a) => a.id)).toEqual(["a", "c", "b"]);
    });
  });

  describe("sort by recent", () => {
    it("sorts by activity sum descending as primary", () => {
      const activityMap = new Map<string, AgentActivity>([
        ["a", { buckets: [{ completed: 1, failed: 0 }] }],
        ["b", { buckets: [{ completed: 5, failed: 2 }] }],
        ["c", { buckets: [{ completed: 3, failed: 0 }] }],
      ]);
      const result = sortAgents(agents, "recent", activityMap, emptyRuns);
      expect(result.map((a) => a.id)).toEqual(["b", "c", "a"]);
    });

    it("uses run count as tiebreaker when activity is equal", () => {
      const activityMap = new Map<string, AgentActivity>([
        ["a", { buckets: [{ completed: 5, failed: 0 }] }],
        ["b", { buckets: [{ completed: 5, failed: 0 }] }],
        ["c", { buckets: [{ completed: 5, failed: 0 }] }],
      ]);
      const runCountMap = new Map([
        ["a", 10],
        ["b", 30],
        ["c", 20],
      ]);
      const result = sortAgents(agents, "recent", activityMap, runCountMap);
      expect(result.map((a) => a.id)).toEqual(["b", "c", "a"]);
    });

    it("uses created_at as final tiebreaker (newest first)", () => {
      const activityMap = new Map<string, AgentActivity>([
        ["a", { buckets: [{ completed: 5, failed: 0 }] }],
        ["b", { buckets: [{ completed: 5, failed: 0 }] }],
        ["c", { buckets: [{ completed: 5, failed: 0 }] }],
      ]);
      const runCountMap = new Map([
        ["a", 10],
        ["b", 10],
        ["c", 10],
      ]);
      const result = sortAgents(agents, "recent", activityMap, runCountMap);
      // a: 2025-01-03, c: 2025-01-02, b: 2025-01-01
      expect(result.map((a) => a.id)).toEqual(["a", "c", "b"]);
    });

    it("treats missing activity as 0 sum", () => {
      const activityMap = new Map<string, AgentActivity>([
        ["b", { buckets: [{ completed: 1, failed: 0 }] }],
      ]);
      const result = sortAgents(agents, "recent", activityMap, emptyRuns);
      expect(result[0]!.id).toBe("b");
    });
  });

  it("returns empty array for empty input", () => {
    const result = sortAgents([], "name", emptyActivity, emptyRuns);
    expect(result).toEqual([]);
  });

  it("returns single-element array unchanged", () => {
    const single = [makeAgent({ id: "x", name: "Solo" })];
    const result = sortAgents(single, "runs", emptyActivity, emptyRuns);
    expect(result).toEqual(single);
  });
});
