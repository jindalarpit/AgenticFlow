import { describe, it, expect, beforeEach, vi } from "vitest";
import {
  saveTaskResultPanelState,
  loadTaskResultPanelState,
  clearTaskResultPanelState,
  type TaskResultPanelSessionData,
} from "../taskResultSession";

describe("taskResultSession", () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  describe("saveTaskResultPanelState", () => {
    it("persists state to sessionStorage under the correct key", () => {
      const state: TaskResultPanelSessionData = { taskId: "task-123", dismissed: false };
      saveTaskResultPanelState(state);
      const raw = sessionStorage.getItem("af_task_result_panel");
      expect(raw).not.toBeNull();
      expect(JSON.parse(raw!)).toEqual(state);
    });

    it("overwrites previous state", () => {
      saveTaskResultPanelState({ taskId: "task-1", dismissed: false });
      saveTaskResultPanelState({ taskId: "task-2", dismissed: true });
      const raw = sessionStorage.getItem("af_task_result_panel");
      expect(JSON.parse(raw!)).toEqual({ taskId: "task-2", dismissed: true });
    });
  });

  describe("loadTaskResultPanelState", () => {
    it("returns the stored state when valid", () => {
      const state: TaskResultPanelSessionData = { taskId: "task-abc", dismissed: true };
      sessionStorage.setItem("af_task_result_panel", JSON.stringify(state));
      expect(loadTaskResultPanelState()).toEqual(state);
    });

    it("returns null when key is missing", () => {
      expect(loadTaskResultPanelState()).toBeNull();
    });

    it("returns null for malformed JSON", () => {
      sessionStorage.setItem("af_task_result_panel", "not-json{{{");
      expect(loadTaskResultPanelState()).toBeNull();
    });

    it("returns null when taskId is not a string", () => {
      sessionStorage.setItem("af_task_result_panel", JSON.stringify({ taskId: 123, dismissed: false }));
      expect(loadTaskResultPanelState()).toBeNull();
    });

    it("returns null when dismissed is not a boolean", () => {
      sessionStorage.setItem("af_task_result_panel", JSON.stringify({ taskId: "t", dismissed: "yes" }));
      expect(loadTaskResultPanelState()).toBeNull();
    });

    it("returns null for null stored value", () => {
      sessionStorage.setItem("af_task_result_panel", "null");
      expect(loadTaskResultPanelState()).toBeNull();
    });
  });

  describe("clearTaskResultPanelState", () => {
    it("removes the key from sessionStorage", () => {
      sessionStorage.setItem("af_task_result_panel", JSON.stringify({ taskId: "t", dismissed: false }));
      clearTaskResultPanelState();
      expect(sessionStorage.getItem("af_task_result_panel")).toBeNull();
    });

    it("does not throw when key does not exist", () => {
      expect(() => clearTaskResultPanelState()).not.toThrow();
    });
  });

  describe("graceful degradation", () => {
    it("saveTaskResultPanelState does not throw when sessionStorage throws", () => {
      vi.spyOn(window, "sessionStorage", "get").mockImplementation(() => {
        throw new Error("SecurityError");
      });
      expect(() => saveTaskResultPanelState({ taskId: "t", dismissed: false })).not.toThrow();
      vi.restoreAllMocks();
    });

    it("loadTaskResultPanelState returns null when sessionStorage throws", () => {
      vi.spyOn(window, "sessionStorage", "get").mockImplementation(() => {
        throw new Error("SecurityError");
      });
      expect(loadTaskResultPanelState()).toBeNull();
      vi.restoreAllMocks();
    });

    it("clearTaskResultPanelState does not throw when sessionStorage throws", () => {
      vi.spyOn(window, "sessionStorage", "get").mockImplementation(() => {
        throw new Error("SecurityError");
      });
      expect(() => clearTaskResultPanelState()).not.toThrow();
      vi.restoreAllMocks();
    });
  });
});
