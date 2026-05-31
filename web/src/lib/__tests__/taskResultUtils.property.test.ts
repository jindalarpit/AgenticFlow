// Feature: task-result-display, Property 1: Content truncation preserves invariants
// **Validates: Requirements 1.3, 3.3**
// Feature: task-result-display, Property 3: Final result extraction concatenates all stdout messages
// **Validates: Requirements 3.1, 3.2**

import { describe, it, expect } from "vitest";
import fc from "fast-check";
import { truncateResultContent, extractDashboardResult } from "../taskResultUtils";
import type { TaskMessage } from "../../hooks/useTasks";

describe("truncateResultContent — Property 1: Content truncation preserves invariants", () => {
  it("when content.length <= 2000: displayText === content and isTruncated === false", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 0, maxLength: 2000 }),
        (content) => {
          const result = truncateResultContent(content);
          expect(result.displayText).toBe(content);
          expect(result.isTruncated).toBe(false);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("when content.length > 2000: displayText === content.slice(0, 500) and isTruncated === true", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 2001, maxLength: 10000 }),
        (content) => {
          const result = truncateResultContent(content);
          expect(result.displayText).toBe(content.slice(0, 500));
          expect(result.isTruncated).toBe(true);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("fullText always === content regardless of length", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 0, maxLength: 10000 }),
        (content) => {
          const result = truncateResultContent(content);
          expect(result.fullText).toBe(content);
        }
      ),
      { numRuns: 100 }
    );
  });
});

// ─── Generators ─────────────────────────────────────────────────────────────

/** Arbitrary for a TaskMessage object. */
const arbTaskMessage = (stream: "stdout" | "stderr" | "stdin"): fc.Arbitrary<TaskMessage> =>
  fc.record({
    id: fc.uuid(),
    task_id: fc.uuid(),
    sequence: fc.integer({ min: 0, max: 100000 }),
    stream: fc.constant(stream),
    content: fc.string({ minLength: 1, maxLength: 500 }),
    created_at: fc.constant("2025-01-01T00:00:00Z"),
  });

/** Arbitrary for a TaskMessage with any stream type. */
const arbAnyMessage: fc.Arbitrary<TaskMessage> = fc.oneof(
  arbTaskMessage("stdout"),
  arbTaskMessage("stderr"),
  arbTaskMessage("stdin")
);

// ─── Property 3 ─────────────────────────────────────────────────────────────

describe("extractDashboardResult — Property 3: Final result extraction concatenates all stdout messages", () => {
  it("returns concatenation of all stdout messages sorted by sequence when stdout messages exist", () => {
    fc.assert(
      fc.property(
        // Generate at least one stdout message plus arbitrary other messages
        fc.array(arbTaskMessage("stdout"), { minLength: 1, maxLength: 20 }),
        fc.array(arbAnyMessage, { minLength: 0, maxLength: 20 }),
        fc.option(fc.string({ minLength: 1, maxLength: 200 }), { nil: null }),
        (stdoutMessages, otherMessages, outputPreview) => {
          const allMessages = [...stdoutMessages, ...otherMessages];

          const result = extractDashboardResult(allMessages, outputPreview);

          // Expected: all stdout messages sorted by sequence, concatenated
          const allStdout = allMessages
            .filter((m) => m.stream === "stdout")
            .sort((a, b) => a.sequence - b.sequence);
          const expected = allStdout.map((m) => m.content).join("");

          expect(result).toBe(expected || null);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("returns outputPreview when no stdout messages exist", () => {
    fc.assert(
      fc.property(
        // Generate only non-stdout messages
        fc.array(
          fc.oneof(arbTaskMessage("stderr"), arbTaskMessage("stdin")),
          { minLength: 1, maxLength: 20 }
        ),
        fc.option(fc.string({ minLength: 1, maxLength: 200 }), { nil: null }),
        (messages, outputPreview) => {
          const result = extractDashboardResult(messages, outputPreview);
          expect(result).toBe(outputPreview);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("returns null when no stdout messages exist and outputPreview is null", () => {
    fc.assert(
      fc.property(
        fc.array(
          fc.oneof(arbTaskMessage("stderr"), arbTaskMessage("stdin")),
          { minLength: 0, maxLength: 20 }
        ),
        (messages) => {
          const result = extractDashboardResult(messages, null);
          expect(result).toBeNull();
        }
      ),
      { numRuns: 100 }
    );
  });
});
