package handler

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/security"
)

type HTTPRequest struct {
	client *http.Client
	policy security.NetworkPolicy
}

func NewHTTPRequest(c *http.Client, p security.NetworkPolicy) *HTTPRequest {
	if c == nil {
		c = p.HTTPClient(30 * time.Second)
	}
	return &HTTPRequest{c, p}
}

func (h *HTTPRequest) Name() string { return "http_request" }

func (h *HTTPRequest) Execute(ctx context.Context, x command.Context) (command.Result, error) {
	req, body, err := h.buildRequest(ctx, x)
	if err != nil {
		return command.Result{}, err
	}
	sensitiveValues := commandSensitiveValues(x.Command, x.Request.Params)
	requestSnapshot := httpRequestSnapshot(req, string(body), sensitiveValues)
	logHTTPSnapshot("http_request snapshot", "HTTP_REQUEST REQUEST", x, requestSnapshot["transcript"].(string))

	resp, err := h.client.Do(req)
	if err != nil {
		slog.Warn("http_request failed", "command", x.Command.Name, "message_id", x.Request.MessageID, "request", requestSnapshot["transcript"], "error", redactSnapshotText(err.Error(), sensitiveValues))
		return command.Result{}, &command.Error{Kind: "dependency", Message: "HTTP request failed", Err: err}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return command.Result{}, err
	}
	responseSnapshot := httpResponseSnapshot(resp, respBody, sensitiveValues)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logHTTPSnapshot("http_request response snapshot", "HTTP_REQUEST RESPONSE", x, requestSnapshot["transcript"].(string), responseSnapshot["transcript"].(string))
		return command.Result{}, &command.Error{Kind: "dependency", Message: fmt.Sprintf("HTTP status %d: %s", resp.StatusCode, snapshotString(string(respBody), sensitiveValues))}
	}
	logHTTPSnapshot("http_request response snapshot", "HTTP_REQUEST RESPONSE", x, requestSnapshot["transcript"].(string), responseSnapshot["transcript"].(string))
	return command.Result{Status: "success", Summary: fmt.Sprintf("HTTP %d", resp.StatusCode), Body: string(respBody)}, nil
}

func (h *HTTPRequest) buildRequest(ctx context.Context, x command.Context) (*http.Request, []byte, error) {
	raw := strings.TrimLeft(x.Request.RawBody, "\r\n\t ")
	if raw == "" {
		return nil, nil, &command.Error{Kind: "invalid_parameters", Message: "mail body must contain an HTTP request"}
	}
	inReq, err := http.ReadRequest(bufio.NewReader(strings.NewReader(raw)))
	if err != nil {
		return nil, nil, &command.Error{Kind: "invalid_parameters", Message: "invalid HTTP request body", Err: err}
	}
	defer inReq.Body.Close()

	body, err := io.ReadAll(io.LimitReader(inReq.Body, 1<<20))
	if err != nil {
		return nil, nil, err
	}
	u, err := resolveForwardURL(x.Command.Config["base_url"], inReq)
	if err != nil {
		return nil, nil, err
	}
	if err = h.policy.Check(ctx, u); err != nil {
		return nil, nil, &command.Error{Kind: "policy", Message: "destination denied", Err: err}
	}

	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}
	outReq, err := http.NewRequestWithContext(ctx, inReq.Method, u.String(), bodyReader)
	if err != nil {
		return nil, nil, err
	}
	copyForwardHeaders(outReq.Header, inReq.Header)
	outReq.Host = u.Host
	return outReq, body, nil
}

func resolveForwardURL(rawBase any, req *http.Request) (*url.URL, error) {
	if req.URL != nil && req.URL.IsAbs() {
		u := *req.URL
		return &u, nil
	}
	base, _ := rawBase.(string)
	if base == "" {
		return nil, &command.Error{Kind: "policy", Message: "http_request requires config.base_url for origin-form requests"}
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	if req.Host != "" {
		u.Host = req.Host
	}
	if u.Host == "" {
		return nil, &command.Error{Kind: "policy", Message: "HTTP request target has no host"}
	}
	if req.URL != nil {
		u.Path = req.URL.Path
		u.RawQuery = req.URL.RawQuery
	}
	return u, nil
}

func copyForwardHeaders(dst, src http.Header) {
	for k, values := range src {
		if skipForwardHeader(k) {
			continue
		}
		for _, value := range values {
			dst.Add(k, value)
		}
	}
}

func skipForwardHeader(key string) bool {
	switch strings.ToLower(key) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade", "host":
		return true
	default:
		return false
	}
}
