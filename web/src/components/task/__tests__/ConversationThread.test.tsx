import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ConversationThread } from "../ConversationThread";
import type { PromptHistoryEntry } from "../ConversationThread";
import { apiFetch } from "../../../lib/api";
import { wsClient } from "../../../lib/ws";

// Mock the api module
vi.mock("../../../lib/api", () => ({
  apiFetch: vi.fn(),
}));

// Mock the ws module
vi.mock("../../../lib/ws", () => ({
  wsClient: {
    on: vi.fn(() => vi.fn()),
  },
}));

const mockedApiFetch = vi.mocked(apiFetch);
const mockedWsClient = vi.mocked(wsClient);

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

const mockHistory: PromptHistoryEntry[] = [
  {
    id: "entry-1",
    task_stage_id: "stage-1",
    task_id: "task-1",
    prompt_text: "Create a plan for the authentication module",
    output_text: "# Authentication Plan\n\nHere is the plan...",
    created_at: "2025-01-15T10:00:00Z",
  },
  {
    id: "entry-2",
    task_stage_id: "stage-1",
    task_id: "task-2",
    prompt_text: "Add OAuth support to the plan",
    output_text: "# Updated Plan\n\nAdded OAuth section...",
    created_at: "2025-01-15T10:05:00Z",
  },
];

describe("ConversationThread", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("loading state", () => {
    it("shows loading skeleton while fetching", () => {
      mockedApiFetch.mockReturnValue(new Promise(() => {})); // never resolves

      render(
        <ConversationThread taskId="task-1" stageName="plan" />,
        { wrapper: createWrapper() }
      );

      expect(
        screen.getByRole("status", { name: "Loading conversation" })
      ).toBeInTheDocument();
    });
  });

  describe("empty state", () => {
    it("shows empty message when no history exists", async () => {
      mockedApiFetch.mockResolvedValue([]);

      render(
        <ConversationThread taskId="task-1" stageName="plan" />,
        { wrapper: createWrapper() }
      );

      expect(
        await screen.findByText("No conversation history yet.")
      ).toBeInTheDocument();
    });
  });

  describe("conversation rendering", () => {
    it("renders user prompts and agent responses", async () => {
      mockedApiFetch.mockResolvedValue(mockHistory);

      render(
        <ConversationThread taskId="task-1" stageName="plan" />,
        { wrapper: createWrapper() }
      );

      // User prompts should be visible
      expect(
        await screen.findByText(
          "Create a plan for the authentication module"
        )
      ).toBeInTheDocument();
      expect(
        screen.getByText("Add OAuth support to the plan")
      ).toBeInTheDocument();

      // Agent responses rendered as markdown (headings)
      expect(
        screen.getByRole("heading", { name: "Authentication Plan" })
      ).toBeInTheDocument();
      expect(
        screen.getByRole("heading", { name: "Updated Plan" })
      ).toBeInTheDocument();
    });

    it("renders conversation as a log region", async () => {
      mockedApiFetch.mockResolvedValue(mockHistory);

      render(
        <ConversationThread taskId="task-1" stageName="plan" />,
        { wrapper: createWrapper() }
      );

      expect(
        await screen.findByRole("log", { name: "Conversation history" })
      ).toBeInTheDocument();
    });

    it("handles entries with null output_text", async () => {
      const historyWithNull: PromptHistoryEntry[] = [
        {
          id: "entry-3",
          task_stage_id: "stage-1",
          task_id: "task-3",
          prompt_text: "Still processing...",
          output_text: null,
          created_at: "2025-01-15T10:10:00Z",
        },
      ];

      mockedApiFetch.mockResolvedValue(historyWithNull);

      render(
        <ConversationThread taskId="task-1" stageName="plan" />,
        { wrapper: createWrapper() }
      );

      // User prompt should still render
      expect(
        await screen.findByText("Still processing...")
      ).toBeInTheDocument();
    });
  });

  describe("API call", () => {
    it("fetches history from the correct endpoint", async () => {
      mockedApiFetch.mockResolvedValue([]);

      render(
        <ConversationThread taskId="task-123" stageName="design" />,
        { wrapper: createWrapper() }
      );

      await screen.findByText("No conversation history yet.");

      expect(mockedApiFetch).toHaveBeenCalledWith(
        "/api/tasks/task-123/stages/design/history"
      );
    });

    it("does not fetch when taskId is empty", () => {
      mockedApiFetch.mockResolvedValue([]);

      render(
        <ConversationThread taskId="" stageName="plan" />,
        { wrapper: createWrapper() }
      );

      expect(mockedApiFetch).not.toHaveBeenCalled();
    });
  });

  describe("WebSocket subscription", () => {
    it("subscribes to task_completed events", () => {
      mockedApiFetch.mockResolvedValue([]);

      render(
        <ConversationThread taskId="task-1" stageName="plan" />,
        { wrapper: createWrapper() }
      );

      expect(mockedWsClient.on).toHaveBeenCalledWith(
        "task_completed",
        expect.any(Function)
      );
    });
  });
});
