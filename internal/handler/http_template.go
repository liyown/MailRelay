package handler

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/becomeopc/opc-mailrelay/internal/command"
)

var templateRE = regexp.MustCompile(`\{\{([^{}]+)\}\}`)

func expand(s string, p map[string]any) string {
	for k, v := range p {
		s = strings.ReplaceAll(s, "{{"+k+"}}", fmt.Sprint(v))
	}
	return s
}

func parseTemplatedURL(raw string, p map[string]any) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if err = prepareURL(u, p); err != nil {
		return nil, err
	}
	return u, nil
}

func prepareURL(u *url.URL, p map[string]any) error {
	if strings.Contains(u.Scheme, "{{") || strings.Contains(u.Host, "{{") || strings.Contains(u.User.String(), "{{") {
		return &command.Error{Kind: "policy", Message: "URL scheme, credentials, and host must be static"}
	}
	if strings.Contains(u.RawQuery, "{{") {
		return &command.Error{Kind: "policy", Message: "URL query templates must use config.query"}
	}
	if strings.Contains(u.Path, "{{") {
		path, rawPath := expandPath(u.Path, p)
		u.Path = path
		u.RawPath = rawPath
	}
	return nil
}

func expandPath(path string, p map[string]any) (string, string) {
	var decoded strings.Builder
	var raw strings.Builder
	last := 0
	for _, loc := range templateRE.FindAllStringSubmatchIndex(path, -1) {
		static := path[last:loc[0]]
		decoded.WriteString(static)
		raw.WriteString(escapePathStatic(static))
		key := path[loc[2]:loc[3]]
		if v, ok := p[key]; ok {
			value := fmt.Sprint(v)
			decoded.WriteString(value)
			raw.WriteString(url.PathEscape(value))
		} else {
			value := path[loc[0]:loc[1]]
			decoded.WriteString(value)
			raw.WriteString(url.PathEscape(value))
		}
		last = loc[1]
	}
	tail := path[last:]
	decoded.WriteString(tail)
	raw.WriteString(escapePathStatic(tail))
	return decoded.String(), raw.String()
}

func escapePathStatic(s string) string {
	parts := strings.Split(s, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func applyQuery(u *url.URL, raw any, p map[string]any) {
	q, ok := raw.(map[string]any)
	if !ok {
		return
	}
	values := u.Query()
	for k, v := range q {
		if k == "" {
			continue
		}
		values.Set(k, expand(fmt.Sprint(v), p))
	}
	u.RawQuery = values.Encode()
}
