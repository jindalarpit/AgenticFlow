// Feature: task-result-display, Property 2: Message sequence ordering
// **Validates: Requirements 2.4, 3.6**

import { describe, it, expect } from "vitest";
import fc from "fast-check";
import type { TaskMessage } from "../../hooks/useTasks";

/**
 * Sort-and-deduplicate logic matching useTaskStream:
 * 1. Accumulate messages into an array
 * 2. Sort ascending by sequence number
 * 3. Remove duplicates (keep first occurrence per sequence)
 */
function sortAndDeduplicate(messages: TaskMessage[]): TaskMessage[] {
  const seen = new Set<number>();
  const result: TaskMessage[] = [];

  // Sort ascending by sequence
  const sorted = [...messages].sort((a, b) => a.sequence - b.sequence);

  // Deduplicate: keep first occurrence of each sequence
  for (const msg of sorted) {
    if (!seen.has(msg.sequence)) {
      seen.add(msg.sequence);
      result.push(msg);
    }
  }

  return result;
}

/**
 * Arbitrary for generating TaskMessage objects with controlled sequence numbers.
 */
const taskMessageArb = (sequence?: number): fc.Arbitrary<TaskMessage> =>
  fc.record({
    id: fc.uuid(),
    task_id: fc.uuid(),
    sequence: sequence !== undefined ? fc.constant(sequence) : fc.integer({ min: 0, max: 10000 }),
    stream: fc.constantFrom("stdout", "stderr") as fc.Arbitrary<"stdout" | "stderr">,
    content: fc.string({ minLength: 0, maxLength: 200 }),
    created_at: fc.integer({ min: 1577836800000, max: 1893456000000 }).map((ts) => new Date(ts).toISOString()),
  });

/**
 * Arbitrary for generating arrays of TaskMessage objects with potentially
 * duplicate sequence numbers (to test deduplication).
 */
const taskMessagesArb: fc.Arbitrary<TaskMessage[]> = fc
  .array(taskMessageArb(), { minLength: 0, maxLength: 50 })
  .chain((messages) => {
    // Optionally inject duplicates by repeating some messages with same sequence
    return fc
      .array(fc.integer({ min: 0, max: Math.max(messages.length - 1, 0) }), {
        minLength: 0,
        maxLength: 10,
      })
      .map((indices) => {
        const duplicates = indices
          .filter((i) => i < messages.length)
          .map((i) => ({
            ...messages[i],
            id: `dup-${messages[i].id}`,
            content: `duplicate-${messages[i].content}`,
          }));
        return [...messages, ...duplicates];
      });
  });

describe("Message sequence ordering — Property 2: Message sequence ordering", () => {
  it("result is always sorted ascending by sequence number", () => {
    fc.assert(
      fc.property(taskMessagesArb, (messages) => {
        const result = sortAndDeduplicate(messages);

        for (let i = 1; i < result.length; i++) {
          expect(result[i].sequence).toBeGreaterThan(result[i - 1].sequence);
        }
      }),
      { numRuns: 100 }
    );
  });

  it("result contains no duplicate sequence numbers", () => {
    fc.assert(
      fc.property(taskMessagesArb, (messages) => {
        const result = sortAndDeduplicate(messages);
        const sequences = result.map((m) => m.sequence);
        const uniqueSequences = new Set(sequences);

        expect(sequences.length).toBe(uniqueSequences.size);
      }),
      { numRuns: 100 }
    );
  });

  it("all unique sequences from input are preserved in output", () => {
    fc.assert(
      fc.property(taskMessagesArb, (messages) => {
        const result = sortAndDeduplicate(messages);
        const inputSequences = new Set(messages.map((m) => m.sequence));
        const outputSequences = new Set(result.map((m) => m.sequence));

        expect(outputSequences).toEqual(inputSequences);
      }),
      { numRuns: 100 }
    );
  });

  it("incremental appending produces same result as batch sort-and-deduplicate", () => {
    fc.assert(
      fc.property(taskMessagesArb, (messages) => {
        // Simulate incremental appending (as useTaskStream does)
        let accumulated: TaskMessage[] = [];
        const seenSequences = new Set<number>();

        for (const msg of messages) {
          if (seenSequences.has(msg.sequence)) continue;
          seenSequences.add(msg.sequence);
          accumulated = [...accumulated, msg];
          accumulated.sort((a, b) => a.sequence - b.sequence);
        }

        // Batch approach
        const batchResult = sortAndDeduplicate(messages);

        // Both should have the same sequences in the same order
        expect(accumulated.map((m) => m.sequence)).toEqual(
          batchResult.map((m) => m.sequence)
        );
      }),
      { numRuns: 100 }
    );
  });
});
