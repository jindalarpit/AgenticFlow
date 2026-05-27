/**
 * Unit tests for editable tabs: InstructionsTab, EnvironmentTab, CustomArgsTab.
 *
 * Tests save/cancel flows, read-only mode, validation errors.
 * - Character limit warning for Instructions (Requirement 12.7)
 * - Duplicate key detection for Environment (Requirement 14.7)
 * - Space-splitting for CustomArgs (Requirement 15.6)
 *
 * Validates: Requirements 12.1, 12.6, 12.7, 14.4, 14.7, 15.6
 */

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { InstructionsTab } from "../InstructionsTab";
import { EnvironmentTab } from "../EnvironmentTab";
import { CustomArgsTab } from "../CustomArgsTab";
import type { Agent } from "../../../lib/agent-detail-types";

/* ─── Mock useToast ─── */

const mockShowToast = vi.fn();

vi.mock("../../Toast", () => ({
  useToast: () => ({ showToast: mockShowToast }),
}));

/* ─── Mock Agent Objects ─── */

function createMockAgent(overrides: Partial<Agent> = {}): Agent {
  return {
    id: "agent-1",
    name: "Test Agent",
    description: "A test agent",
    instructions: "You are a helpful assistant.",
    avatar_url: null,
    runtime_id: "runtime-1",
    runtime_name: "Local CLI",
    custom_env: {},
    custom_args: [],
    model: "claude-sonnet-4-20250514",
    visibility: "private",
    status: "idle",
    max_concurrent_tasks: 1,
    owner_id: "user-1",
    owner_name: "Test User",
    skills: [],
    mcp_config: null,
    created_at: "2025-01-01T00:00:00Z",
    updated_at: "2025-01-15T00:00:00Z",
    ...overrides,
  };
}

/* ─── Setup ─── */

beforeEach(() => {
  vi.clearAllMocks();
});

/* ═══════════════════════════════════════════════════════════════════════════
   InstructionsTab Tests
   ═══════════════════════════════════════════════════════════════════════════ */

describe("InstructionsTab", () => {
  const defaultProps = {
    agent: createMockAgent(),
    isOwner: true,
    onDirtyChange: vi.fn(),
    onSave: vi.fn().mockResolvedValue(undefined),
  };

  it("renders textarea with agent instructions", () => {
    render(<InstructionsTab {...defaultProps} />);
    const textarea = screen.getByLabelText("Agent instructions editor");
    expect(textarea).toBeInTheDocument();
    expect(textarea).toHaveValue("You are a helpful assistant.");
  });

  it('shows "Unsaved changes" when text is modified', () => {
    render(<InstructionsTab {...defaultProps} />);
    const textarea = screen.getByLabelText("Agent instructions editor");

    fireEvent.change(textarea, { target: { value: "Modified instructions" } });

    expect(screen.getByText("Unsaved changes")).toBeInTheDocument();
  });

  it("Save button disabled when not dirty", () => {
    render(<InstructionsTab {...defaultProps} />);
    const saveButton = screen.getByRole("button", { name: /save/i });
    expect(saveButton).toBeDisabled();
  });

  it("Save button enabled when dirty and under limit", () => {
    render(<InstructionsTab {...defaultProps} />);
    const textarea = screen.getByLabelText("Agent instructions editor");

    fireEvent.change(textarea, { target: { value: "New instructions" } });

    const saveButton = screen.getByRole("button", { name: /save/i });
    expect(saveButton).not.toBeDisabled();
  });

  it("character limit warning shown when > 50,000 chars", () => {
    render(<InstructionsTab {...defaultProps} />);
    const textarea = screen.getByLabelText("Agent instructions editor");

    const longText = "x".repeat(50_001);
    fireEvent.change(textarea, { target: { value: longText } });

    expect(screen.getByText(/character limit exceeded/i)).toBeInTheDocument();
  });

  it("Save button disabled when over limit", () => {
    render(<InstructionsTab {...defaultProps} />);
    const textarea = screen.getByLabelText("Agent instructions editor");

    const longText = "x".repeat(50_001);
    fireEvent.change(textarea, { target: { value: longText } });

    const saveButton = screen.getByRole("button", { name: /save/i });
    expect(saveButton).toBeDisabled();
  });

  it("read-only mode: textarea is readOnly, no Save button", () => {
    render(<InstructionsTab {...defaultProps} isOwner={false} />);
    const textarea = screen.getByLabelText("Agent instructions editor");
    expect(textarea).toHaveAttribute("readOnly");
    expect(screen.queryByRole("button", { name: /save/i })).not.toBeInTheDocument();
  });

  it("successful save: calls onSave, shows success toast", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(<InstructionsTab {...defaultProps} onSave={onSave} />);
    const textarea = screen.getByLabelText("Agent instructions editor");

    fireEvent.change(textarea, { target: { value: "Updated instructions" } });

    const saveButton = screen.getByRole("button", { name: /save/i });
    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith({ instructions: "Updated instructions" });
    });

    await waitFor(() => {
      expect(mockShowToast).toHaveBeenCalledWith(
        "Instructions saved successfully",
        "success"
      );
    });
  });
});

/* ═══════════════════════════════════════════════════════════════════════════
   EnvironmentTab Tests
   ═══════════════════════════════════════════════════════════════════════════ */

describe("EnvironmentTab", () => {
  const agentWithEnv = createMockAgent({
    custom_env: { API_KEY: "secret123", DB_HOST: "localhost" },
  });

  const defaultProps = {
    agent: agentWithEnv,
    isOwner: true,
    onDirtyChange: vi.fn(),
    onSave: vi.fn().mockResolvedValue(undefined),
  };

  it("renders existing env vars as key-value rows", () => {
    render(<EnvironmentTab {...defaultProps} />);
    // Check that key inputs contain the keys
    const keyInputs = screen.getAllByLabelText(/environment variable key/i);
    const keyValues = keyInputs.map((input) => (input as HTMLInputElement).value);
    expect(keyValues).toContain("API_KEY");
    expect(keyValues).toContain("DB_HOST");
  });

  it("values are masked by default", () => {
    render(<EnvironmentTab {...defaultProps} />);
    const valueInputs = screen.getAllByLabelText(/environment variable value \d+/i);
    // All value inputs should be type="password" by default
    valueInputs.forEach((input) => {
      expect(input).toHaveAttribute("type", "password");
    });
  });

  it("show/hide toggle works", () => {
    render(<EnvironmentTab {...defaultProps} />);
    // Find the first "Show" button
    const showButtons = screen.getAllByLabelText(/show value/i);
    expect(showButtons.length).toBeGreaterThan(0);

    fireEvent.click(showButtons[0]!);

    // After clicking, the button should now say "Hide value"
    expect(screen.getByLabelText("Hide value")).toBeInTheDocument();
    // The corresponding input should now be type="text"
    const valueInputs = screen.getAllByLabelText(/environment variable value/i);
    expect(valueInputs[0]).toHaveAttribute("type", "text");
  });

  it("Add button adds new row", () => {
    render(<EnvironmentTab {...defaultProps} />);
    const addButton = screen.getByLabelText("Add environment variable");
    const initialKeyInputs = screen.getAllByLabelText(/environment variable key/i);
    const initialCount = initialKeyInputs.length;

    fireEvent.click(addButton);

    const updatedKeyInputs = screen.getAllByLabelText(/environment variable key/i);
    expect(updatedKeyInputs.length).toBe(initialCount + 1);
  });

  it("Add button disabled at 20 entries", () => {
    const envWith20 = Object.fromEntries(
      Array.from({ length: 20 }, (_, i) => [`KEY_${i}`, `val_${i}`])
    );
    const agent20 = createMockAgent({ custom_env: envWith20 });
    render(<EnvironmentTab {...defaultProps} agent={agent20} />);

    const addButton = screen.getByLabelText("Add environment variable");
    expect(addButton).toBeDisabled();
  });

  it("Delete button removes row", () => {
    render(<EnvironmentTab {...defaultProps} />);
    const deleteButtons = screen.getAllByLabelText(/remove environment variable/i);
    const initialKeyInputs = screen.getAllByLabelText(/environment variable key/i);
    const initialCount = initialKeyInputs.length;

    fireEvent.click(deleteButtons[0]!);

    const updatedKeyInputs = screen.getAllByLabelText(/environment variable key/i);
    expect(updatedKeyInputs.length).toBe(initialCount - 1);
  });

  it("duplicate key detection shows error toast on save", async () => {
    render(<EnvironmentTab {...defaultProps} />);

    // Add a new row and set its key to duplicate an existing one
    const addButton = screen.getByLabelText("Add environment variable");
    fireEvent.click(addButton);

    const keyInputs = screen.getAllByLabelText(/environment variable key/i);
    const lastKeyInput = keyInputs[keyInputs.length - 1]!;
    fireEvent.change(lastKeyInput, { target: { value: "API_KEY" } });

    // Click save
    const saveButton = screen.getByRole("button", { name: /save/i });
    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(mockShowToast).toHaveBeenCalledWith(
        expect.stringContaining("Duplicate"),
        "error"
      );
    });

    // onSave should NOT have been called
    expect(defaultProps.onSave).not.toHaveBeenCalled();
  });

  it("read-only mode: no Add/Delete/Save controls", () => {
    render(<EnvironmentTab {...defaultProps} isOwner={false} />);
    expect(screen.queryByLabelText("Add environment variable")).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/remove environment variable/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /save/i })).not.toBeInTheDocument();
  });
});

/* ═══════════════════════════════════════════════════════════════════════════
   CustomArgsTab Tests
   ═══════════════════════════════════════════════════════════════════════════ */

describe("CustomArgsTab", () => {
  const agentWithArgs = createMockAgent({
    custom_args: ["--verbose", "--timeout 30"],
  });

  const defaultProps = {
    agent: agentWithArgs,
    isOwner: true,
    onDirtyChange: vi.fn(),
    onSave: vi.fn().mockResolvedValue(undefined),
  };

  it("renders existing args as input rows", () => {
    render(<CustomArgsTab {...defaultProps} />);
    const input1 = screen.getByLabelText("Argument 1");
    const input2 = screen.getByLabelText("Argument 2");
    expect(input1).toHaveValue("--verbose");
    expect(input2).toHaveValue("--timeout 30");
  });

  it("Add button adds new row", () => {
    render(<CustomArgsTab {...defaultProps} />);
    const addButton = screen.getByLabelText("Add argument");
    const initialInputs = screen.getAllByRole("textbox");
    const initialCount = initialInputs.length;

    fireEvent.click(addButton);

    const updatedInputs = screen.getAllByRole("textbox");
    expect(updatedInputs.length).toBe(initialCount + 1);
  });

  it("Delete button removes row", () => {
    render(<CustomArgsTab {...defaultProps} />);
    const deleteButtons = screen.getAllByLabelText(/delete argument/i);
    const initialInputs = screen.getAllByRole("textbox");
    const initialCount = initialInputs.length;

    fireEvent.click(deleteButtons[0]!);

    const updatedInputs = screen.getAllByRole("textbox");
    expect(updatedInputs.length).toBe(initialCount - 1);
  });

  it("Save splits space-separated tokens", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    const agent = createMockAgent({ custom_args: ["--flag value"] });
    render(<CustomArgsTab agent={agent} isOwner={true} onDirtyChange={vi.fn()} onSave={onSave} />);

    // Modify the row to trigger dirty state
    const input = screen.getByLabelText("Argument 1");
    fireEvent.change(input, { target: { value: "--flag value --extra" } });

    const saveButton = screen.getByLabelText("Save custom arguments");
    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith({
        custom_args: ["--flag", "value", "--extra"],
      });
    });
  });

  it("read-only mode: inputs disabled, no controls", () => {
    render(<CustomArgsTab {...defaultProps} isOwner={false} />);
    // In read-only mode, no Add/Delete/Save buttons
    expect(screen.queryByLabelText("Add argument")).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/delete argument/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Save custom arguments")).not.toBeInTheDocument();
  });
});
