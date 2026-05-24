/**
 * Validation logic for the Create Agent Dialog.
 *
 * The "Create" button is disabled when the agent name (after trimming) is empty
 * OR when no runtime has been selected.
 */

/**
 * Returns true (disabled) iff name.trim() === "" OR selectedRuntimeId === "".
 */
export function isCreateDisabled(name: string, selectedRuntimeId: string): boolean {
  return name.trim() === "" || selectedRuntimeId === "";
}
