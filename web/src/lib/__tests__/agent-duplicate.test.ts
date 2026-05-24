import { describe, it, expect } from "vitest";
import { buildDuplicatePayload, type AgentListItem } from "../agent-duplicate";

describe("buildDuplicatePayload", () => {
  const fullAgent: AgentListItem = {
    name: "Nexus",
    description: "Your local AI coding agent",
    instructions: "You are a helpful coding agent",
    runtime_id: "runtime-123",
    model: "claude-sonnet-4-20250514",
    visibility: "shared",
    avatar_url: "https://example.com/avatar.png",
    custom_env: { GITHUB_TOKEN: "abc123" },
    custom_args: ["--dangerously-skip-permissions"],
    max_concurrent_tasks: 3,
  };

  it("appends ' copy' to the name", () => {
    const payload = buildDuplicatePayload(fullAgent);
    expect(payload.name).toBe("Nexus copy");
  });

  it("copies description directly", () => {
    const payload = buildDuplicatePayload(fullAgent);
    expect(payload.description).toBe("Your local AI coding agent");
  });

  it("copies runtime_id directly", () => {
    const payload = buildDuplicatePayload(fullAgent);
    expect(payload.runtime_id).toBe("runtime-123");
  });

  it("copies visibility directly", () => {
    const payload = buildDuplicatePayload(fullAgent);
    expect(payload.visibility).toBe("shared");
  });

  it("includes instructions when non-empty", () => {
    const payload = buildDuplicatePayload(fullAgent);
    expect(payload.instructions).toBe("You are a helpful coding agent");
  });

  it("includes model when non-empty", () => {
    const payload = buildDuplicatePayload(fullAgent);
    expect(payload.model).toBe("claude-sonnet-4-20250514");
  });

  it("includes avatar_url when non-empty", () => {
    const payload = buildDuplicatePayload(fullAgent);
    expect(payload.avatar_url).toBe("https://example.com/avatar.png");
  });

  it("includes custom_env when non-empty object", () => {
    const payload = buildDuplicatePayload(fullAgent);
    expect(payload.custom_env).toEqual({ GITHUB_TOKEN: "abc123" });
  });

  it("includes custom_args when non-empty array", () => {
    const payload = buildDuplicatePayload(fullAgent);
    expect(payload.custom_args).toEqual(["--dangerously-skip-permissions"]);
  });

  it("includes max_concurrent_tasks when positive", () => {
    const payload = buildDuplicatePayload(fullAgent);
    expect(payload.max_concurrent_tasks).toBe(3);
  });

  it("omits optional fields when they have no meaningful value", () => {
    const minimalAgent: AgentListItem = {
      name: "Agent",
      description: "",
      instructions: "",
      runtime_id: "rt-1",
      model: "",
      visibility: "private",
      avatar_url: null,
      custom_env: {},
      custom_args: [],
      max_concurrent_tasks: 0,
    };

    const payload = buildDuplicatePayload(minimalAgent);

    expect(payload.name).toBe("Agent copy");
    expect(payload.description).toBe("");
    expect(payload.runtime_id).toBe("rt-1");
    expect(payload.visibility).toBe("private");
    expect(payload.instructions).toBeUndefined();
    expect(payload.model).toBeUndefined();
    expect(payload.avatar_url).toBeUndefined();
    expect(payload.custom_env).toBeUndefined();
    expect(payload.custom_args).toBeUndefined();
    expect(payload.max_concurrent_tasks).toBeUndefined();
  });
});
