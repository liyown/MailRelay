export type Session = { user: { id: string; name: string }; csrf: string };
export type Page<T> = { items: T[]; next_cursor?: string };

export type Execution = {
  id: number;
  message_id: string;
  command: string;
  handler: string;
  status: string;
  summary: string;
  error_kind?: string;
  sender?: string;
  started_at: string;
  duration_ms: number;
};

export type CommandItem = {
  name: string;
  description: string;
  handler: string;
  maturity: string;
  parameter_count: number;
};

export type CommandActivity = {
  command: string;
  status: string;
  summary: string;
  error_kind?: string;
  started_at: string;
  duration_ms: number;
};

export type MailPreview = {
  accepted: boolean;
  stage: string;
  command?: string;
  handler?: string;
  parameters?: string[];
  error_kind?: string;
};

// Parameter and CommandDetail mirror internal/command.Command — the full,
// editable shape (with parameters and handler config), as opposed to the reduced
// read-only CommandItem projection.
export type Parameter = {
  description?: string;
  type?: string;
  required?: boolean;
  sensitive?: boolean;
  example?: unknown;
};

export type CommandDetail = {
  name: string;
  description?: string;
  handler: string;
  parameters?: Record<string, Parameter>;
  config?: Record<string, unknown>;
};

// ConfigDraft is the console-editable slice of configuration returned by
// GET /api/v1/config/draft and submitted whole to PUT /api/v1/config/draft.
export type ConfigDraft = {
  commands: CommandDetail[];
  http_hosts: string[];
  catalog_notify: string[];
  token: string;
  allow: string[];
};

export type Job = {
  id: number;
  command: string;
  status: string;
  attempts: number;
  max_attempts: number;
  available_at: string;
};

export type Reply = {
  id: number;
  message_id: string;
  recipient: string;
  status: string;
  attempts: number;
  max_attempts: number;
  available_at: string;
  created_at: string;
  error_kind?: string;
};

export type EventItem = {
  id: number;
  at: string;
  severity: string;
  phase: string;
  message_id?: string;
  command?: string;
  handler?: string;
  error_kind?: string;
  summary: string;
};

export type WorkCounts = { pending: number; running: number; dead: number };
export type SeriesPoint = { at: string; count: number; success: number };

export type Dashboard = {
  range: string;
  execution_count: number;
  success_count: number;
  success_rate: number;
  p95_duration_ms: number;
  active_handlers: number;
  queue: WorkCounts;
  replies: WorkCounts;
  series: SeriesPoint[];
  recent_executions: Execution[];
  recent_events: EventItem[];
};

export type SystemInfo = {
  started_at: string;
  uptime_seconds: number;
  command_count: number;
  version: string;
  commit: string;
  build_time: string;
  go_version: string;
  inbox_address: string;
};
