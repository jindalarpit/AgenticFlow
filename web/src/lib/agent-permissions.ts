/**
 * Pure permission-checking functions for agent row-level actions.
 */

/**
 * Determines whether the current user can archive a given agent.
 *
 * The archive action is visible if and only if:
 * - The current user is the agent's owner (agentOwnerId === currentUserId), OR
 * - The current user is a workspace admin/owner (isAdmin === true)
 *
 * Property 8: Permission-gated archive action
 * Validates: Requirements 7.4
 */
export function canArchive(
  agentOwnerId: string,
  currentUserId: string,
  isAdmin: boolean
): boolean {
  return agentOwnerId === currentUserId || isAdmin;
}
