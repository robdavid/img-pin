package yaml

import y3 "gopkg.in/yaml.v3"

// EqualData reports whether two YAML AST trees contain the same data.
//
// It ignores presentation/style metadata (string style, comments, anchors,
// formatting positions, etc.) and compares only data content.
func EqualData(a, b *y3.Node) bool {
	seen := make(map[nodePair]bool)
	return equalNodeData(a, b, seen)
}

type nodePair struct {
	a *y3.Node
	b *y3.Node
}

func equalNodeData(a, b *y3.Node, seen map[nodePair]bool) bool {
	a = derefAliases(a)
	b = derefAliases(b)

	if a == nil || b == nil {
		return a == b
	}

	p := nodePair{a: a, b: b}
	if seen[p] {
		return true
	}
	seen[p] = true

	if a.Kind != b.Kind {
		return false
	}

	switch a.Kind {
	case y3.DocumentNode:
		if len(a.Content) != len(b.Content) {
			return false
		}
		for i := range a.Content {
			if !equalNodeData(a.Content[i], b.Content[i], seen) {
				return false
			}
		}
		return true

	case y3.MappingNode:
		if len(a.Content) != len(b.Content) || len(a.Content)%2 != 0 {
			return false
		}

		used := make([]bool, len(b.Content)/2)
		for i := 0; i < len(a.Content); i += 2 {
			ak, av := a.Content[i], a.Content[i+1]
			matched := false

			for j := 0; j < len(b.Content); j += 2 {
				idx := j / 2
				if used[idx] {
					continue
				}

				bk, bv := b.Content[j], b.Content[j+1]
				if equalNodeData(ak, bk, seen) && equalNodeData(av, bv, seen) {
					used[idx] = true
					matched = true
					break
				}
			}

			if !matched {
				return false
			}
		}
		return true

	case y3.SequenceNode:
		if len(a.Content) != len(b.Content) {
			return false
		}
		for i := range a.Content {
			if !equalNodeData(a.Content[i], b.Content[i], seen) {
				return false
			}
		}
		return true

	case y3.ScalarNode:
		return a.Tag == b.Tag && a.Value == b.Value

	case y3.AliasNode:
		return equalNodeData(a.Alias, b.Alias, seen)
	}

	return false
}

func derefAliases(n *y3.Node) *y3.Node {
	for n != nil && n.Kind == y3.AliasNode && n.Alias != nil {
		n = n.Alias
	}
	return n
}
