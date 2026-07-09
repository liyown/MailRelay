package handler

import (
	"errors"
	"regexp"
	"strings"

	"github.com/becomeopc/opc-mailrelay/internal/command"
)

var (
	logEmailRE  = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	logSecretRE = regexp.MustCompile(`(?i)\b(token|password|passwd|secret|authorization|bearer|api[-_]?key)\b\s*[:=]?\s*[^;,\s"']+`)
)

func safeLogText(v string, sensitiveValues []string) string {
	v = redactSnapshotText(v, sensitiveValues)
	v = strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(v)
	v = strings.Join(strings.Fields(v), " ")
	v = logEmailRE.ReplaceAllString(v, "[email]")
	v = logSecretRE.ReplaceAllString(v, "$1=[REDACTED]")
	if len(v) > snapshotBodyLimit {
		return v[:snapshotBodyLimit] + "...[truncated]"
	}
	return v
}

func safeErrorText(err error, sensitiveValues []string) string {
	if err == nil {
		return ""
	}
	var commandErr *command.Error
	if errors.As(err, &commandErr) {
		detail := commandErr.Message
		if commandErr.Err != nil {
			detail += ": " + commandErr.Err.Error()
		}
		return safeLogText(detail, sensitiveValues)
	}
	return safeLogText(err.Error(), sensitiveValues)
}

func safeStringSlice(values []string, sensitiveValues []string) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = safeLogText(value, sensitiveValues)
	}
	return out
}
