// Feature: task-tool-chain-ui, Property 1: Parser type classification
// **Validates: Requirements 1.1, 1.2, 1.3, 1.4, 1.5, 1.7**

import { describe, it, expect } from "vitest";
import fc from "fast-check";
import { parseMessageContent, parseMessages } from "../tool-chain-parser";
import type { TaskMessage } from "../../hooks/useTasks";

/**
 * Property 1: Parser type classification
 *
 * For any raw message with `stream` set to "stderr", the parser SHALL classify
 * it as an `error`-type TimelineItem; and for any raw message containing a valid
 * JSON object with a "type" field matching "tool_use", "tool_result", or "thinking",
 * the parser SHALL classify it as that corresponding type; and for any raw message
 * content that does not match any known pattern, the parser SHALL classify it as
 * a `text`-type TimelineItem.
 */
describe("Property 1: Parser type classification", () => {
  // ─── Sub-property: stderr plain text → error type ────────────────────────

  it("any plain text on stderr is classified as error type", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1 }).filter((s) => {
          // Exclude strings that start with '{' (could be parsed as JSON)
          const trimmed = s.trim();
          return trimmed.length > 0 && !trimmed.startsWith("{");
        }),
        (content) => {
          const items = parseMessageContent(content, "stderr", 1);
          expect(items.length).toBeGreaterThanOrEqual(1);
          for (const item of items) {
            expect(item.type).toBe("error");
          }
        }
      ),
      { numRuns: 100 }
    );
  });

  // ─── Sub-property: JSON with "type": "tool_use" → tool_use type ──────────

  it('JSON with "type": "tool_use" is classified as tool_use', () => {
    fc.assert(
      fc.property(
        fc.record({
          name: fc.string({ minLength: 1, maxLength: 50 }).filter((s) => s.trim().length > 0),
          input: fc.dictionary(
            fc.string({ minLength: 1, maxLength: 20 }).filter((s) => s.trim().length > 0),
            fc.oneof(fc.string(), fc.integer(), fc.boolean())
          ),
        }),
        ({ name, input }) => {
          const jsonObj = { type: "tool_use", name, input };
          const content = JSON.stringify(jsonObj);
          const items = parseMessageContent(content, "stdout", 1);
          expect(items).toHaveLength(1);
          expect(items[0]!.type).toBe("tool_use");
          expect(items[0]!.tool).toBe(name);
        }
      ),
      { numRuns: 100 }
    );
  });

  // ─── Sub-property: JSON with "type": "tool_result" → tool_result type ────

  it('JSON with "type": "tool_result" is classified as tool_result', () => {
    fc.assert(
      fc.property(
        fc.record({
          name: fc.string({ minLength: 1, maxLength: 50 }).filter((s) => s.trim().length > 0),
          output: fc.string({ minLength: 0, maxLength: 200 }),
        }),
        ({ name, output }) => {
          const jsonObj = { type: "tool_result", name, output };
          const content = JSON.stringify(jsonObj);
          const items = parseMessageContent(content, "stdout", 1);
          expect(items).toHaveLength(1);
          expect(items[0]!.type).toBe("tool_result");
        }
      ),
      { numRuns: 100 }
    );
  });

  // ─── Sub-property: JSON with "type": "thinking" → thinking type ──────────

  it('JSON with "type": "thinking" is classified as thinking', () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1, maxLength: 500 }),
        (thinkingContent) => {
          const jsonObj = { type: "thinking", content: thinkingContent };
          const content = JSON.stringify(jsonObj);
          const items = parseMessageContent(content, "stdout", 1);
          expect(items).toHaveLength(1);
          expect(items[0]!.type).toBe("thinking");
        }
      ),
      { numRuns: 100 }
    );
  });

  // ─── Sub-property: Non-JSON text on stdout → text type ───────────────────

  it("plain text on stdout that is not JSON is classified as text type", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1 }).filter((s) => {
          const trimmed = s.trim();
          // Exclude strings that could be parsed as JSON objects
          return trimmed.length > 0 && !trimmed.startsWith("{");
        }),
        (content) => {
          const items = parseMessageContent(content, "stdout", 1);
          expect(items.length).toBeGreaterThanOrEqual(1);
          for (const item of items) {
            expect(item.type).toBe("text");
          }
        }
      ),
      { numRuns: 100 }
    );
  });

  // ─── Sub-property: JSON with unknown type field → text type ──────────────

  it("JSON with an unknown type field is classified as text type", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1, maxLength: 50 }).filter((s) => {
          const trimmed = s.trim();
          // Exclude the known type values
          return (
            trimmed.length > 0 &&
            trimmed !== "tool_use" &&
            trimmed !== "tool_result" &&
            trimmed !== "thinking" &&
            trimmed !== "error"
          );
        }),
        fc.string({ minLength: 0, maxLength: 100 }),
        (unknownType, someContent) => {
          const jsonObj = { type: unknownType, content: someContent };
          const content = JSON.stringify(jsonObj);
          const items = parseMessageContent(content, "stdout", 1);
          expect(items).toHaveLength(1);
          expect(items[0]!.type).toBe("text");
        }
      ),
      { numRuns: 100 }
    );
  });

  // ─── Sub-property: stderr with valid JSON type overrides error default ───

  it("stderr with valid JSON type field uses the JSON type, not error", () => {
    fc.assert(
      fc.property(
        fc.constantFrom("tool_use", "tool_result", "thinking") as fc.Arbitrary<string>,
        fc.string({ minLength: 1, maxLength: 50 }).filter((s) => s.trim().length > 0),
        (knownType, name) => {
          const jsonObj: Record<string, unknown> = { type: knownType, name };
          if (knownType === "tool_use") {
            jsonObj["input"] = {};
          }
          if (knownType === "tool_result") {
            jsonObj["output"] = "result";
          }
          if (knownType === "thinking") {
            jsonObj["content"] = "thinking...";
          }
          const content = JSON.stringify(jsonObj);
          const items = parseMessageContent(content, "stderr", 1);
          expect(items).toHaveLength(1);
          expect(items[0]!.type).toBe(knownType);
        }
      ),
      { numRuns: 100 }
    );
  });

  // ─── Sub-property: Empty content produces no items ───────────────────────

  it("empty or whitespace-only content produces no timeline items", () => {
    fc.assert(
      fc.property(
        fc.array(fc.constantFrom(" ", "\t", "\n", "\r"), { minLength: 0, maxLength: 20 }).map(
          (chars) => chars.join("")
        ),
        fc.constantFrom("stdout", "stderr", "stdin") as fc.Arbitrary<"stdout" | "stderr" | "stdin">,
        (whitespace, stream) => {
          const items = parseMessageContent(whitespace, stream, 1);
          expect(items).toHaveLength(0);
        }
      ),
      { numRuns: 100 }
    );
  });

  // ─── Sub-property: Every item has a valid type ───────────────────────────

  it("every parsed item has a type from the valid set", () => {
    const validTypes = new Set(["tool_use", "tool_result", "thinking", "text", "error"]);

    fc.assert(
      fc.property(
        fc.string({ minLength: 1, maxLength: 500 }).filter((s) => s.trim().length > 0),
        fc.constantFrom("stdout", "stderr", "stdin") as fc.Arbitrary<"stdout" | "stderr" | "stdin">,
        (content, stream) => {
          const items = parseMessageContent(content, stream, 1);
          for (const item of items) {
            expect(validTypes.has(item.type)).toBe(true);
          }
        }
      ),
      { numRuns: 100 }
    );
  });
});

// ─── Property 6: Deduplication by sequence number ────────────────────────────
// Feature: task-tool-chain-ui, Property 6: Deduplication by sequence number

/**
 * **Validates: Requirements 5.5**
 *
 * For any array of TaskMessages containing duplicate sequence numbers,
 * the resulting TimelineItem array SHALL contain at most one item per
 * unique sequence number.
 */

/** Generate a valid stream type for Property 6 */
const streamArbP6 = fc.constantFrom("stdout", "stderr", "stdin") as fc.Arbitrary<
  "stdout" | "stderr" | "stdin"
>;

/** Generate message content for Property 6: either plain text or JSON */
const messageContentArbP6: fc.Arbitrary<string> = fc.oneof(
  // Plain text (prefix to avoid accidental JSON parsing)
  fc.string({ minLength: 1, maxLength: 80 }).map((s) =>
    s.startsWith("{") ? "text_" + s : s
  ),
  // JSON tool_use
  fc
    .record({
      type: fc.constant("tool_use"),
      name: fc.string({ minLength: 1, maxLength: 20 }),
      input: fc.dictionary(
        fc.string({ minLength: 1, maxLength: 10 }),
        fc.string({ minLength: 1, maxLength: 30 })
      ),
    })
    .map((obj) => JSON.stringify(obj)),
  // JSON tool_result
  fc
    .record({
      type: fc.constant("tool_result"),
      output: fc.string({ minLength: 1, maxLength: 100 }),
    })
    .map((obj) => JSON.stringify(obj)),
  // JSON thinking
  fc
    .record({
      type: fc.constant("thinking"),
      content: fc.string({ minLength: 1, maxLength: 100 }),
    })
    .map((obj) => JSON.stringify(obj)),
  // JSON error
  fc
    .record({
      type: fc.constant("error"),
      message: fc.string({ minLength: 1, maxLength: 100 }),
    })
    .map((obj) => JSON.stringify(obj))
);

/** Generate a TaskMessage with a specified sequence number for Property 6 */
function taskMessageArbP6(seq: number): fc.Arbitrary<TaskMessage> {
  return fc.tuple(messageContentArbP6, streamArbP6).map(([content, stream]) => ({
    id: `msg-${seq}-${Math.random().toString(36).slice(2, 8)}`,
    task_id: "task-1",
    sequence: seq,
    stream,
    content,
    created_at: new Date().toISOString(),
  }));
}

/**
 * Generate an array of TaskMessages where some messages share the same sequence number.
 * Guarantees at least one duplicate sequence exists.
 */
const messagesWithDuplicatesArb: fc.Arbitrary<TaskMessage[]> = fc
  .array(fc.integer({ min: 1, max: 50 }), { minLength: 2, maxLength: 15 })
  .chain((sequences) => {
    // Force at least one duplicate by appending the first sequence again
    const withDuplicate = [...sequences, sequences[0]!];

    // Generate a TaskMessage for each sequence (including duplicates)
    const arbs = withDuplicate.map((seq) => taskMessageArbP6(seq));
    return fc.tuple(...arbs) as fc.Arbitrary<TaskMessage[]>;
  });

describe("Property 6: Deduplication by sequence number", () => {
  it("parsing messages with duplicates produces the same result as parsing only first occurrences", () => {
    fc.assert(
      fc.property(messagesWithDuplicatesArb, (messages) => {
        // Parse the full array (with duplicates)
        const itemsWithDuplicates = parseMessages(messages);

        // Build a deduplicated array keeping only first occurrence of each sequence
        const firstOccurrenceMessages: TaskMessage[] = [];
        const seen = new Set<number>();
        for (const msg of messages) {
          if (!seen.has(msg.sequence)) {
            seen.add(msg.sequence);
            firstOccurrenceMessages.push(msg);
          }
        }

        // Parse the deduplicated array
        const itemsWithoutDuplicates = parseMessages(firstOccurrenceMessages);

        // Results must be identical — duplicates are fully ignored
        expect(itemsWithDuplicates).toEqual(itemsWithoutDuplicates);
      }),
      { numRuns: 150 }
    );
  });

  it("processes exactly N unique source messages when input has N unique sequence numbers", () => {
    fc.assert(
      fc.property(messagesWithDuplicatesArb, (messages) => {
        const uniqueSequenceCount = new Set(
          messages.map((m) => m.sequence)
        ).size;
        const items = parseMessages(messages);

        // Build expected from first-occurrence only
        const deduped: TaskMessage[] = [];
        const seen = new Set<number>();
        for (const msg of messages) {
          if (!seen.has(msg.sequence)) {
            seen.add(msg.sequence);
            deduped.push(msg);
          }
        }

        // The deduped set should have exactly uniqueSequenceCount messages
        expect(deduped.length).toBe(uniqueSequenceCount);

        // Parsing the deduped set should produce the same item count
        const dedupedItems = parseMessages(deduped);
        expect(items.length).toBe(dedupedItems.length);
      }),
      { numRuns: 150 }
    );
  });

  it("output seq values are all unique (no duplicate seq in items)", () => {
    fc.assert(
      fc.property(messagesWithDuplicatesArb, (messages) => {
        const items = parseMessages(messages);

        const outputSeqs = items.map((item) => item.seq);
        const uniqueOutputSeqs = new Set(outputSeqs);

        expect(uniqueOutputSeqs.size).toBe(outputSeqs.length);
      }),
      { numRuns: 150 }
    );
  });
});

// ─── Property 2: Monotonic sequence assignment ───────────────────────────────
// Feature: task-tool-chain-ui, Property 2: Monotonic sequence assignment

/**
 * **Validates: Requirements 1.6**
 *
 * For any array of TaskMessages passed to the parser, the resulting
 * TimelineItem array SHALL have strictly monotonically increasing seq
 * values — that is, for all adjacent pairs items[i] and items[i+1],
 * items[i].seq < items[i+1].seq.
 */

/** Generate message content for Property 2 using valid fast-check v4 APIs */
const arbContentP2 = fc.oneof(
  // Plain text content
  fc.string({ minLength: 1, maxLength: 80 }),
  // JSON tool_use
  fc
    .record({
      type: fc.constant("tool_use"),
      name: fc.string({ minLength: 1, maxLength: 20 }),
      input: fc.dictionary(
        fc.string({ minLength: 1, maxLength: 10 }),
        fc.string({ minLength: 1, maxLength: 50 })
      ),
    })
    .map((obj) => JSON.stringify(obj)),
  // JSON tool_result
  fc
    .record({
      type: fc.constant("tool_result"),
      output: fc.string({ minLength: 1, maxLength: 100 }),
    })
    .map((obj) => JSON.stringify(obj)),
  // JSON thinking
  fc
    .record({
      type: fc.constant("thinking"),
      content: fc.string({ minLength: 1, maxLength: 100 }),
    })
    .map((obj) => JSON.stringify(obj)),
  // JSON error
  fc
    .record({
      type: fc.constant("error"),
      message: fc.string({ minLength: 1, maxLength: 100 }),
    })
    .map((obj) => JSON.stringify(obj))
);

const arbStreamP2 = fc.constantFrom("stdout", "stderr", "stdin") as fc.Arbitrary<
  "stdout" | "stderr" | "stdin"
>;

/** Generate a TaskMessage with a given sequence number for Property 2 */
function arbTaskMessageP2(seq: number): fc.Arbitrary<TaskMessage> {
  return fc.tuple(arbContentP2, arbStreamP2).map(([content, stream]) => ({
    id: `msg-${seq}`,
    task_id: "task-1",
    sequence: seq,
    stream,
    content,
    created_at: new Date().toISOString(),
  }));
}

/** Generate an array of TaskMessages with unique, increasing sequence numbers */
const taskMessagesArbP2 = fc
  .integer({ min: 1, max: 20 })
  .chain((length) => {
    const arbs = Array.from({ length }, (_, i) => arbTaskMessageP2(i + 1));
    return fc.tuple(...arbs);
  })
  .map((msgs) => [...msgs]);

/** Generate messages where content contains multiple JSON blocks (multi-block) */
const multiBlockMessagesArbP2 = fc
  .integer({ min: 1, max: 10 })
  .chain((length) => {
    return fc.tuple(
      ...Array.from({ length }, (_, i) =>
        fc
          .tuple(
            arbStreamP2,
            fc.array(
              fc.oneof(
                fc
                  .record({
                    type: fc.constant("tool_use"),
                    name: fc.string({ minLength: 1, maxLength: 15 }),
                    input: fc.constant({}),
                  })
                  .map((o) => JSON.stringify(o)),
                fc
                  .record({
                    type: fc.constant("tool_result"),
                    output: fc.string({ minLength: 1, maxLength: 50 }),
                  })
                  .map((o) => JSON.stringify(o)),
                fc
                  .record({
                    type: fc.constant("thinking"),
                    content: fc.string({ minLength: 1, maxLength: 50 }),
                  })
                  .map((o) => JSON.stringify(o))
              ),
              { minLength: 2, maxLength: 4 }
            )
          )
          .map(([stream, blocks]) => ({
            id: `msg-${i + 1}`,
            task_id: "task-1",
            sequence: i + 1,
            stream,
            content: blocks.join("\n"),
            created_at: new Date().toISOString(),
          }))
      )
    ) as fc.Arbitrary<TaskMessage[]>;
  });

describe("Property 2: Monotonic sequence assignment", () => {
  it("seq values are strictly monotonically increasing for any input messages", () => {
    fc.assert(
      fc.property(taskMessagesArbP2, (messages) => {
        const items = parseMessages(messages);

        // If fewer than 2 items, property holds vacuously
        if (items.length < 2) return true;

        for (let i = 0; i < items.length - 1; i++) {
          expect(items[i]!.seq).toBeLessThan(items[i + 1]!.seq);
        }

        return true;
      }),
      { numRuns: 150 }
    );
  });

  it("seq values start at 0 and are consecutive integers", () => {
    fc.assert(
      fc.property(taskMessagesArbP2, (messages) => {
        const items = parseMessages(messages);

        if (items.length === 0) return true;

        // First item should have seq 0
        expect(items[0]!.seq).toBe(0);

        // Each subsequent item should have seq exactly 1 more than previous
        for (let i = 1; i < items.length; i++) {
          expect(items[i]!.seq).toBe(items[i - 1]!.seq + 1);
        }

        return true;
      }),
      { numRuns: 150 }
    );
  });

  it("seq values are strictly increasing even with multi-block messages", () => {
    fc.assert(
      fc.property(multiBlockMessagesArbP2, (messages) => {
        const items = parseMessages(messages);

        if (items.length < 2) return true;

        for (let i = 0; i < items.length - 1; i++) {
          expect(items[i]!.seq).toBeLessThan(items[i + 1]!.seq);
        }

        return true;
      }),
      { numRuns: 150 }
    );
  });
});

// ─── Property 3: Timeline segment partitioning ───────────────────────────────
// Feature: task-tool-chain-ui, Property 3: Timeline segment partitioning

/**
 * **Validates: Requirements 3.1**
 *
 * For any non-empty array of TimelineItems, the computed timeline bar segments
 * SHALL form a complete partition of the array — the sum of all segment counts
 * equals the total item count, segments are contiguous, and each segment
 * contains only items of the same type.
 */

import { computeSegments } from "../tool-chain-parser";
import type { TimelineItem, TimelineItemType } from "../tool-chain-parser";

/** Valid timeline item types */
const validTypes: TimelineItemType[] = [
  "tool_use",
  "tool_result",
  "thinking",
  "text",
  "error",
];

/** Generate a single TimelineItem with a given seq */
function arbTimelineItem(seq: number): fc.Arbitrary<TimelineItem> {
  return fc
    .constantFrom(...validTypes)
    .map((type) => ({ seq, type }));
}

/** Generate a non-empty array of TimelineItems with sequential seq values */
const nonEmptyTimelineItemsArb: fc.Arbitrary<TimelineItem[]> = fc
  .integer({ min: 1, max: 50 })
  .chain((length) => {
    const arbs = Array.from({ length }, (_, i) => arbTimelineItem(i));
    return fc.tuple(...arbs);
  })
  .map((items) => [...items]);

describe("Property 3: Timeline segment partitioning", () => {
  it("sum of all segment counts equals total item count", () => {
    fc.assert(
      fc.property(nonEmptyTimelineItemsArb, (items) => {
        const segments = computeSegments(items);

        const totalSegmentCount = segments.reduce((sum, seg) => sum + seg.count, 0);
        expect(totalSegmentCount).toBe(items.length);
      }),
      { numRuns: 100 }
    );
  });

  it("segments are contiguous (no gaps or overlaps)", () => {
    fc.assert(
      fc.property(nonEmptyTimelineItemsArb, (items) => {
        const segments = computeSegments(items);

        // First segment starts at the first item's seq
        expect(segments[0]!.startSeq).toBe(items[0]!.seq);

        // Each subsequent segment starts exactly where the previous one ends
        for (let i = 1; i < segments.length; i++) {
          const prevSegEnd = segments[i - 1]!.startSeq + segments[i - 1]!.count;
          // The startSeq of the next segment should correspond to the item at that index
          const expectedStartSeq = items[prevSegEnd - items[0]!.seq]?.seq;
          expect(segments[i]!.startSeq).toBe(expectedStartSeq);
        }
      }),
      { numRuns: 100 }
    );
  });

  it("each segment contains only items of the same type", () => {
    fc.assert(
      fc.property(nonEmptyTimelineItemsArb, (items) => {
        const segments = computeSegments(items);

        let itemIndex = 0;
        for (const segment of segments) {
          for (let j = 0; j < segment.count; j++) {
            expect(items[itemIndex]!.type).toBe(segment.type);
            itemIndex++;
          }
        }

        // Ensure we consumed all items
        expect(itemIndex).toBe(items.length);
      }),
      { numRuns: 100 }
    );
  });

  it("adjacent segments have different types (segments are maximally merged)", () => {
    fc.assert(
      fc.property(nonEmptyTimelineItemsArb, (items) => {
        const segments = computeSegments(items);

        for (let i = 1; i < segments.length; i++) {
          expect(segments[i]!.type).not.toBe(segments[i - 1]!.type);
        }
      }),
      { numRuns: 100 }
    );
  });

  it("empty input produces empty segments", () => {
    const segments = computeSegments([]);
    expect(segments).toEqual([]);
  });

  it("single item produces exactly one segment with count 1", () => {
    fc.assert(
      fc.property(
        fc.constantFrom(...validTypes),
        (type) => {
          const items: TimelineItem[] = [{ seq: 0, type }];
          const segments = computeSegments(items);

          expect(segments).toHaveLength(1);
          expect(segments[0]!.type).toBe(type);
          expect(segments[0]!.count).toBe(1);
          expect(segments[0]!.startSeq).toBe(0);
        }
      ),
      { numRuns: 100 }
    );
  });
});


// ─── Property 4: Final result extraction ─────────────────────────────────────
// Feature: task-tool-chain-ui, Property 4: Final result extraction

/**
 * **Validates: Requirements 4.1, 4.3, 4.4**
 *
 * For any array of TimelineItems and a terminal task status, the
 * Final_Result_Panel extraction logic SHALL return the last text-type item
 * when status is "completed", the last error-type item when status is "failed",
 * and nothing when status is "running" or "pending".
 */

import { extractFinalResult } from "../tool-chain-parser";

/** Valid timeline item types for Property 4 */
const validTypesP4: TimelineItemType[] = [
  "tool_use",
  "tool_result",
  "thinking",
  "text",
  "error",
];

/** Generate a TimelineItem with a given seq and optional type constraint */
function arbTimelineItemP4(seq: number): fc.Arbitrary<TimelineItem> {
  return fc.constantFrom(...validTypesP4).chain((type): fc.Arbitrary<TimelineItem> => {
    const base: TimelineItem = { seq, type };
    if (type === "text" || type === "thinking" || type === "error") {
      return fc
        .string({ minLength: 1, maxLength: 100 })
        .map((content): TimelineItem => ({ ...base, content }));
    }
    if (type === "tool_use") {
      return fc
        .string({ minLength: 1, maxLength: 20 })
        .map((tool): TimelineItem => ({ ...base, tool, input: {} }));
    }
    // tool_result
    return fc
      .string({ minLength: 1, maxLength: 100 })
      .map((output): TimelineItem => ({ ...base, output }));
  });
}

/** Generate a non-empty array of TimelineItems for Property 4 */
const timelineItemsArbP4: fc.Arbitrary<TimelineItem[]> = fc
  .integer({ min: 1, max: 30 })
  .chain((length) => {
    const arbs = Array.from({ length }, (_, i) => arbTimelineItemP4(i));
    return fc.tuple(...arbs);
  })
  .map((items) => [...items]);

/** Generate an empty or non-empty array of TimelineItems for Property 4 */
const timelineItemsMaybeEmptyArbP4: fc.Arbitrary<TimelineItem[]> = fc.oneof(
  fc.constant([] as TimelineItem[]),
  timelineItemsArbP4
);

describe("Property 4: Final result extraction", () => {
  it('returns the last text-type item when status is "completed"', () => {
    fc.assert(
      fc.property(timelineItemsArbP4, (items) => {
        const result = extractFinalResult(items, "completed");

        // Find the expected last text-type item
        let expectedItem: TimelineItem | null = null;
        for (let i = items.length - 1; i >= 0; i--) {
          if (items[i]!.type === "text") {
            expectedItem = items[i]!;
            break;
          }
        }

        if (expectedItem === null) {
          expect(result).toBeNull();
        } else {
          expect(result).not.toBeNull();
          expect(result!.type).toBe("text");
          expect(result).toEqual(expectedItem);
        }
      }),
      { numRuns: 100 }
    );
  });

  it('returns the last error-type item when status is "failed"', () => {
    fc.assert(
      fc.property(timelineItemsArbP4, (items) => {
        const result = extractFinalResult(items, "failed");

        // Find the expected last error-type item
        let expectedItem: TimelineItem | null = null;
        for (let i = items.length - 1; i >= 0; i--) {
          if (items[i]!.type === "error") {
            expectedItem = items[i]!;
            break;
          }
        }

        if (expectedItem === null) {
          expect(result).toBeNull();
        } else {
          expect(result).not.toBeNull();
          expect(result!.type).toBe("error");
          expect(result).toEqual(expectedItem);
        }
      }),
      { numRuns: 100 }
    );
  });

  it('returns null when status is "running" regardless of items', () => {
    fc.assert(
      fc.property(timelineItemsMaybeEmptyArbP4, (items) => {
        const result = extractFinalResult(items, "running");
        expect(result).toBeNull();
      }),
      { numRuns: 100 }
    );
  });

  it('returns null when status is "pending" regardless of items', () => {
    fc.assert(
      fc.property(timelineItemsMaybeEmptyArbP4, (items) => {
        const result = extractFinalResult(items, "pending");
        expect(result).toBeNull();
      }),
      { numRuns: 100 }
    );
  });

  it("returned item (if any) is present in the input array", () => {
    fc.assert(
      fc.property(
        timelineItemsArbP4,
        fc.constantFrom("completed", "failed", "running", "pending"),
        (items, status) => {
          const result = extractFinalResult(items, status);

          if (result !== null) {
            // The returned item must be one of the items in the input array
            const found = items.some(
              (item) => item.seq === result.seq && item.type === result.type
            );
            expect(found).toBe(true);
          }
        }
      ),
      { numRuns: 100 }
    );
  });

  it('returns null for "completed" when no text-type items exist', () => {
    // Generate items that are NOT text type
    const nonTextItemsArb: fc.Arbitrary<TimelineItem[]> = fc
      .integer({ min: 1, max: 20 })
      .chain((length) => {
        const arbs = Array.from({ length }, (_, i) =>
          fc
            .constantFrom("tool_use", "tool_result", "thinking", "error" as TimelineItemType)
            .map((type) => ({ seq: i, type }) as TimelineItem)
        );
        return fc.tuple(...arbs);
      })
      .map((items) => [...items]);

    fc.assert(
      fc.property(nonTextItemsArb, (items) => {
        const result = extractFinalResult(items, "completed");
        expect(result).toBeNull();
      }),
      { numRuns: 100 }
    );
  });

  it('returns null for "failed" when no error-type items exist', () => {
    // Generate items that are NOT error type
    const nonErrorItemsArb: fc.Arbitrary<TimelineItem[]> = fc
      .integer({ min: 1, max: 20 })
      .chain((length) => {
        const arbs = Array.from({ length }, (_, i) =>
          fc
            .constantFrom("tool_use", "tool_result", "thinking", "text" as TimelineItemType)
            .map((type) => ({ seq: i, type }) as TimelineItem)
        );
        return fc.tuple(...arbs);
      })
      .map((items) => [...items]);

    fc.assert(
      fc.property(nonErrorItemsArb, (items) => {
        const result = extractFinalResult(items, "failed");
        expect(result).toBeNull();
      }),
      { numRuns: 100 }
    );
  });

  it("returns null for empty items array regardless of status", () => {
    fc.assert(
      fc.property(
        fc.constantFrom("completed", "failed", "running", "pending"),
        (status) => {
          const result = extractFinalResult([], status);
          expect(result).toBeNull();
        }
      ),
      { numRuns: 100 }
    );
  });
});


// ─── Property 10: Copy text formatting ───────────────────────────────────────
// Feature: task-tool-chain-ui, Property 10: Copy text formatting

/**
 * **Validates: Requirements 9.1, 9.3**
 *
 * For any array of visible TimelineItems, the copy-to-clipboard text SHALL
 * contain exactly one line per item, each line formatted as `[{label}] {summary}`,
 * and the total line count SHALL equal the item count.
 */

import { formatCopyText } from "../tool-chain-parser";

/** Generate a TimelineItem with richer content for copy text testing */
function arbTimelineItemForCopy(seq: number): fc.Arbitrary<TimelineItem> {
  return fc.oneof(
    // tool_use with tool name and input
    fc
      .record({
        tool: fc.string({ minLength: 1, maxLength: 30 }).filter((s) => s.trim().length > 0),
        input: fc.dictionary(
          fc.constantFrom("file_path", "command", "query", "pattern", "other_key"),
          fc.string({ minLength: 1, maxLength: 80 })
        ),
      })
      .map(({ tool, input }) => ({
        seq,
        type: "tool_use" as TimelineItemType,
        tool,
        input,
      })),
    // tool_result with output
    fc
      .string({ minLength: 1, maxLength: 100 })
      .map((output) => ({
        seq,
        type: "tool_result" as TimelineItemType,
        tool: "SomeTool",
        output,
      })),
    // thinking with content
    fc
      .string({ minLength: 1, maxLength: 100 })
      .map((content) => ({
        seq,
        type: "thinking" as TimelineItemType,
        content,
      })),
    // text with content
    fc
      .string({ minLength: 1, maxLength: 100 })
      .map((content) => ({
        seq,
        type: "text" as TimelineItemType,
        content,
      })),
    // error with content
    fc
      .string({ minLength: 1, maxLength: 100 })
      .map((content) => ({
        seq,
        type: "error" as TimelineItemType,
        content,
      }))
  );
}

/** Generate a non-empty array of TimelineItems for copy text testing */
const nonEmptyItemsForCopyArb: fc.Arbitrary<TimelineItem[]> = fc
  .integer({ min: 1, max: 30 })
  .chain((length) => {
    const arbs = Array.from({ length }, (_, i) => arbTimelineItemForCopy(i));
    return fc.tuple(...arbs);
  })
  .map((items) => [...items]);

describe("Property 10: Copy text formatting", () => {
  it("line count equals item count", () => {
    fc.assert(
      fc.property(nonEmptyItemsForCopyArb, (items) => {
        const text = formatCopyText(items);
        const lines = text.split("\n");
        expect(lines.length).toBe(items.length);
      }),
      { numRuns: 100 }
    );
  });

  it("each line starts with [ and contains ] (bracket format)", () => {
    fc.assert(
      fc.property(nonEmptyItemsForCopyArb, (items) => {
        const text = formatCopyText(items);
        const lines = text.split("\n");

        for (const line of lines) {
          expect(line.startsWith("[")).toBe(true);
          expect(line.includes("]")).toBe(true);
        }
      }),
      { numRuns: 100 }
    );
  });

  it("each line has the format [{label}] {summary} where label is non-empty", () => {
    fc.assert(
      fc.property(nonEmptyItemsForCopyArb, (items) => {
        const text = formatCopyText(items);
        const lines = text.split("\n");

        for (let i = 0; i < lines.length; i++) {
          const line = lines[i]!;
          const item = items[i]!;

          // Determine expected label based on item type
          let expectedLabel: string;
          switch (item.type) {
            case "tool_use":
              expectedLabel = item.tool || "Tool";
              break;
            case "tool_result":
              expectedLabel = "Result";
              break;
            case "thinking":
              expectedLabel = "Thinking";
              break;
            case "text":
              expectedLabel = "Text";
              break;
            case "error":
              expectedLabel = "Error";
              break;
          }

          // Line must start with [{expectedLabel}]
          const expectedPrefix = `[${expectedLabel}] `;
          expect(line.startsWith(expectedPrefix)).toBe(true);

          // Summary (after the prefix) must be non-empty
          const summary = line.slice(expectedPrefix.length);
          expect(summary.length).toBeGreaterThan(0);
        }
      }),
      { numRuns: 100 }
    );
  });

  it("empty input produces empty string", () => {
    const result = formatCopyText([]);
    expect(result).toBe("");
  });

  it("no trailing newline in output", () => {
    fc.assert(
      fc.property(nonEmptyItemsForCopyArb, (items) => {
        const text = formatCopyText(items);
        expect(text.endsWith("\n")).toBe(false);
      }),
      { numRuns: 100 }
    );
  });
});


// ─── Feature: structured-task-output ─────────────────────────────────────────

import { filterItems, sortItems, deriveSummary } from "../tool-chain-parser";

// ─── Property 9: Filter Correctness ──────────────────────────────────────────
// Feature: structured-task-output, Property 9: Filter Correctness

/**
 * **Validates: Requirements 5.2**
 *
 * For any array of TimelineItems and any Set of filter values, `filterItems`
 * returns only items where type is in the set OR `tool:${item.tool}` is in the
 * set. Empty filter set returns all items.
 */

/** Generate a TimelineItem with optional tool field for filter testing */
function arbTimelineItemForFilter(seq: number): fc.Arbitrary<TimelineItem> {
  return fc
    .record({
      type: fc.constantFrom(...validTypes),
      tool: fc.option(
        fc.string({ minLength: 1, maxLength: 30 }).filter((s) => s.trim().length > 0),
        { nil: undefined }
      ),
    })
    .map(({ type, tool }) => {
      const item: TimelineItem = { seq, type };
      if (tool !== undefined) {
        item.tool = tool;
      }
      return item;
    });
}

/** Generate a non-empty array of TimelineItems for filter testing */
const filterItemsArb: fc.Arbitrary<TimelineItem[]> = fc
  .integer({ min: 1, max: 30 })
  .chain((length) => {
    const arbs = Array.from({ length }, (_, i) => arbTimelineItemForFilter(i));
    return fc.tuple(...arbs);
  })
  .map((items) => [...items]);

/** Generate a filter set from possible type values and tool: prefixed values */
const filterSetArb: fc.Arbitrary<Set<string>> = fc
  .array(
    fc.oneof(
      fc.constantFrom("tool_use", "tool_result", "thinking", "text", "error"),
      fc.string({ minLength: 1, maxLength: 30 }).filter((s) => s.trim().length > 0).map((s) => `tool:${s}`)
    ),
    { minLength: 0, maxLength: 8 }
  )
  .map((arr) => new Set(arr));

describe("Property 9: Filter Correctness", () => {
  it("empty filter set returns all items", () => {
    fc.assert(
      fc.property(filterItemsArb, (items) => {
        const result = filterItems(items, new Set());
        expect(result).toEqual(items);
      }),
      { numRuns: 100 }
    );
  });

  it("filtered items only contain items matching type OR tool:toolName in filter set", () => {
    fc.assert(
      fc.property(filterItemsArb, filterSetArb, (items, filters) => {
        const result = filterItems(items, filters);

        if (filters.size === 0) {
          // Empty filter returns all
          expect(result).toEqual(items);
          return;
        }

        // Every returned item must match at least one filter criterion
        for (const item of result) {
          const matchesType = filters.has(item.type);
          const matchesTool = item.tool ? filters.has(`tool:${item.tool}`) : false;
          expect(matchesType || matchesTool).toBe(true);
        }

        // Every item in the original that matches should be in the result
        const expected = items.filter((item) => {
          if (filters.has(item.type)) return true;
          if (item.tool && filters.has(`tool:${item.tool}`)) return true;
          return false;
        });
        expect(result).toEqual(expected);
      }),
      { numRuns: 100 }
    );
  });

  it("result is a subset of the input (no new items introduced)", () => {
    fc.assert(
      fc.property(filterItemsArb, filterSetArb, (items, filters) => {
        const result = filterItems(items, filters);
        for (const item of result) {
          expect(items).toContainEqual(item);
        }
      }),
      { numRuns: 100 }
    );
  });
});

// ─── Property 10: Sort Direction Correctness ─────────────────────────────────
// Feature: structured-task-output, Property 10: Sort Direction Correctness

/**
 * **Validates: Requirements 6.2, 6.3**
 *
 * For any array of TimelineItems sorted by ascending seq,
 * `sortItems(items, "chronological")` returns same order, and
 * `sortItems(items, "newest_first")` returns reverse order.
 */

/** Generate a sorted (ascending seq) array of TimelineItems for sort testing */
const sortedItemsArb: fc.Arbitrary<TimelineItem[]> = fc
  .integer({ min: 1, max: 30 })
  .chain((length) => {
    const arbs = Array.from({ length }, (_, i) =>
      fc.constantFrom(...validTypes).map((type) => ({ seq: i, type }) as TimelineItem)
    );
    return fc.tuple(...arbs);
  })
  .map((items) => [...items]);

describe("Property 10: Sort Direction Correctness", () => {
  it('"chronological" returns items in the same order', () => {
    fc.assert(
      fc.property(sortedItemsArb, (items) => {
        const result = sortItems(items, "chronological");
        expect(result).toEqual(items);
      }),
      { numRuns: 100 }
    );
  });

  it('"newest_first" returns items in reverse order', () => {
    fc.assert(
      fc.property(sortedItemsArb, (items) => {
        const result = sortItems(items, "newest_first");
        const reversed = [...items].reverse();
        expect(result).toEqual(reversed);
      }),
      { numRuns: 100 }
    );
  });

  it('"newest_first" followed by "newest_first" returns original order', () => {
    fc.assert(
      fc.property(sortedItemsArb, (items) => {
        const reversed = sortItems(items, "newest_first");
        const doubleReversed = sortItems(reversed, "newest_first");
        expect(doubleReversed).toEqual(items);
      }),
      { numRuns: 100 }
    );
  });

  it("sort does not add or remove items", () => {
    fc.assert(
      fc.property(sortedItemsArb, (items) => {
        const chrono = sortItems(items, "chronological");
        const newest = sortItems(items, "newest_first");
        expect(chrono.length).toBe(items.length);
        expect(newest.length).toBe(items.length);
      }),
      { numRuns: 100 }
    );
  });
});

// ─── Property 11: Thinking Summary Bounds and Fallback ───────────────────────
// Feature: structured-task-output, Property 11: Thinking Summary Bounds and Fallback

/**
 * **Validates: Requirements 7.2, 7.5**
 *
 * For any TimelineItem with type "thinking", the summary preview is at most
 * 150 characters (excluding italic markers `_..._`). If content is
 * empty/whitespace, summary is "(empty)".
 */

describe("Property 11: Thinking Summary Bounds and Fallback", () => {
  it("thinking summary is at most 150 chars excluding italic markers", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1, maxLength: 500 }).filter((s) => s.trim().length > 0),
        (content) => {
          const item: TimelineItem = { seq: 0, type: "thinking", content };
          const summary = deriveSummary(item);

          // Summary should be wrapped in italic markers _..._
          expect(summary.startsWith("_")).toBe(true);
          expect(summary.endsWith("_")).toBe(true);

          // Extract inner content (between the _ markers)
          const inner = summary.slice(1, -1);
          // Inner content (excluding trailing ellipsis) should be at most 150 chars
          // The ellipsis character "…" counts as 1 char in the slice
          expect(inner.length).toBeLessThanOrEqual(151); // 150 chars + possible "…"
        }
      ),
      { numRuns: 100 }
    );
  });

  it("thinking with empty/whitespace content returns (empty)", () => {
    fc.assert(
      fc.property(
        fc.array(fc.constantFrom(" ", "\t", "\n", "\r"), { minLength: 0, maxLength: 20 }).map(
          (chars) => chars.join("")
        ),
        (whitespace) => {
          const item: TimelineItem = { seq: 0, type: "thinking", content: whitespace };
          const summary = deriveSummary(item);
          expect(summary).toBe("(empty)");
        }
      ),
      { numRuns: 100 }
    );
  });

  it("thinking with undefined content returns (empty)", () => {
    const item: TimelineItem = { seq: 0, type: "thinking" };
    const summary = deriveSummary(item);
    expect(summary).toBe("(empty)");
  });

  it("thinking content ≤150 chars produces summary without ellipsis", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1, maxLength: 150 }).filter((s) => s.trim().length > 0),
        (content) => {
          const item: TimelineItem = { seq: 0, type: "thinking", content };
          const summary = deriveSummary(item);
          // Should be _content_ without ellipsis
          expect(summary).toBe(`_${content}_`);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("thinking content >150 chars produces summary with ellipsis", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 151, maxLength: 500 }).filter((s) => s.trim().length > 0),
        (content) => {
          const item: TimelineItem = { seq: 0, type: "thinking", content };
          const summary = deriveSummary(item);
          // Should be _first150chars…_
          expect(summary).toBe(`_${content.slice(0, 150)}…_`);
          expect(summary.endsWith("…_")).toBe(true);
        }
      ),
      { numRuns: 100 }
    );
  });
});

// ─── Property 12: Error Summary Fallback ─────────────────────────────────────
// Feature: structured-task-output, Property 12: Error Summary Fallback

/**
 * **Validates: Requirements 8.5**
 *
 * For any TimelineItem with type "error" whose content is empty/whitespace,
 * the summary is "(no error details)".
 */

describe("Property 12: Error Summary Fallback", () => {
  it("error with empty/whitespace content returns (no error details)", () => {
    fc.assert(
      fc.property(
        fc.array(fc.constantFrom(" ", "\t", "\n", "\r"), { minLength: 0, maxLength: 20 }).map(
          (chars) => chars.join("")
        ),
        (whitespace) => {
          const item: TimelineItem = { seq: 0, type: "error", content: whitespace };
          const summary = deriveSummary(item);
          expect(summary).toBe("(no error details)");
        }
      ),
      { numRuns: 100 }
    );
  });

  it("error with undefined content returns (no error details)", () => {
    const item: TimelineItem = { seq: 0, type: "error" };
    const summary = deriveSummary(item);
    expect(summary).toBe("(no error details)");
  });

  it("error with non-empty content returns the content (not the fallback)", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1, maxLength: 200 }).filter((s) => s.trim().length > 0),
        (content) => {
          const item: TimelineItem = { seq: 0, type: "error", content };
          const summary = deriveSummary(item);
          expect(summary).not.toBe("(no error details)");
          expect(summary).toBe(content);
        }
      ),
      { numRuns: 100 }
    );
  });
});


// ─── Feature: structured-task-output ─────────────────────────────────────────

// ─── Property 5: Pure Function (No Mutation) ─────────────────────────────────
// Feature: structured-task-output, Property 5: Pure Function (No Mutation)

/**
 * **Validates: Requirements 2.4**
 *
 * For any input array, calling `parseMessages` does not modify the input array
 * or any of its elements. Deep comparison before and after shows no differences.
 */

import { deriveSummary, filterItems, sortItems } from "../tool-chain-parser";

describe("Feature: structured-task-output", () => {
  describe("Property 5: Pure Function (No Mutation)", () => {
    /** Generate a TaskMessage for mutation testing */
    const arbStreamP5 = fc.constantFrom("stdout", "stderr", "stdin") as fc.Arbitrary<
      "stdout" | "stderr" | "stdin"
    >;

    const arbContentP5 = fc.oneof(
      fc.string({ minLength: 1, maxLength: 80 }).map((s) =>
        s.startsWith("{") ? "text_" + s : s
      ),
      fc
        .record({
          type: fc.constant("tool_use"),
          name: fc.string({ minLength: 1, maxLength: 20 }),
          input: fc.dictionary(
            fc.string({ minLength: 1, maxLength: 10 }),
            fc.string({ minLength: 1, maxLength: 30 })
          ),
        })
        .map((obj) => JSON.stringify(obj)),
      fc
        .record({
          type: fc.constant("tool_result"),
          output: fc.string({ minLength: 1, maxLength: 100 }),
        })
        .map((obj) => JSON.stringify(obj)),
      fc
        .record({
          type: fc.constant("thinking"),
          content: fc.string({ minLength: 1, maxLength: 100 }),
        })
        .map((obj) => JSON.stringify(obj))
    );

    const arbMessagesP5: fc.Arbitrary<TaskMessage[]> = fc
      .integer({ min: 0, max: 15 })
      .chain((length) => {
        if (length === 0) return fc.constant([] as TaskMessage[]);
        const arbs = Array.from({ length }, (_, i) =>
          fc.tuple(arbContentP5, arbStreamP5).map(([content, stream]) => ({
            id: `msg-${i}`,
            task_id: "task-1",
            sequence: i + 1,
            stream,
            content,
            created_at: new Date().toISOString(),
          }))
        );
        return fc.tuple(...arbs).map((msgs) => [...msgs]);
      });

    it("parseMessages does not modify the input array length", () => {
      fc.assert(
        fc.property(arbMessagesP5, (messages) => {
          const originalLength = messages.length;
          parseMessages(messages);
          expect(messages.length).toBe(originalLength);
        }),
        { numRuns: 100 }
      );
    });

    it("parseMessages does not modify any element in the input array", () => {
      fc.assert(
        fc.property(arbMessagesP5, (messages) => {
          // Deep clone the input for comparison
          const snapshot = JSON.parse(JSON.stringify(messages));
          parseMessages(messages);
          expect(messages).toEqual(snapshot);
        }),
        { numRuns: 100 }
      );
    });

    it("parseMessages does not mutate individual message objects", () => {
      fc.assert(
        fc.property(arbMessagesP5, (messages) => {
          // Freeze each message to detect mutations (would throw in strict mode)
          const frozenMessages = messages.map((m) => ({ ...m }));
          const snapshot = frozenMessages.map((m) => ({ ...m }));
          parseMessages(frozenMessages);
          for (let i = 0; i < frozenMessages.length; i++) {
            expect(frozenMessages[i]).toEqual(snapshot[i]);
          }
        }),
        { numRuns: 100 }
      );
    });
  });

  // ─── Property 6: Deduplication by Sequence ─────────────────────────────────
  // Feature: structured-task-output, Property 6: Deduplication by Sequence

  /**
   * **Validates: Requirements 2.6**
   *
   * For any input array with duplicate sequence numbers, `parseMessages` retains
   * only the first occurrence and all output seq values are unique.
   */
  describe("Property 6: Deduplication by Sequence", () => {
    const arbStreamP6b = fc.constantFrom("stdout", "stderr", "stdin") as fc.Arbitrary<
      "stdout" | "stderr" | "stdin"
    >;

    const arbContentP6b = fc.oneof(
      fc.string({ minLength: 1, maxLength: 60 }).map((s) =>
        s.startsWith("{") ? "text_" + s : s
      ),
      fc
        .record({
          type: fc.constant("tool_use"),
          name: fc.string({ minLength: 1, maxLength: 15 }),
          input: fc.constant({}),
        })
        .map((obj) => JSON.stringify(obj)),
      fc
        .record({
          type: fc.constant("thinking"),
          content: fc.string({ minLength: 1, maxLength: 50 }),
        })
        .map((obj) => JSON.stringify(obj))
    );

    /** Generate messages with guaranteed duplicate sequences */
    const arbMessagesWithDupsP6b: fc.Arbitrary<TaskMessage[]> = fc
      .array(fc.integer({ min: 1, max: 30 }), { minLength: 2, maxLength: 12 })
      .chain((sequences) => {
        // Force at least one duplicate
        const withDup = [...sequences, sequences[0]!];
        const arbs = withDup.map((seq, idx) =>
          fc.tuple(arbContentP6b, arbStreamP6b).map(([content, stream]) => ({
            id: `msg-${idx}-${seq}`,
            task_id: "task-1",
            sequence: seq,
            stream,
            content,
            created_at: new Date().toISOString(),
          }))
        );
        return fc.tuple(...arbs).map((msgs) => [...msgs]);
      });

    it("retains only the first occurrence of each sequence number", () => {
      fc.assert(
        fc.property(arbMessagesWithDupsP6b, (messages) => {
          const items = parseMessages(messages);

          // Build expected: keep only first occurrence per sequence
          const seen = new Set<number>();
          const firstOccurrences: TaskMessage[] = [];
          for (const msg of messages) {
            if (!seen.has(msg.sequence)) {
              seen.add(msg.sequence);
              firstOccurrences.push(msg);
            }
          }

          const expectedItems = parseMessages(firstOccurrences);
          expect(items).toEqual(expectedItems);
        }),
        { numRuns: 100 }
      );
    });

    it("all output seq values are unique", () => {
      fc.assert(
        fc.property(arbMessagesWithDupsP6b, (messages) => {
          const items = parseMessages(messages);
          const seqs = items.map((item) => item.seq);
          const uniqueSeqs = new Set(seqs);
          expect(uniqueSeqs.size).toBe(seqs.length);
        }),
        { numRuns: 100 }
      );
    });
  });

  // ─── Property 7: Segment Computation Invariants ────────────────────────────
  // Feature: structured-task-output, Property 7: Segment Computation Invariants

  /**
   * **Validates: Requirements 3.1, 3.2**
   *
   * For any non-empty array of TimelineItems, `computeSegments` produces segments
   * where: sum of counts equals total items, each segment has a single type, and
   * adjacent segments have different types.
   */
  describe("Property 7: Segment Computation Invariants", () => {
    const validTypesP7: TimelineItemType[] = [
      "tool_use",
      "tool_result",
      "thinking",
      "text",
      "error",
    ];

    /** Generate a non-empty array of TimelineItems with sequential seq values */
    const arbNonEmptyItemsP7: fc.Arbitrary<TimelineItem[]> = fc
      .integer({ min: 1, max: 50 })
      .chain((length) => {
        const arbs = Array.from({ length }, (_, i) =>
          fc.constantFrom(...validTypesP7).map(
            (type): TimelineItem => ({ seq: i, type })
          )
        );
        return fc.tuple(...arbs);
      })
      .map((items) => [...items]);

    it("sum of segment counts equals total item count", () => {
      fc.assert(
        fc.property(arbNonEmptyItemsP7, (items) => {
          const segments = computeSegments(items);
          const totalCount = segments.reduce((sum, seg) => sum + seg.count, 0);
          expect(totalCount).toBe(items.length);
        }),
        { numRuns: 100 }
      );
    });

    it("each segment contains only items of a single type", () => {
      fc.assert(
        fc.property(arbNonEmptyItemsP7, (items) => {
          const segments = computeSegments(items);
          let idx = 0;
          for (const segment of segments) {
            for (let j = 0; j < segment.count; j++) {
              expect(items[idx]!.type).toBe(segment.type);
              idx++;
            }
          }
          expect(idx).toBe(items.length);
        }),
        { numRuns: 100 }
      );
    });

    it("adjacent segments have different types", () => {
      fc.assert(
        fc.property(arbNonEmptyItemsP7, (items) => {
          const segments = computeSegments(items);
          for (let i = 1; i < segments.length; i++) {
            expect(segments[i]!.type).not.toBe(segments[i - 1]!.type);
          }
        }),
        { numRuns: 100 }
      );
    });
  });

  // ─── Property 8: Summary Derivation Bounds ─────────────────────────────────
  // Feature: structured-task-output, Property 8: Summary Derivation Bounds

  /**
   * **Validates: Requirements 4.2, 4.3**
   *
   * For any TimelineItem with type "tool_use", `deriveSummary` returns a string
   * of at most 120 characters. If no valid summary can be derived, returns "(no details)".
   */
  describe("Property 8: Summary Derivation Bounds", () => {
    /** Generate a tool_use TimelineItem with arbitrary input */
    const arbToolUseItem: fc.Arbitrary<TimelineItem> = fc
      .record({
        seq: fc.nat({ max: 1000 }),
        tool: fc.oneof(
          fc.string({ minLength: 1, maxLength: 50 }),
          fc.constant("unknown")
        ),
        input: fc.oneof(
          // Empty input
          fc.constant({} as Record<string, unknown>),
          // Input with query
          fc.record({
            query: fc.string({ minLength: 0, maxLength: 200 }),
          }) as fc.Arbitrary<Record<string, unknown>>,
          // Input with file_path
          fc.record({
            file_path: fc.string({ minLength: 0, maxLength: 200 }),
          }) as fc.Arbitrary<Record<string, unknown>>,
          // Input with path
          fc.record({
            path: fc.string({ minLength: 0, maxLength: 200 }),
          }) as fc.Arbitrary<Record<string, unknown>>,
          // Input with command
          fc.record({
            command: fc.string({ minLength: 0, maxLength: 300 }),
          }) as fc.Arbitrary<Record<string, unknown>>,
          // Input with pattern
          fc.record({
            pattern: fc.string({ minLength: 0, maxLength: 200 }),
          }) as fc.Arbitrary<Record<string, unknown>>,
          // Input with only non-string or long string values
          fc.dictionary(
            fc.string({ minLength: 1, maxLength: 10 }),
            fc.oneof(
              fc.integer(),
              fc.boolean(),
              fc.constant(null),
              fc.string({ minLength: 121, maxLength: 200 })
            )
          ) as fc.Arbitrary<Record<string, unknown>>,
          // Input with mixed values
          fc.dictionary(
            fc.string({ minLength: 1, maxLength: 15 }),
            fc.oneof(
              fc.string({ minLength: 0, maxLength: 200 }),
              fc.integer(),
              fc.boolean()
            )
          ) as fc.Arbitrary<Record<string, unknown>>
        ),
      })
      .map(({ seq, tool, input }): TimelineItem => ({
        seq,
        type: "tool_use",
        tool,
        input,
      }));

    it("deriveSummary returns at most 120 characters for tool_use items", () => {
      fc.assert(
        fc.property(arbToolUseItem, (item) => {
          const summary = deriveSummary(item);
          expect(summary.length).toBeLessThanOrEqual(120);
        }),
        { numRuns: 100 }
      );
    });

    it('deriveSummary returns "(no details)" when no valid summary can be derived', () => {
      fc.assert(
        fc.property(
          fc.nat({ max: 1000 }),
          (seq) => {
            // Item with input where all values are non-string, empty, or >120 chars
            const item: TimelineItem = {
              seq,
              type: "tool_use",
              tool: "some_tool",
              input: {
                count: 42,
                flag: true,
                nested: { key: "value" },
                longStr: "x".repeat(121),
                emptyStr: "",
              },
            };
            const summary = deriveSummary(item);
            expect(summary).toBe("(no details)");
          }
        ),
        { numRuns: 100 }
      );
    });

    it("deriveSummary always returns a non-empty string for tool_use items", () => {
      fc.assert(
        fc.property(arbToolUseItem, (item) => {
          const summary = deriveSummary(item);
          expect(summary.length).toBeGreaterThan(0);
        }),
        { numRuns: 100 }
      );
    });
  });
});


// ═══════════════════════════════════════════════════════════════════════════════
// Feature: structured-task-output — Property-Based Tests (Properties 1–4)
// ═══════════════════════════════════════════════════════════════════════════════

import { filterItems, sortItems, deriveSummary } from "../tool-chain-parser";

describe("Feature: structured-task-output", () => {
  // ─── Shared Generators ───────────────────────────────────────────────────

  const validTimelineTypes: TimelineItemType[] = [
    "tool_use",
    "tool_result",
    "thinking",
    "text",
    "error",
  ];

  const streamArb = fc.constantFrom("stdout", "stderr", "stdin") as fc.Arbitrary<
    "stdout" | "stderr" | "stdin"
  >;

  /**
   * Generate message content that covers all parsing paths:
   * - Plain text (non-JSON)
   * - JSON with valid type fields
   * - JSON with invalid/unknown type fields
   * - JSON with missing tool field for tool_use
   */
  const messageContentArb: fc.Arbitrary<string> = fc.oneof(
    // Plain text (ensure it doesn't start with '{' to avoid JSON parsing)
    fc
      .string({ minLength: 1, maxLength: 100 })
      .map((s) => (s.trim().startsWith("{") ? "text_" + s : s))
      .filter((s) => s.trim().length > 0),
    // JSON tool_use with name
    fc
      .record({
        type: fc.constant("tool_use"),
        name: fc.string({ minLength: 1, maxLength: 30 }).filter((s) => s.trim().length > 0),
        input: fc.dictionary(
          fc.string({ minLength: 1, maxLength: 10 }).filter((s) => s.trim().length > 0),
          fc.string({ minLength: 1, maxLength: 50 })
        ),
      })
      .map((obj) => JSON.stringify(obj)),
    // JSON tool_result
    fc
      .record({
        type: fc.constant("tool_result"),
        name: fc.string({ minLength: 1, maxLength: 30 }).filter((s) => s.trim().length > 0),
        output: fc.string({ minLength: 1, maxLength: 100 }),
      })
      .map((obj) => JSON.stringify(obj)),
    // JSON thinking
    fc
      .record({
        type: fc.constant("thinking"),
        content: fc.string({ minLength: 1, maxLength: 100 }),
      })
      .map((obj) => JSON.stringify(obj)),
    // JSON error
    fc
      .record({
        type: fc.constant("error"),
        message: fc.string({ minLength: 1, maxLength: 100 }),
      })
      .map((obj) => JSON.stringify(obj)),
    // JSON with unknown type (should default to "text")
    fc
      .record({
        type: fc
          .string({ minLength: 1, maxLength: 20 })
          .filter(
            (s) =>
              s.trim().length > 0 &&
              !["tool_use", "tool_result", "thinking", "error", "text"].includes(s)
          ),
        content: fc.string({ minLength: 1, maxLength: 50 }),
      })
      .map((obj) => JSON.stringify(obj))
  );

  /** Generate a TaskMessage with a given sequence number */
  function taskMessageArb(seq: number): fc.Arbitrary<TaskMessage> {
    return fc.tuple(messageContentArb, streamArb).map(([content, stream]) => ({
      id: `msg-${seq}-${Math.random().toString(36).slice(2, 8)}`,
      task_id: "task-test",
      sequence: seq,
      stream,
      content,
      created_at: new Date().toISOString(),
    }));
  }

  /** Generate an array of TaskMessages with unique sequence numbers */
  const uniqueSeqMessagesArb: fc.Arbitrary<TaskMessage[]> = fc
    .integer({ min: 1, max: 20 })
    .chain((length) => {
      const arbs = Array.from({ length }, (_, i) => taskMessageArb(i + 1));
      return fc.tuple(...arbs);
    })
    .map((msgs) => [...msgs]);

  /** Generate an array of TaskMessages with potentially shuffled/duplicate sequences */
  const arbitraryMessagesArb: fc.Arbitrary<TaskMessage[]> = fc
    .array(fc.integer({ min: 1, max: 50 }), { minLength: 1, maxLength: 15 })
    .chain((sequences) => {
      const arbs = sequences.map((seq) => taskMessageArb(seq));
      return fc.tuple(...arbs) as fc.Arbitrary<TaskMessage[]>;
    });

  // ─── Property 1: Type Normalization ────────────────────────────────────────
  // Feature: structured-task-output, Property 1: Type Normalization
  //
  // **Validates: Requirements 1.1, 1.6, 1.7, 2.7, 9.6**

  describe("Property 1: Type Normalization", () => {
    it("parseMessages always produces items with valid type values", () => {
      const validTypeSet = new Set<string>(validTimelineTypes);

      fc.assert(
        fc.property(arbitraryMessagesArb, (messages) => {
          const items = parseMessages(messages);
          for (const item of items) {
            expect(validTypeSet.has(item.type)).toBe(true);
          }
        }),
        { numRuns: 100 }
      );
    });

    it("legacy messages without type field: stdout/stdin → text, stderr → error", () => {
      // Generate plain text messages (non-JSON) to test legacy fallback
      const plainTextContentArb = fc
        .string({ minLength: 1, maxLength: 80 })
        .map((s) => (s.trim().startsWith("{") ? "plain_" + s : s))
        .filter((s) => s.trim().length > 0);

      fc.assert(
        fc.property(
          plainTextContentArb,
          streamArb,
          fc.integer({ min: 1, max: 100 }),
          (content, stream, seq) => {
            const messages: TaskMessage[] = [
              {
                id: `msg-${seq}`,
                task_id: "task-test",
                sequence: seq,
                stream,
                content,
                created_at: new Date().toISOString(),
              },
            ];
            const items = parseMessages(messages);
            for (const item of items) {
              if (stream === "stderr") {
                expect(item.type).toBe("error");
              } else {
                // stdout and stdin → text
                expect(item.type).toBe("text");
              }
            }
          }
        ),
        { numRuns: 100 }
      );
    });

    it("messages with unknown JSON type field are normalized to text", () => {
      const unknownTypeArb = fc
        .string({ minLength: 1, maxLength: 20 })
        .filter(
          (s) =>
            s.trim().length > 0 &&
            !["tool_use", "tool_result", "thinking", "error", "text"].includes(s.trim())
        );

      fc.assert(
        fc.property(
          unknownTypeArb,
          fc.string({ minLength: 0, maxLength: 50 }),
          fc.integer({ min: 1, max: 100 }),
          (unknownType, someContent, seq) => {
            const jsonContent = JSON.stringify({ type: unknownType, content: someContent });
            const messages: TaskMessage[] = [
              {
                id: `msg-${seq}`,
                task_id: "task-test",
                sequence: seq,
                stream: "stdout",
                content: jsonContent,
                created_at: new Date().toISOString(),
              },
            ];
            const items = parseMessages(messages);
            expect(items.length).toBeGreaterThanOrEqual(1);
            for (const item of items) {
              expect(item.type).toBe("text");
            }
          }
        ),
        { numRuns: 100 }
      );
    });
  });

  // ─── Property 2: Tool Field Fallback ───────────────────────────────────────
  // Feature: structured-task-output, Property 2: Tool Field Fallback
  //
  // **Validates: Requirements 1.8**

  describe("Property 2: Tool Field Fallback", () => {
    it('tool_use messages with absent/empty tool field produce items with tool="unknown"', () => {
      // Generate tool_use JSON without a name/tool field, or with empty name
      const toolUseMissingToolArb: fc.Arbitrary<string> = fc.oneof(
        // No name or tool field at all
        fc
          .dictionary(
            fc.string({ minLength: 1, maxLength: 10 }).filter((s) => s.trim().length > 0),
            fc.string({ minLength: 1, maxLength: 30 })
          )
          .map((input) => JSON.stringify({ type: "tool_use", input })),
        // Empty name field
        fc
          .dictionary(
            fc.string({ minLength: 1, maxLength: 10 }).filter((s) => s.trim().length > 0),
            fc.string({ minLength: 1, maxLength: 30 })
          )
          .map((input) => JSON.stringify({ type: "tool_use", name: "", input })),
        // Null-ish name (undefined is not serializable, so just omit)
        fc.constant(JSON.stringify({ type: "tool_use", input: { key: "value" } }))
      );

      fc.assert(
        fc.property(
          toolUseMissingToolArb,
          fc.integer({ min: 1, max: 100 }),
          (content, seq) => {
            const messages: TaskMessage[] = [
              {
                id: `msg-${seq}`,
                task_id: "task-test",
                sequence: seq,
                stream: "stdout",
                content,
                created_at: new Date().toISOString(),
              },
            ];
            const items = parseMessages(messages);
            const toolUseItems = items.filter((i) => i.type === "tool_use");
            for (const item of toolUseItems) {
              expect(item.tool).toBe("unknown");
            }
          }
        ),
        { numRuns: 100 }
      );
    });

    it("tool_use messages with a valid tool name preserve that name", () => {
      const toolNameArb = fc
        .string({ minLength: 1, maxLength: 50 })
        .filter((s) => s.trim().length > 0);

      fc.assert(
        fc.property(
          toolNameArb,
          fc.integer({ min: 1, max: 100 }),
          (toolName, seq) => {
            const content = JSON.stringify({
              type: "tool_use",
              name: toolName,
              input: { file: "test.ts" },
            });
            const messages: TaskMessage[] = [
              {
                id: `msg-${seq}`,
                task_id: "task-test",
                sequence: seq,
                stream: "stdout",
                content,
                created_at: new Date().toISOString(),
              },
            ];
            const items = parseMessages(messages);
            const toolUseItems = items.filter((i) => i.type === "tool_use");
            expect(toolUseItems.length).toBe(1);
            expect(toolUseItems[0]!.tool).toBe(toolName);
          }
        ),
        { numRuns: 100 }
      );
    });
  });

  // ─── Property 3: Output Sorted by Sequence ────────────────────────────────
  // Feature: structured-task-output, Property 3: Output Sorted by Sequence
  //
  // **Validates: Requirements 2.1**

  describe("Property 3: Output Sorted by Sequence", () => {
    it("output items are in strictly non-decreasing order by seq", () => {
      fc.assert(
        fc.property(arbitraryMessagesArb, (messages) => {
          const items = parseMessages(messages);
          for (let i = 1; i < items.length; i++) {
            expect(items[i]!.seq).toBeGreaterThanOrEqual(items[i - 1]!.seq);
          }
        }),
        { numRuns: 100 }
      );
    });

    it("output is sorted even when input messages are in random order", () => {
      // Generate messages with shuffled sequence numbers
      const shuffledMessagesArb: fc.Arbitrary<TaskMessage[]> = fc
        .array(fc.integer({ min: 1, max: 50 }), { minLength: 2, maxLength: 15 })
        .chain((sequences) => {
          // Shuffle the sequences
          const shuffled = [...sequences].sort(() => Math.random() - 0.5);
          const arbs = shuffled.map((seq) => taskMessageArb(seq));
          return fc.tuple(...arbs) as fc.Arbitrary<TaskMessage[]>;
        });

      fc.assert(
        fc.property(shuffledMessagesArb, (messages) => {
          const items = parseMessages(messages);
          for (let i = 1; i < items.length; i++) {
            expect(items[i]!.seq).toBeGreaterThanOrEqual(items[i - 1]!.seq);
          }
        }),
        { numRuns: 100 }
      );
    });
  });

  // ─── Property 4: One-to-One Mapping with Field Preservation ────────────────
  // Feature: structured-task-output, Property 4: One-to-One Mapping with Field Preservation
  //
  // **Validates: Requirements 2.2, 2.3**

  describe("Property 4: One-to-One Mapping with Field Preservation", () => {
    /**
     * Generate messages that each produce exactly ONE timeline item.
     * This means single-block content (no multi-JSON messages).
     */
    const singleBlockMessageArb = (seq: number): fc.Arbitrary<TaskMessage> => {
      const contentArb: fc.Arbitrary<{ content: string; expectedType: TimelineItemType; expectedFields: Partial<TimelineItem> }> = fc.oneof(
        // tool_use with name and input
        fc
          .record({
            name: fc.string({ minLength: 1, maxLength: 30 }).filter((s) => s.trim().length > 0),
            input: fc.dictionary(
              fc.string({ minLength: 1, maxLength: 10 }).filter((s) => s.trim().length > 0),
              fc.string({ minLength: 1, maxLength: 30 })
            ),
          })
          .map(({ name, input }) => ({
            content: JSON.stringify({ type: "tool_use", name, input }),
            expectedType: "tool_use" as TimelineItemType,
            expectedFields: { tool: name, input },
          })),
        // tool_result with output
        fc
          .record({
            name: fc.string({ minLength: 1, maxLength: 30 }).filter((s) => s.trim().length > 0),
            output: fc.string({ minLength: 1, maxLength: 100 }).filter((s) => s.length > 0),
          })
          .map(({ name, output }) => ({
            content: JSON.stringify({ type: "tool_result", name, output }),
            expectedType: "tool_result" as TimelineItemType,
            expectedFields: { tool: name, output },
          })),
        // thinking with content
        fc
          .string({ minLength: 1, maxLength: 100 })
          .map((thinkContent) => ({
            content: JSON.stringify({ type: "thinking", content: thinkContent }),
            expectedType: "thinking" as TimelineItemType,
            expectedFields: { content: thinkContent },
          }))
      );

      return contentArb.map(({ content }) => ({
        id: `msg-${seq}`,
        task_id: "task-test",
        sequence: seq,
        stream: "stdout" as const,
        content,
        created_at: new Date().toISOString(),
      }));
    };

    const uniqueSeqSingleBlockArb: fc.Arbitrary<TaskMessage[]> = fc
      .integer({ min: 1, max: 15 })
      .chain((length) => {
        const arbs = Array.from({ length }, (_, i) => singleBlockMessageArb(i + 1));
        return fc.tuple(...arbs);
      })
      .map((msgs) => [...msgs]);

    it("produces exactly one TimelineItem per input message with unique sequences", () => {
      fc.assert(
        fc.property(uniqueSeqSingleBlockArb, (messages) => {
          const items = parseMessages(messages);
          // Each single-block message produces exactly one item
          expect(items.length).toBe(messages.length);
        }),
        { numRuns: 100 }
      );
    });

    it("preserves tool field from tool_use messages", () => {
      const toolNameArb = fc
        .string({ minLength: 1, maxLength: 30 })
        .filter((s) => s.trim().length > 0);

      fc.assert(
        fc.property(
          toolNameArb,
          fc.dictionary(
            fc.string({ minLength: 1, maxLength: 10 }).filter((s) => s.trim().length > 0),
            fc.string({ minLength: 1, maxLength: 30 })
          ),
          fc.integer({ min: 1, max: 100 }),
          (toolName, input, seq) => {
            const content = JSON.stringify({ type: "tool_use", name: toolName, input });
            const messages: TaskMessage[] = [
              {
                id: `msg-${seq}`,
                task_id: "task-test",
                sequence: seq,
                stream: "stdout",
                content,
                created_at: new Date().toISOString(),
              },
            ];
            const items = parseMessages(messages);
            expect(items.length).toBe(1);
            expect(items[0]!.tool).toBe(toolName);
            expect(items[0]!.input).toEqual(input);
          }
        ),
        { numRuns: 100 }
      );
    });

    it("preserves output field from tool_result messages", () => {
      fc.assert(
        fc.property(
          fc.string({ minLength: 1, maxLength: 30 }).filter((s) => s.trim().length > 0),
          fc.string({ minLength: 1, maxLength: 200 }).filter((s) => s.length > 0),
          fc.integer({ min: 1, max: 100 }),
          (toolName, output, seq) => {
            const content = JSON.stringify({ type: "tool_result", name: toolName, output });
            const messages: TaskMessage[] = [
              {
                id: `msg-${seq}`,
                task_id: "task-test",
                sequence: seq,
                stream: "stdout",
                content,
                created_at: new Date().toISOString(),
              },
            ];
            const items = parseMessages(messages);
            expect(items.length).toBe(1);
            expect(items[0]!.output).toBe(output);
          }
        ),
        { numRuns: 100 }
      );
    });

    it("preserves content field from thinking messages", () => {
      fc.assert(
        fc.property(
          fc.string({ minLength: 1, maxLength: 200 }),
          fc.integer({ min: 1, max: 100 }),
          (thinkContent, seq) => {
            const content = JSON.stringify({ type: "thinking", content: thinkContent });
            const messages: TaskMessage[] = [
              {
                id: `msg-${seq}`,
                task_id: "task-test",
                sequence: seq,
                stream: "stdout",
                content,
                created_at: new Date().toISOString(),
              },
            ];
            const items = parseMessages(messages);
            expect(items.length).toBe(1);
            expect(items[0]!.content).toBe(thinkContent);
          }
        ),
        { numRuns: 100 }
      );
    });
  });
});
