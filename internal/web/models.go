package web

import "time"

type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type ExecutionItem struct {
	ID         int64     `json:"id"`
	MessageID  string    `json:"message_id"`
	Command    string    `json:"command"`
	Handler    string    `json:"handler"`
	Status     string    `json:"status"`
	Summary    string    `json:"summary"`
	ErrorKind  string    `json:"error_kind,omitempty"`
	Sender     string    `json:"sender,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	DurationMS int64     `json:"duration_ms"`
}

type JobItem struct {
	ID          int64     `json:"id"`
	Command     string    `json:"command"`
	Status      string    `json:"status"`
	Attempts    int       `json:"attempts"`
	MaxAttempts int       `json:"max_attempts"`
	AvailableAt time.Time `json:"available_at"`
}

type ReplyItem struct {
	ID          int64     `json:"id"`
	MessageID   string    `json:"message_id"`
	Recipient   string    `json:"recipient"`
	Status      string    `json:"status"`
	Attempts    int       `json:"attempts"`
	MaxAttempts int       `json:"max_attempts"`
	AvailableAt time.Time `json:"available_at"`
	CreatedAt   time.Time `json:"created_at"`
	ErrorKind   string    `json:"error_kind,omitempty"`
}

type EventItem struct {
	ID        int64     `json:"id"`
	At        time.Time `json:"at"`
	Severity  string    `json:"severity"`
	Phase     string    `json:"phase"`
	MessageID string    `json:"message_id,omitempty"`
	Command   string    `json:"command,omitempty"`
	Handler   string    `json:"handler,omitempty"`
	ErrorKind string    `json:"error_kind,omitempty"`
	Summary   string    `json:"summary"`
}

type CommandItem struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Handler        string `json:"handler"`
	Maturity       string `json:"maturity"`
	ParameterCount int    `json:"parameter_count"`
}

type WorkCounts struct {
	Pending int `json:"pending"`
	Running int `json:"running"`
	Dead    int `json:"dead"`
}

type SeriesPoint struct {
	At      time.Time `json:"at"`
	Count   int       `json:"count"`
	Success int       `json:"success"`
}

type Dashboard struct {
	Range            string          `json:"range"`
	ExecutionCount   int             `json:"execution_count"`
	SuccessCount     int             `json:"success_count"`
	SuccessRate      float64         `json:"success_rate"`
	P95DurationMS    int64           `json:"p95_duration_ms"`
	ActiveHandlers   int             `json:"active_handlers"`
	Queue            WorkCounts      `json:"queue"`
	Replies          WorkCounts      `json:"replies"`
	Series           []SeriesPoint   `json:"series"`
	RecentExecutions []ExecutionItem `json:"recent_executions"`
	RecentEvents     []EventItem     `json:"recent_events"`
}

type SystemInfo struct {
	StartedAt    time.Time `json:"started_at"`
	UptimeSecond int64     `json:"uptime_seconds"`
	CommandCount int       `json:"command_count"`
	Version      string    `json:"version"`
	Commit       string    `json:"commit"`
	BuildTime    string    `json:"build_time"`
	GoVersion    string    `json:"go_version"`
	InboxAddress string    `json:"inbox_address"`
}
