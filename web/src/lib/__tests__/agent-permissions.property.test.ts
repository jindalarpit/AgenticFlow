// Feature: agent-management-ui, Property 8: Permission-gated archive action
// **Validates: Requirements 7.4**

import { describe, it, expect } from "vitest";
import fc from "fast-check";
import { canArchive } from "../agent-permissions";

describe("canArchive — Property 8: Permission-gated archive action", () => {
  it("returns true when user is the agent owner (regardless of admin status)", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1, maxLength: 36 }),
        fc.boolean(),
        (userId, isAdmin) => {
          // When agentOwnerId === currentUserId, canArchive is always true
          expect(canArchive(userId, userId, isAdmin)).toBe(true);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("returns true when user is a workspace admin/owner (regardless of ownership)", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1, maxLength: 36 }),
        fc.string({ minLength: 1, maxLength: 36 }),
        (agentOwnerId, currentUserId) => {
          // When isAdmin is true, canArchive is always true
          expect(canArchive(agentOwnerId, currentUserId, true)).toBe(true);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("returns false when user is neither the owner nor an admin", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1, maxLength: 36 }),
        fc.string({ minLength: 1, maxLength: 36 }).filter((s) => s.length > 0),
        (agentOwnerId, suffix) => {
          // Ensure currentUserId is different from agentOwnerId
          const currentUserId = agentOwnerId + suffix;
          expect(canArchive(agentOwnerId, currentUserId, false)).toBe(false);
        }
      ),
      { numRuns: 100 }
    );
  });

  it("archive visible iff user is owner OR workspace admin/owner (universal property)", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1, maxLength: 36 }),
        fc.string({ minLength: 1, maxLength: 36 }),
        fc.boolean(),
        (agentOwnerId, currentUserId, isAdmin) => {
          const result = canArchive(agentOwnerId, currentUserId, isAdmin);
          const expected = agentOwnerId === currentUserId || isAdmin;
          expect(result).toBe(expected);
        }
      ),
      { numRuns: 100 }
    );
  });
});
