package web

type ErrorEnvelope struct {
	Error APIError `json:"error"`
}

type APIError struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

// DraftError marks a configuration-edit failure whose message is safe to return
// to the authenticated admin: it describes the admin's own submitted config
// (unknown handler, duplicate command, un-allowlisted host, …), never a mail
// provider error or a secret. The editor handler maps it to HTTP 422 so the UI
// can show what to fix. Plain (non-DraftError) failures stay generic 500s.
type DraftError struct {
	Message string
	Fields  map[string]string
}

func (e *DraftError) Error() string { return e.Message }
