package yaml

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// TrimMultiline walks a YAML AST and removes trailing
// spaces/tabs from each line of literal-style multiline scalar nodes.
// It returns the number of scalar nodes modified.
func TrimMultiline(root *yaml.Node) int {
	if root == nil {
		return 0
	}
	return trimMultilineNode(root)
}

func trimMultilineNode(n *yaml.Node) int {
	modified := 0

	if n.Kind == yaml.ScalarNode && (n.Style == yaml.LiteralStyle || n.Style == yaml.FoldedStyle) && strings.Contains(n.Value, "\n") {
		trimmed := trimMultilineString(n.Value)
		if trimmed != n.Value {
			n.Value = trimmed
			modified++
		}
	}

	if n.Kind == yaml.MappingNode {
		// Mapping content is [key0, value0, key1, value1, ...].
		// Only process values; leave keys untouched.
		for i := 1; i < len(n.Content); i += 2 {
			modified += trimMultilineNode(n.Content[i])
		}
	} else {
		for i := range n.Content {
			modified += trimMultilineNode(n.Content[i])
		}
	}

	return modified
}

func trimMultilineString(s string) string {
	if s == "" {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))

	lineStart := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '\n' {
			continue
		}

		line := s[lineStart:i]
		b.WriteString(strings.TrimRight(line, " \t"))
		b.WriteByte('\n')
		lineStart = i + 1
	}

	if lineStart < len(s) {
		b.WriteString(strings.TrimRight(s[lineStart:], " \t"))
	}

	return b.String()
}
