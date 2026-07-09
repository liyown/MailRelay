package config

import (
	"bytes"
	"fmt"
	"os"

	"github.com/becomeopc/opc-mailrelay/internal/command"
	"gopkg.in/yaml.v3"
)

// Draft is the subset of configuration the web console is allowed to edit: the
// command catalog, the outbound host allowlist, and the catalog-change
// notification recipients. Values are carried UNRESOLVED — env ${VAR} tokens are
// preserved verbatim — so persisting a draft never bakes a resolved secret into
// the file.
type Draft struct {
	Commands      []command.Command `json:"commands"`
	HTTPHosts     []string          `json:"http_hosts"`
	CatalogNotify []string          `json:"catalog_notify"`
}

// ParseDraft extracts the editable sections from raw YAML WITHOUT env
// substitution, so ${VAR} references survive a load/edit/save round-trip. It is
// intentionally lenient about unknown keys — it only reads the three editable
// sections and ignores everything else.
func ParseDraft(raw []byte) (Draft, error) {
	var doc struct {
		Security struct {
			HTTPHosts []string `yaml:"http_hosts"`
		} `yaml:"security"`
		Runtime struct {
			CatalogNotify []string `yaml:"catalog_notify"`
		} `yaml:"runtime"`
		Commands []command.Command `yaml:"commands"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return Draft{}, err
	}
	return Draft{Commands: doc.Commands, HTTPHosts: doc.Security.HTTPHosts, CatalogNotify: doc.Runtime.CatalogNotify}, nil
}

// LoadDraft reads the editable sections from a config file on disk.
func LoadDraft(path string) (Draft, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Draft{}, err
	}
	return ParseDraft(raw)
}

// RenderDraft returns a copy of the original YAML document with ONLY the
// commands, security.http_hosts, and runtime.catalog_notify nodes replaced by
// the draft's values. Every other node — mail credentials, tokens, the session
// secret, ${VAR} references, and comments outside the edited sections — is left
// intact. This is why editing never round-trips the resolved Config (which
// would leak secrets); it is a surgical edit of three sub-trees only.
func RenderDraft(original []byte, d Draft) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(original, &doc); err != nil {
		return nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config root is not a mapping")
	}

	commandsNode, err := encodeNode(d.Commands)
	if err != nil {
		return nil, err
	}
	setChild(root, "commands", commandsNode)

	hostsNode, err := encodeNode(d.HTTPHosts)
	if err != nil {
		return nil, err
	}
	setChild(childMapping(root, "security"), "http_hosts", hostsNode)

	notifyNode, err := encodeNode(d.CatalogNotify)
	if err != nil {
		return nil, err
	}
	setChild(childMapping(root, "runtime"), "catalog_notify", notifyNode)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeNode(v any) (*yaml.Node, error) {
	var n yaml.Node
	if err := n.Encode(v); err != nil {
		return nil, err
	}
	return &n, nil
}

// childMapping returns the mapping value node for key, creating an empty mapping
// (and the key) when it is missing so nested edits always have a home.
func childMapping(m *yaml.Node, key string) *yaml.Node {
	if v := mappingValue(m, key); v != nil {
		if v.Kind != yaml.MappingNode {
			v.Kind = yaml.MappingNode
			v.Tag = "!!map"
			v.Value = ""
			v.Content = nil
		}
		return v
	}
	value := &yaml.Node{Kind: yaml.MappingNode}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
	return value
}

func mappingValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// setChild replaces the value node for key in-place (preserving the key node and
// its comments), or appends the key/value pair when the key is absent.
func setChild(m *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = value
			return
		}
	}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
}
