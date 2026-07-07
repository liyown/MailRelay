package security

import (
	"crypto/subtle"
	"github.com/liyown/MailRelay/internal/command"
	"strings"
)

func Authenticate(r command.Request, token string, allow []string, want string) error {
	ok := false
	for _, a := range allow {
		if strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(r.Sender)) {
			ok = true
			break
		}
	}
	if !ok {
		return &command.Error{Kind: "authentication", Message: "sender is not allowed"}
	}
	if len(token) != len(want) || subtle.ConstantTimeCompare([]byte(token), []byte(want)) != 1 {
		return &command.Error{Kind: "authentication", Message: "invalid token"}
	}
	return nil
}
func Redact(c command.Command, p map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range p {
		if c.Parameters[k].Sensitive {
			out[k] = "[REDACTED]"
		} else {
			out[k] = v
		}
	}
	return out
}
