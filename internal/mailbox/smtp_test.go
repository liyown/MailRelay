package mailbox

import (
	"github.com/liyown/MailRelay/internal/command"
	"strings"
	"testing"
)

func TestBuildReplyThreadsAndSanitizes(t *testing.T) {
	b, err := BuildReply("relay@example.com", "me@example.com", "<m1@example.com>", "push", command.Result{Status: "success", Summary: "done", Body: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"In-Reply-To: <m1@example.com>", "References: <m1@example.com>", "Subject: [MailRelay success] push", "hello"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %s", want, s)
		}
	}
}
