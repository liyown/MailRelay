package mail

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	mailstd "net/mail"
	"strings"
	"time"

	"github.com/becomeopc/opc-mailrelay/internal/command"
)

type Attachment struct {
	Name, ContentType string
	Size              int
}
type Message struct {
	Request     command.Request
	Token       string
	Attachments []Attachment
}

type Envelope struct {
	MessageID string
	Sender    string
	Name      string
	InReplyTo string
}

func parseErr(message string, err error) error {
	return &command.Error{Kind: "parse", Message: message, Err: err}
}

func Parse(r io.Reader) (Message, error) {
	m, err := mailstd.ReadMessage(r)
	if err != nil {
		return Message{}, parseErr("invalid message", err)
	}
	addr, err := mailstd.ParseAddress(m.Header.Get("From"))
	if err != nil {
		return Message{}, parseErr("invalid From", err)
	}
	subj, err := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	if err != nil {
		return Message{}, parseErr("invalid Subject", err)
	}
	fields := strings.Fields(strings.TrimSpace(subj))
	if len(fields) == 0 {
		return Message{}, parseErr("empty subject", nil)
	}
	name := strings.ToLower(fields[0])
	params := map[string]any{}
	if name == "help" && len(fields) > 1 {
		params["command"] = strings.ToLower(fields[1])
	}
	body, atts, ctype, err := readBody(m)
	if err != nil {
		return Message{}, parseErr("invalid body", err)
	}
	if strings.Contains(ctype, "application/json") {
		var x map[string]any
		if err = json.Unmarshal(body, &x); err != nil {
			return Message{}, parseErr("invalid JSON body", err)
		}
		for k, v := range x {
			params[k] = v
		}
	} else if looksLikeHTTPRequest(body) {
		// http_request commands intentionally use the whole mail body as an
		// RFC 7230-style request. Authentication for these mails must use the
		// X-MailRelay-Token header because the body is no longer key=value.
	} else {
		s := bufio.NewScanner(strings.NewReader(string(body)))
		for s.Scan() {
			line := strings.TrimSpace(s.Text())
			if line == "" {
				continue
			}
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				return Message{}, parseErr("invalid body line", nil)
			}
			params[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
		if err = s.Err(); err != nil {
			return Message{}, parseErr("invalid body", err)
		}
	}
	token, _ := params["_token"].(string)
	delete(params, "_token")
	if h := m.Header.Get("X-MailRelay-Token"); h != "" {
		token = h
	}
	id := strings.Trim(strings.TrimSpace(m.Header.Get("Message-ID")), "<>")
	if id == "" {
		sum := sha256.Sum256([]byte(strings.ToLower(addr.Address) + "\n" + subj + "\n" + string(body)))
		id = "sha256:" + hex.EncodeToString(sum[:])
	}
	received := time.Now().UTC()
	if d, err := m.Header.Date(); err == nil {
		received = d
	}
	return Message{Request: command.Request{MessageID: id, Sender: strings.ToLower(addr.Address), Name: name, Params: params, RawBody: string(body), Received: received, InReplyTo: m.Header.Get("Message-ID")}, Token: token, Attachments: atts}, nil
}

func looksLikeHTTPRequest(body []byte) bool {
	line, _, _ := strings.Cut(strings.TrimLeft(string(body), "\r\n\t "), "\n")
	line = strings.TrimSpace(line)
	parts := strings.Fields(line)
	if len(parts) != 3 || !strings.HasPrefix(parts[2], "HTTP/") {
		return false
	}
	switch parts[0] {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS":
		return true
	default:
		return false
	}
}

func ParseEnvelope(r io.Reader) (Envelope, error) {
	m, err := mailstd.ReadMessage(r)
	if err != nil {
		return Envelope{}, parseErr("invalid message", err)
	}
	addr, err := mailstd.ParseAddress(m.Header.Get("From"))
	if err != nil {
		return Envelope{}, parseErr("invalid From", err)
	}
	subj, err := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	if err != nil {
		return Envelope{}, parseErr("invalid Subject", err)
	}
	fields := strings.Fields(strings.TrimSpace(subj))
	name := ""
	if len(fields) > 0 {
		name = strings.ToLower(fields[0])
	}
	id := strings.Trim(strings.TrimSpace(m.Header.Get("Message-ID")), "<>")
	if id == "" {
		sum := sha256.Sum256([]byte(strings.ToLower(addr.Address) + "\n" + subj))
		id = "sha256:" + hex.EncodeToString(sum[:])
	}
	return Envelope{MessageID: id, Sender: strings.ToLower(addr.Address), Name: name, InReplyTo: m.Header.Get("Message-ID")}, nil
}
func readBody(m *mailstd.Message) ([]byte, []Attachment, string, error) {
	ct := m.Header.Get("Content-Type")
	mt, p, _ := mime.ParseMediaType(ct)
	if strings.HasPrefix(mt, "multipart/") {
		mr := multipart.NewReader(m.Body, p["boundary"])
		var body []byte
		var atts []Attachment
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, nil, "", err
			}
			b, err := io.ReadAll(io.LimitReader(part, 10<<20))
			if err != nil {
				return nil, nil, "", err
			}
			fn := part.FileName()
			pct := part.Header.Get("Content-Type")
			if fn != "" {
				atts = append(atts, Attachment{Name: fn, ContentType: pct, Size: len(b)})
			} else if body == nil {
				body = b
				ct = pct
			}
		}
		return body, atts, ct, nil
	}
	b, err := io.ReadAll(io.LimitReader(m.Body, 10<<20))
	return b, nil, ct, err
}
