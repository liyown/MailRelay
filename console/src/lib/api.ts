export class APIError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
  }
}

export type Session = { user: { id: string; name: string }; csrf: string };
export type Page<T> = { items: T[]; next_cursor?: string };
export type Execution = { id: number; message_id: string; command: string; handler: string; status: string; summary: string; error_kind?: string; sender?: string; started_at: string; duration_ms: number };
export type CommandItem = { name: string; description: string; handler: string; maturity: string; parameter_count: number };
export type Job = { id: number; command: string; status: string; attempts: number; max_attempts: number; available_at: string };
export type Reply = { id: number; message_id: string; recipient: string; status: string; attempts: number; max_attempts: number; available_at: string; created_at: string; error_kind?: string };
export type EventItem = { id: number; at: string; severity: string; phase: string; message_id?: string; command?: string; handler?: string; error_kind?: string; summary: string };
export type Dashboard = { range: string; execution_count: number; success_count: number; success_rate: number; p95_duration_ms: number; active_handlers: number; queue: { pending: number; running: number; dead: number }; replies: { pending: number; running: number; dead: number }; recent_executions: Execution[]; recent_events: EventItem[] };
export type SystemInfo = { started_at: string; uptime_seconds: number; command_count: number };

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(path, {
    ...init,
    credentials: "same-origin",
    headers: {
      ...(init.body ? { "Content-Type": "application/json" } : {}),
      ...init.headers,
    },
  });
  if (response.status === 204) return undefined as T;
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = body?.error;
    throw new APIError(response.status, error?.code ?? "request_failed", error?.message ?? "请求失败");
  }
  return body as T;
}

const query = (values: Record<string, string | number | undefined>) => {
  const search = new URLSearchParams();
  Object.entries(values).forEach(([key, value]) => value !== undefined && value !== "" && search.set(key, String(value)));
  const encoded = search.toString();
  return encoded ? `?${encoded}` : "";
};

export const api = {
  health: () => request<{ status: string }>("/api/v1/health"),
  session: () => request<Session>("/api/v1/session"),
  login: (password: string) => request<Session>("/api/v1/login", { method: "POST", body: JSON.stringify({ password }) }),
  logout: (csrf: string) => request<void>("/api/v1/logout", { method: "POST", headers: { "X-CSRF-Token": csrf } }),
  dashboard: (range = "24h") => request<Dashboard>(`/api/v1/dashboard${query({ range })}`),
  commands: () => request<Page<CommandItem>>("/api/v1/commands"),
  executions: (values: { limit?: number; status?: string; command?: string; cursor?: string } = {}) => request<Page<Execution>>(`/api/v1/executions${query(values)}`),
  jobs: (values: { limit?: number; status?: string; cursor?: string } = {}) => request<Page<Job>>(`/api/v1/jobs${query(values)}`),
  replies: (values: { limit?: number; status?: string; cursor?: string } = {}) => request<Page<Reply>>(`/api/v1/replies${query(values)}`),
  events: (values: { limit?: number; severity?: string; cursor?: string } = {}) => request<Page<EventItem>>(`/api/v1/events${query(values)}`),
  system: () => request<SystemInfo>("/api/v1/system"),
};
