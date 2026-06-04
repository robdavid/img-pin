package yaml

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	. "github.com/robdavid/genutil-go/errors/handler"
	"github.com/robdavid/genutil-go/functions"
	"github.com/robdavid/genutil-go/opt"
	"github.com/robdavid/genutil-go/slices"
	"gopkg.in/yaml.v3"
)

var ErrUnknownPE = errors.New("Unknown path element content")
var ErrBadType = errors.New("bad type")
var ErrWriteFailed = errors.New("failed to verify write")

type PathElement struct {
	index      opt.Val[int]
	key        opt.Val[string]
	re         opt.Ref[regexp.Regexp]
	multiLevel bool
}

func (pe PathElement) String() string {
	anyData := opt.FirstOfAny(pe.index, pe.key, pe.re)
	if anyData.IsEmpty() {
		return functions.IfElse(pe.multiLevel, "**", "*")
	}
	switch data := anyData.(type) {
	case opt.Opt[int]:
		return strconv.Itoa(data.Get())
	case opt.Opt[string]:
		return data.Get()
	case opt.Opt[regexp.Regexp]:
		return "/" + data.Ref().String() + "/"
	}
	panic(ErrUnknownPE)
}

func (pe *PathElement) Object() any {
	if obj := opt.FirstOfAny(pe.index, pe.key, pe.re); obj.HasValue() {
		return functions.IfElseF(obj.IsRef(), obj.RefAny, obj.GetAny)
	} else if pe.multiLevel {
		return "**"
	} else {
		return "*"
	}
}

func PeAny(arg any) (pe PathElement) {
	switch t := any(arg).(type) {
	case string:
		switch {
		case t == "**":
			pe.multiLevel = true
		case t == "*":
		case strings.HasPrefix(t, "/") && strings.HasSuffix(t, "/"):
			pe.re = opt.Reference(regexp.MustCompile(t[1 : len(t)-1]))
		default:
			pe.key = opt.Value(t)
		}
	case int:
		pe.index = opt.Value(t)
	case *regexp.Regexp:
		pe.re = opt.Reference(t)
	case PathElement:
		pe = t
	default:
		Raise(fmt.Errorf("%w: cannot build a path element from a %T", ErrBadType, arg))
	}
	return
}

func Pe[T int | string | *regexp.Regexp](arg T) (pe PathElement) {
	return PeAny(arg)
}

func (pe *PathElement) Equals(peo *PathElement) bool {
	o1 := opt.FirstOfAny(pe.index, pe.key, pe.re)
	o2 := opt.FirstOfAny(peo.index, peo.key, peo.re)
	if o1.IsEmpty() && o2.IsEmpty() {
		return pe.multiLevel == peo.multiLevel
	} else if o1.IsEmpty() != o2.IsEmpty() {
		return false
	} else {
		if o1.IsRef() != o2.IsRef() {
			return false
		} else if o1.IsRef() {
			re1, ok1 := o1.RefAny().(*regexp.Regexp)
			re2, ok2 := o2.RefAny().(*regexp.Regexp)
			return ok1 && ok2 && re1.String() == re2.String()
		} else {
			return o1.GetAny() == o2.GetAny()
		}
	}
}

func (pe *PathElement) Compare(peo *PathElement) int {
	i1, ok1 := pe.index.GetOK()
	i2, ok2 := peo.index.GetOK()
	if ok1 && ok2 {
		return i1 - i2
	} else {
		return strings.Compare(pe.String(), peo.String())
	}
}

func (pe *PathElement) IsIndex() bool   { return pe.index.HasValue() }
func (pe *PathElement) IsKey() bool     { return pe.key.HasValue() }
func (pe *PathElement) IsMatcher() bool { return pe.key.IsEmpty() && pe.index.IsEmpty() }
func (pe *PathElement) IsMatchAll() bool {
	return pe.key.IsEmpty() && pe.index.IsEmpty() && pe.re.IsEmpty()
}
func (pe *PathElement) IsMultiMatchAll() bool { return pe.IsMatchAll() && pe.multiLevel }

func (pe *PathElement) MatchKey(node *yaml.Node) (match opt.Opt[PathElement]) {
	var nodeKey string
	var nodeIndex int
	var err error
	match = opt.EmptyRef[PathElement]()
	if err = node.Decode(&nodeKey); err == nil {
		if key, ok := pe.key.GetOK(); ok {
			if key == nodeKey {
				match = opt.Reference(pe)
			}
		} else if re, ok := pe.re.RefOK(); ok {
			if re.MatchString(nodeKey) {
				match = opt.Value(Pe(nodeKey))
			}
		} else if pe.IsMatchAll() {
			match = opt.Value(Pe(nodeKey))
		}
	} else if err = node.Decode(&nodeIndex); err == nil {
		if index, ok := pe.index.GetOK(); ok {
			if index == nodeIndex {
				match = opt.Reference(pe)
			}
		} else if pe.IsMatchAll() {
			match = opt.Value(Pe(nodeIndex))
		}
	}
	return
}

func (pe *PathElement) MatchIndex(nodeIndex int) opt.Opt[PathElement] {
	if index, ok := pe.index.GetOK(); ok {
		if index == nodeIndex {
			return opt.Reference(pe)
		}
		return opt.Empty[PathElement]()
	} else if pe.IsMatchAll() {
		return opt.Value(Pe(nodeIndex))
	} else {
		return opt.Empty[PathElement]()
	}
}

type Path []PathElement

func (p Path) String() string {
	var out strings.Builder
	for i, pe := range p {
		var str string
		if pe.IsIndex() {
			str = "[" + pe.String() + "]"
		} else {
			if i > 0 {
				out.WriteRune('.')
			}
			str = pe.String()
			if strings.Contains(str, ".") || strings.Contains(str, "[") || strings.Contains(str, "]") {
				str = `"` + strings.ReplaceAll(str, `"`, `\"`) + `"`
			}
		}
		out.WriteString(str)
	}
	return out.String()
}

func (p Path) Objects() []any {
	return slices.Map(p, func(pe PathElement) any { return pe.Object() })
}

func (p Path) Equals(p2 Path) bool {
	if len(p) != len(p2) {
		return false
	}
	for i := range p {
		if !p[i].Equals(&p2[i]) {
			return false
		}
	}
	return true
}

func (p Path) Compare(p2 Path) int {
	for i := range min(len(p), len(p2)) {
		cmp := p[i].Compare(&p2[i])
		if cmp != 0 {
			return cmp
		}
	}
	return len(p) - len(p2)
}

// PathOf builds a path from a list of elements. Elements can be
// one of [string], [int], *[regexp.Regexp] or the literal "*"
// (match exactly one element), or the literal "**" (match zero
// or more elements).
func MkPath(elements []any) Path {
	return slices.Map(elements, PeAny)
}

// PathOf builds a path from a list of elements. Elements can be
// one of [string], [int], *[regexp.Regexp] or the literal "*"
// (match exactly one element), or the literal "**" (match zero
// or more elements).
func PathOf(elements ...any) Path {
	return MkPath(elements)
}

// SortPaths sorts a slice of Paths in place
func SortPaths(paths []Path) {
	less := func(p1, p2 Path) bool {
		return p1.Compare(p2) < 0
	}
	slices.SortUsing(paths, less)
}

type PathAndNode struct {
	Path Path
	Node *yaml.Node
}

func collect(node *yaml.Node, path Path, wild bool, parent Path) (matches []PathAndNode) {

	if len(path) == 0 {
		if len(node.Content) == 0 {
			matches = slices.New(PathAndNode{slices.Clone(parent), node})
		}
		return
	}

	if path[0].IsMultiMatchAll() {
		return collect(node, path[1:], true, parent)
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			matches = append(matches, collect(child, path, wild, parent)...)
		}
	case yaml.MappingNode:
		pe := path[0]
		for i := 0; i+1 < len(node.Content); i += 2 {
			var newMatches []PathAndNode
			matchPe, ok := pe.MatchKey(node.Content[i]).GetOK()
			if ok {
				newMatches = collect(node.Content[i+1], path[1:], false, append(parent, matchPe))
			}
			if wild && newMatches == nil {
				var key string
				if node.Content[i].Decode(&key) == nil {
					newMatches = collect(node.Content[i+1], path, true, append(parent, Pe(key)))
				}
			}
			matches = append(matches, newMatches...)
		}
	case yaml.SequenceNode:
		pe := path[0]
		for i, node := range node.Content {
			if matchPe, ok := pe.MatchIndex(i).GetOK(); ok {
				matches = append(matches, collect(node, path[1:], false, append(parent, matchPe))...)
			} else if wild {
				matches = append(matches, collect(node, path, true, append(parent, Pe(i)))...)
			}
		}
	case yaml.AliasNode:
		return collect(node.Alias, path, wild, parent)
	}
	return
}

// MatchOne returns a single path that matches the one given. If there is no match,
// a nil node is return with a false boolean. If there is one match, it is returned
// along with a true boolean. If there is more than one, the first is returned with
// a false boolean.
func MatchOne(node *yaml.Node, path Path) (*yaml.Node, bool) {
	matches := collect(node, path, false, nil)
	if len(matches) > 0 {
		return matches[0].Node, len(matches) == 1
	} else {
		return nil, false
	}
}

type Scalar interface {
	string | int | bool
}

func Get[T Scalar](root *yaml.Node, path ...any) opt.Val[T] {
	return GetPath[T](root, MkPath(path))
}

func GetPath[T Scalar](root *yaml.Node, path Path) opt.Val[T] {
	node, ok := MatchOne(root, path)
	if !ok {
		return opt.Empty[T]()
	} else {
		var result T
		err := node.Decode(&result)
		if err != nil {
			return opt.Empty[T]()
		} else {
			return opt.Value(result)
		}
	}
}

type PathValue[T Scalar] struct {
	Path  Path
	Value T
}

func NewPathValue[T Scalar](path Path, value T) PathValue[T] {
	return PathValue[T]{Path: path, Value: value}
}

func ReadPathValue[T Scalar](root *yaml.Node, path Path) PathValue[T] {
	var zero T
	return PathValue[T]{
		Path:  path,
		Value: GetPath[T](root, path).GetOr(zero),
	}
}

func (pv PathValue[T]) WriteInto(root *yaml.Node) error {
	return WriteNode(root, pv.Path, pv.Value)
}

func (pv PathValue[T]) IsNil() bool { return pv.Path == nil }

func (pv PathValue[T]) IsEmpty() bool {
	if pv.Path == nil {
		return true
	}
	var zero T
	return pv.Value == zero
}

// MatchMany returns all the [yaml.Node]s that reside at a path matching path, relative to node
func MatchMany(node *yaml.Node, path Path) []*yaml.Node {
	return slices.Map(collect(node, path, false, nil),
		func(vp PathAndNode) *yaml.Node { return vp.Node })
}

func MatchManyWithNode(node *yaml.Node, path Path) []PathAndNode {
	return collect(node, path, false, nil)
}

// EntryGroup groups PathAndNode entries that share a common prefix (parent path).
type EntryGroup struct {
	Prefix  Path
	Entries []PathAndNode
}

// GroupByPrefix groups a slice of PathAndNode entries by their parent path
// (prefix), where the prefix is the path with the final element removed.
func GroupByPrefix(entries []PathAndNode) []EntryGroup {
	groups := make(map[string]*EntryGroup)
	var keyOrder []string

	for _, entry := range entries {
		var prefix Path
		if len(entry.Path) > 0 {
			prefix = entry.Path[:len(entry.Path)-1]
		}
		key := prefix.String()
		if b, ok := groups[key]; ok {
			b.Entries = append(b.Entries, entry)
		} else {
			keyOrder = append(keyOrder, key)
			groups[key] = &EntryGroup{Prefix: prefix, Entries: []PathAndNode{entry}}
		}
	}

	result := make([]EntryGroup, len(keyOrder))
	for i, key := range keyOrder {
		result[i] = *groups[key]
	}
	return result
}

func verifyPut[T Scalar](root *yaml.Node, value T, path Path) error {
	if current, ok := GetPath[T](root, path).GetOK(); ok {
		if current != value {
			return fmt.Errorf("%w at path %s, wrote %v, found %v", ErrWriteFailed, path, value, current)
		}
	} else {
		return fmt.Errorf("%w at path %s, wrote %v, found no value", ErrWriteFailed, path, value)
	}
	return nil
}

func Put[T Scalar](root *yaml.Node, value T, path ...any) error {
	p := MkPath(path)
	if err := WriteNode(root, p, value); err != nil {
		return err
	}
	return verifyPut(root, value, p)

}

func PutPath[T Scalar](root *yaml.Node, value T, path Path) error {
	if err := WriteNode(root, path, value); err != nil {
		return err
	}
	return verifyPut(root, value, path)
}

var (
	ErrWritePath      = errors.New("write path error")
	ErrWriteCollision = errors.New("write path collision")
)

// WriteNode writes a scalar value into a yaml AST tree at the location
// specified by path, relative to root. Intermediate mapping and sequence nodes
// are created as needed. Returns ErrWriteCollision if an existing node of the
// wrong type occupies any position along the path.
func WriteNode(root *yaml.Node, path Path, value any) error {
	if root == nil {
		return fmt.Errorf("%w: root node is nil", ErrWritePath)
	}
	node := derefAlias(root)
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) != 1 {
			return fmt.Errorf("%w: document node must have exactly one child", ErrWritePath)
		}
		node = derefAlias(node.Content[0])
	}
	for i, pe := range path {
		node = derefAlias(node)
		if pe.IsMatchAll() {
			return fmt.Errorf("%w: wildcard path element %q not allowed for writing", ErrWritePath, pe.String())
		}
		if !pe.IsIndex() && !pe.IsKey() {
			return fmt.Errorf("%w: path element %q must be a key or index", ErrWritePath, pe.String())
		}
		isLast := i == len(path)-1
		if isLast {
			if err := setLeaf(node, pe, value); err != nil {
				return err
			}
		} else {
			var err error
			node, err = getOrCreateContainer(node, pe, path[i+1])
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// derefAlias follows any chain of AliasNode references to the underlying node.
func derefAlias(node *yaml.Node) *yaml.Node {
	for node.Kind == yaml.AliasNode {
		node = node.Alias
	}
	return node
}

// getOrCreateContainer navigates from parent into the child identified by pe,
// creating intermediate container nodes as needed. nextPE determines the type
// of container to create if the child doesn't exist.
func getOrCreateContainer(parent *yaml.Node, pe PathElement, nextPE PathElement) (*yaml.Node, error) {
	childKind := yaml.MappingNode
	if nextPE.IsIndex() {
		childKind = yaml.SequenceNode
	}
	if idx, ok := pe.index.GetOK(); ok {
		if parent.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("%w: expected sequence node but got %s", ErrWriteCollision, nodeKind(parent.Kind))
		}
		for len(parent.Content) <= idx {
			parent.Content = append(parent.Content, &yaml.Node{Kind: childKind, Tag: containerTag(childKind)})
		}
		return typedChild(derefAlias(parent.Content[idx]), childKind)
	}
	if key, ok := pe.key.GetOK(); ok {
		if parent.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%w: expected mapping node but got %s", ErrWriteCollision, nodeKind(parent.Kind))
		}
		for i := 0; i+1 < len(parent.Content); i += 2 {
			if parent.Content[i].Value == key {
				return typedChild(derefAlias(parent.Content[i+1]), childKind)
			}
		}
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
		valueNode := &yaml.Node{Kind: childKind, Tag: containerTag(childKind)}
		parent.Content = append(parent.Content, keyNode, valueNode)
		return valueNode, nil
	}
	return nil, fmt.Errorf("%w: path element %q has no key or index", ErrWritePath, pe.String())
}
func updateScalarMetadata(scalar *yaml.Node, reference *yaml.Node) {
	scalar.Line = reference.Line
	scalar.Column = reference.Column
	scalar.Style = reference.Style
	scalar.Anchor = reference.Anchor
	scalar.FootComment = reference.FootComment
	scalar.LineComment = reference.LineComment
	scalar.HeadComment = reference.HeadComment
}

// setLeaf writes a scalar value into the child of parent identified by pe,
// creating the child or updating an existing scalar as needed.
func setLeaf(parent *yaml.Node, pe PathElement, value any) error {
	var scalar yaml.Node
	if err := scalar.Encode(value); err != nil {
		return err
	}
	if scalar.Kind != yaml.ScalarNode {
		return fmt.Errorf("%w: value must be a scalar, got %s", ErrWritePath, nodeKind(scalar.Kind))
	}
	if idx, ok := pe.index.GetOK(); ok {
		if parent.Kind != yaml.SequenceNode {
			return fmt.Errorf("%w: expected sequence node for index access but got %s", ErrWriteCollision, nodeKind(parent.Kind))
		}
		for len(parent.Content) <= idx {
			parent.Content = append(parent.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null"})
		}
		existing := derefAlias(parent.Content[idx])
		if existing.Kind != yaml.ScalarNode {
			return fmt.Errorf("%w: expected scalar at path index %d but found %s", ErrWriteCollision, idx, nodeKind(existing.Kind))
		}
		updateScalarMetadata(&scalar, existing)
		*existing = scalar
		return nil
	}
	if key, ok := pe.key.GetOK(); ok {
		if parent.Kind != yaml.MappingNode {
			return fmt.Errorf("%w: expected mapping node for key access but got %s", ErrWriteCollision, nodeKind(parent.Kind))
		}
		for i := 0; i+1 < len(parent.Content); i += 2 {
			if parent.Content[i].Value == key {
				existing := derefAlias(parent.Content[i+1])
				if existing.Kind != yaml.ScalarNode {
					return fmt.Errorf("%w: expected scalar at path key %q but found %s", ErrWriteCollision, key, nodeKind(existing.Kind))
				}
				updateScalarMetadata(&scalar, existing)
				*existing = scalar
				return nil
			}
		}
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
		parent.Content = append(parent.Content, keyNode, &scalar)
		return nil
	}
	return fmt.Errorf("%w: path element %q has no key or index", ErrWritePath, pe.String())
}

// typedChild checks that node is a container of the expected kind, returning an
// error if it is a scalar or a different container type.
func typedChild(node *yaml.Node, expectedKind yaml.Kind) (*yaml.Node, error) {
	if node.Kind == yaml.ScalarNode {
		return nil, fmt.Errorf("%w: expected %s but found scalar", ErrWriteCollision, nodeKind(expectedKind))
	}
	if node.Kind != expectedKind {
		return nil, fmt.Errorf("%w: expected %s but found %s", ErrWriteCollision, nodeKind(expectedKind), nodeKind(node.Kind))
	}
	return node, nil
}

// nodeKind returns a human-readable name for a yaml node kind.
func nodeKind(kind yaml.Kind) string {
	switch kind {
	case yaml.ScalarNode:
		return "scalar"
	case yaml.MappingNode:
		return "mapping"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.DocumentNode:
		return "document"
	case yaml.AliasNode:
		return "alias"
	default:
		return "unknown"
	}
}

// containerTag returns the YAML tag for a mapping or sequence node kind.
func containerTag(kind yaml.Kind) string {
	switch kind {
	case yaml.MappingNode:
		return "!!map"
	case yaml.SequenceNode:
		return "!!seq"
	default:
		return ""
	}
}
