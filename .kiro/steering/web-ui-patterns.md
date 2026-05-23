---
inclusion: fileMatch
fileMatchPattern: "**/web/**"
---

# Web UI Implementation Patterns

## Tech Stack

- **Vite** — Build tool
- **React 18+** — UI framework
- **TypeScript** — Type safety
- **Tailwind CSS** — Styling (utility-first)
- **@tanstack/react-query** — Server state management
- **react-router-dom** — Client-side routing
- **Native WebSocket** — Real-time updates (no socket.io)

## Pages

| Route | Page | Description |
|-------|------|-------------|
| `/login` | Login | OAuth buttons + email/password form |
| `/` | Dashboard | Daemons, agents, active tasks |
| `/tasks/:id` | Task Detail | Streaming output, status, metadata |
| `/tasks` | Task History | Paginated list with filters |
| `/agents` | Agents | List + create/edit agents |
| `/agents/new` | Create Agent | Agent creation form |
| `/agents/:id` | Edit Agent | Agent edit form |

## Dashboard Layout

The dashboard shows three sections (similar to multica's workspace view):

1. **Connected Daemons** — Card per daemon showing: device name, status (online/offline), detected runtimes list, last heartbeat time
2. **Agents** — Card per agent showing: name, bound runtime, status (idle/working/offline), model
3. **Task Queue** — Table of recent/active tasks: status badge, agent name, prompt preview, duration, created time

## Task Detail Page (Real-time Streaming)

This is the most important UI page. It must show:

1. **Header**: Task status badge, agent name, created time, duration
2. **Prompt**: The original task prompt (collapsible)
3. **Output Stream**: Terminal-like display that updates in real-time via WebSocket
   - Monospace font
   - Auto-scroll to bottom
   - Distinguish stdout (white) vs stderr (red/orange)
   - Show timestamps per chunk
4. **Actions**: Cancel button (if running)

## WebSocket Integration

```typescript
// lib/websocket.ts
class WSClient {
    private ws: WebSocket | null = null;
    private reconnectInterval = 5000;
    
    connect(token: string) {
        this.ws = new WebSocket(`${WS_URL}/ws?token=${token}`);
        this.ws.onmessage = (event) => {
            const msg = JSON.parse(event.data);
            this.handleEvent(msg.type, msg.payload);
        };
        this.ws.onclose = () => {
            setTimeout(() => this.connect(token), this.reconnectInterval);
        };
    }
    
    private handleEvent(type: string, payload: any) {
        // Invalidate React Query caches based on event type
        switch (type) {
            case 'task_created':
            case 'task_completed':
            case 'task_failed':
                queryClient.invalidateQueries(['tasks']);
                break;
            case 'task_output':
                // Append to task output stream (not full invalidation)
                break;
            case 'daemon_connected':
            case 'daemon_disconnected':
                queryClient.invalidateQueries(['daemons']);
                queryClient.invalidateQueries(['agents']);
                break;
        }
    }
}
```

## Agent Form Fields

The agent create/edit form must include (matching multica's agent UI):

```tsx
<form>
    <Input label="Name" required maxLength={64} />
    <Textarea label="Description" maxLength={255} />
    <Textarea label="Instructions" placeholder="System prompt for the agent..." />
    <Select label="Runtime" options={availableRuntimes} />
    <Input label="Model" placeholder="e.g., claude-sonnet-4-20250514" />
    <KeyValueEditor label="Environment Variables" maxPairs={20} />
    <ArrayEditor label="Custom Arguments" />
    <NumberInput label="Max Concurrent Tasks" min={1} max={20} default={1} />
</form>
```

## API Client Pattern

```typescript
// lib/api.ts
const api = {
    headers() {
        return {
            'Authorization': `Bearer ${getToken()}`,
            'Content-Type': 'application/json',
        };
    },
    
    async get<T>(path: string): Promise<T> {
        const res = await fetch(`${API_URL}${path}`, { headers: this.headers() });
        if (!res.ok) throw new ApiError(res);
        return res.json();
    },
    
    async post<T>(path: string, body: any): Promise<T> {
        const res = await fetch(`${API_URL}${path}`, {
            method: 'POST',
            headers: this.headers(),
            body: JSON.stringify(body),
        });
        if (!res.ok) throw new ApiError(res);
        return res.json();
    },
};
```

## React Query Hooks

```typescript
// hooks/useTasks.ts
export function useTasks() {
    return useQuery(['tasks'], () => api.get('/api/tasks'));
}

export function useTask(id: string) {
    return useQuery(['tasks', id], () => api.get(`/api/tasks/${id}`));
}

export function useTaskMessages(id: string) {
    return useQuery(['tasks', id, 'messages'], () => api.get(`/api/tasks/${id}/messages`), {
        refetchInterval: false, // WebSocket handles updates
    });
}

export function useCreateTask() {
    return useMutation((data) => api.post('/api/tasks', data), {
        onSuccess: () => queryClient.invalidateQueries(['tasks']),
    });
}
```

## Connection Status Indicator

Always show a small indicator in the header/footer:
- 🟢 Connected (WebSocket active)
- 🟡 Reconnecting... (WebSocket disconnected, retrying)
- 🔴 Disconnected (after multiple retries)
