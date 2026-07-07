package mailbox

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/liyown/MailRelay/internal/command"
	"github.com/liyown/MailRelay/internal/config"
	"net"
	"net/smtp"
	"strings"
	"time"
)

type Sender interface {
	Send(context.Context, string, []byte) error
	Notify(context.Context, []string, string, string) error
}
type SMTPSender struct {
	cfg       config.MailEndpoint
	tlsConfig *tls.Config
}

func NewSMTP(c config.MailEndpoint) *SMTPSender { return &SMTPSender{cfg: c} }
func BuildReply(from, to, inReplyTo, name string, res command.Result) ([]byte, error) {
	for _, v := range []string{from, to, inReplyTo, name, res.Status} {
		if strings.ContainsAny(v, "\r\n") {
			return nil, fmt.Errorf("invalid header")
		}
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: [MailRelay %s] %s\r\nDate: %s\r\nMessage-ID: <%d@mailrelay.local>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n", from, to, res.Status, name, time.Now().UTC().Format(time.RFC1123Z), time.Now().UnixNano())
	if inReplyTo != "" {
		fmt.Fprintf(&b, "In-Reply-To: %s\r\nReferences: %s\r\n", inReplyTo, inReplyTo)
	}
	b.WriteString("\r\n")
	if res.Summary != "" {
		b.WriteString(res.Summary + "\n\n")
	}
	b.WriteString(res.Body)
	return b.Bytes(), nil
}
func (s *SMTPSender) Send(ctx context.Context, to string, msg []byte) error {
	return s.send(ctx, []string{to}, msg)
}
func (s *SMTPSender) Notify(ctx context.Context, to []string, subject, body string) error {
	res := command.Result{Status: "catalog", Summary: subject, Body: body}
	msg, err := BuildReply(s.cfg.From, strings.Join(to, ", "), "", "Catalog", res)
	if err != nil {
		return err
	}
	return s.send(ctx, to, msg)
}
func (s *SMTPSender) send(ctx context.Context, to []string, msg []byte) error {
	host, _, err := net.SplitHostPort(s.cfg.Address)
	if err != nil {
		return err
	}
	d := &net.Dialer{}
	tlsCfg := s.tlsConfig
	if tlsCfg == nil {
		tlsCfg = &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	} else {
		tlsCfg = tlsCfg.Clone()
		if tlsCfg.ServerName == "" {
			tlsCfg.ServerName = host
		}
		if tlsCfg.MinVersion == 0 {
			tlsCfg.MinVersion = tls.VersionTLS12
		}
	}
	conn, err := tls.DialWithDialer(d, "tcp", s.cfg.Address, tlsCfg)
	if err != nil {
		return err
	}
	defer conn.Close()
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer c.Close()
	if s.cfg.Username != "" {
		if err = c.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, host)); err != nil {
			return err
		}
	}
	if err = c.Mail(s.cfg.From); err != nil {
		return err
	}
	for _, x := range to {
		if err = c.Rcpt(x); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(msg); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
