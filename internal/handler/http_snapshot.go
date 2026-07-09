package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/becomeopc/opc-mailrelay/internal/command"
)

var sensitiveHeaderRE = regexp.MustCompile(`(?i)(authorization|cookie|token|secret|password|passwd|api[-_]?key|signature)`)

const snapshotBodyLimit = 4096

func logHTTPSnapshot(message, label string, x command.Context, request string, response ...string) {
	args := []any{"command", x.Command.Name, "message_id", x.Request.MessageID, "request", request}
	if len(response) > 0 && response[0] != "" {
		args = append(args, "response", response[0])
	}
	slog.Info(message, args...)
	logHTTPTranscript(label, x.Command.Name, x.Request.MessageID, firstNonEmpty(response, request))
}

func firstNonEmpty(values []string, fallback string) string {
	if len(values) > 0 && values[0] != "" {
		return values[0]
	}
	return fallback
}

func logHTTPTranscript(label, commandName, messageID, transcript string) {
	slog.Info(fmt.Sprintf("\n--- %s command=%s message_id=%s ---\n%s\n--- END %s ---", label, commandName, messageID, transcript, label))
}

func commandSensitiveValues(c command.Command, params map[string]any) []string {
	values := make([]string, 0)
	for name, p := range c.Parameters {
		if !p.Sensitive {
			continue
		}
		if v, ok := params[name]; ok {
			if s := fmt.Sprint(v); s != "" {
				values = append(values, s)
			}
		}
	}
	return values
}

func httpRequestSnapshot(req *http.Request, body string, sensitiveValues []string) map[string]any {
	headers := redactHeader(req.Header, sensitiveValues)
	if req.Host != "" {
		headers["Host"] = []string{snapshotString(req.Host, sensitiveValues)}
	} else if req.URL != nil {
		headers["Host"] = []string{snapshotString(req.URL.Host, sensitiveValues)}
	}
	return map[string]any{
		"method":     req.Method,
		"url":        redactURL(req.URL, sensitiveValues),
		"headers":    headers,
		"body":       snapshotString(body, sensitiveValues),
		"transcript": requestTranscript(req, headers, body, sensitiveValues),
	}
}

func httpResponseSnapshot(resp *http.Response, body []byte, sensitiveValues []string) map[string]any {
	return map[string]any{
		"status":     resp.Status,
		"code":       resp.StatusCode,
		"headers":    redactHeader(resp.Header, sensitiveValues),
		"body":       snapshotString(string(body), sensitiveValues),
		"transcript": responseTranscript(resp, body, sensitiveValues),
	}
}

func requestTranscript(req *http.Request, headers map[string][]string, body string, sensitiveValues []string) string {
	target := "/"
	if req.URL != nil {
		target = req.URL.RequestURI()
		if target == "" {
			target = "/"
		}
		target = snapshotString(target, sensitiveValues)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s HTTP/1.1\n", req.Method, target)
	writeHeaderLines(&b, headers)
	if body != "" {
		b.WriteString("\n")
		b.WriteString(snapshotString(body, sensitiveValues))
	}
	return b.String()
}

func responseTranscript(resp *http.Response, body []byte, sensitiveValues []string) string {
	status := resp.Status
	if status == "" {
		status = fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "HTTP/1.1 %s\n", status)
	writeHeaderLines(&b, redactHeader(resp.Header, sensitiveValues))
	if len(body) > 0 {
		b.WriteString("\n")
		b.WriteString(snapshotString(string(body), sensitiveValues))
	}
	return b.String()
}

func writeHeaderLines(b *strings.Builder, headers map[string][]string) {
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, value := range headers[key] {
			fmt.Fprintf(b, "%s: %s\n", key, value)
		}
	}
}

func redactHeader(header http.Header, sensitiveValues []string) map[string][]string {
	out := make(map[string][]string, len(header))
	for k, values := range header {
		copied := make([]string, len(values))
		for i, v := range values {
			if sensitiveHeaderRE.MatchString(k) {
				copied[i] = "[REDACTED]"
			} else {
				copied[i] = snapshotString(v, sensitiveValues)
			}
		}
		out[k] = copied
	}
	return out
}

func redactURL(u *url.URL, sensitiveValues []string) string {
	if u == nil {
		return ""
	}
	copied := *u
	if copied.User != nil {
		username := copied.User.Username()
		if _, hasPassword := copied.User.Password(); hasPassword {
			copied.User = url.UserPassword(username, "[REDACTED]")
		}
	}
	return snapshotString(copied.String(), sensitiveValues)
}

func snapshotString(v string, sensitiveValues []string) string {
	v = redactSnapshotText(v, sensitiveValues)
	if len(v) > snapshotBodyLimit {
		return v[:snapshotBodyLimit] + "...[truncated]"
	}
	return v
}

func redactSnapshotText(v string, sensitiveValues []string) string {
	for _, sensitive := range sensitiveValues {
		if sensitive == "" {
			continue
		}
		v = strings.ReplaceAll(v, sensitive, "[REDACTED]")
	}
	return v
}
