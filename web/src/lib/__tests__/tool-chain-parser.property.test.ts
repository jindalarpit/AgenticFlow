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
