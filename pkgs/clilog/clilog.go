// Package clilog provides an [slog.Handler] for human-readable console output.
//
// Features:
//   - Output to any [io.Writer] (default: os.Stderr).
//   - Each log line is prefixed with the program name (filepath.Base(os.Args[0])).
//   - Message field may contain a Go template (detected by "{{...}}" sequences).
//     Template data includes .Level, .Time, and all current attributes/groups.
//   - Non-template messages render level, optional timestamp, message and
//     attributes on one line (default) or multiple indented lines.
//   - Template functions: rfc3339, timeFormat, oneline.
//   - Error attribute values (including [ferrors] joined errors) are collapsed
//     to a single line in single-line mode and re-indented in multi-line mode.
package clilog

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/robdavid/genutil-go/opt"
	"github.com/robdavid/img-pin/pkgs/ferrors"
)

// Options configures the Handler.
type Options struct {
	// Level is the minimum log level. Defaults to slog.LevelInfo.
	Level slog.Leveler
	// MultiLine renders attributes on separate indented lines for non-template messages.
	MultiLine bool
	// IndentWidth is the number of spaces per indent level in multi-line mode. Defaults to 2.
	IndentWidth int
	// NoTime suppresses the timestamp from non-template log output.
	NoTime bool
	// TimeFormat is the time layout used in non-template output.
	// Defaults to time.RFC3339. Ignored when NoTime is true.
	TimeFormat string
	// ProgramName is the prefix prepended to every log line.
	// Defaults to filepath.Base(os.Args[0]).
	ProgramName string
	// AlwaysEmitLevel, when non-nil, causes the log level to be automatically
	// prepended to template-rendered output whenever the record's level is at
	// or below this value.  Non-template messages always include the level and
	// are unaffected.  Nil (the default) disables automatic level emission for
	// templates entirely.
	AlwaysEmitLevel opt.Val[slog.Level]
}

type groupOrAttrs struct {
	group string
	attrs []slog.Attr
}

// Handler is an [slog.Handler] that writes formatted log records to a writer.
type Handler struct {
	opts Options
	mu   *sync.Mutex
	out  io.Writer
	goas []groupOrAttrs
}

// New creates a new Handler writing to out. If out is nil, os.Stderr is used.
func New(out io.Writer, opts *Options) *Handler {
	h := &Handler{mu: &sync.Mutex{}}
	if out != nil {
		h.out = out
	} else {
		h.out = os.Stderr
	}
	if opts != nil {
		h.opts = *opts
	}
	if h.opts.Level == nil {
		h.opts.Level = slog.LevelInfo
	}
	if h.opts.IndentWidth == 0 {
		h.opts.IndentWidth = 2
	}
	if h.opts.ProgramName == "" && len(os.Args) > 0 {
		h.opts.ProgramName = filepath.Base(os.Args[0])
	}
	if !h.opts.NoTime && h.opts.TimeFormat == "" {
		h.opts.TimeFormat = time.RFC3339
	}
	return h
}

// Enabled reports whether the handler handles records at the given level.
func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

// WithAttrs returns a new Handler with the given attrs pre-applied.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	return h.withGroupOrAttrs(groupOrAttrs{attrs: attrs})
}

// WithGroup returns a new Handler with the given group name pushed onto the stack.
func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return h.withGroupOrAttrs(groupOrAttrs{group: name})
}

func (h *Handler) withGroupOrAttrs(goa groupOrAttrs) *Handler {
	h2 := *h
	h2.goas = make([]groupOrAttrs, len(h.goas)+1)
	copy(h2.goas, h.goas)
	h2.goas[len(h2.goas)-1] = goa
	return &h2
}

// Handle formats and writes the log record to the configured writer.
//
// If the message contains "{{" and "}}", it is treated as a Go template.
// The template data is a map with keys "Level" (string), "Time" (time.Time),
// and one key per attribute (or group name for nested groups).
// On template execution error the record is still emitted with an inline error note.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	if !h.Enabled(ctx, r.Level) {
		return nil
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s: ", h.opts.ProgramName)
	msg := r.Message
	if strings.Contains(msg, "{{") && strings.Contains(msg, "}}") {
		if err := h.handleTemplate(&buf, r, msg); err != nil {
			buf.Reset()
			fmt.Fprintf(&buf, "%s: %s [template error: %s]", h.opts.ProgramName, r.Level, err)
		}
	} else {
		h.handlePlain(&buf, r)
	}
	b := buf.Bytes()
	if len(b) > 0 && b[len(b)-1] != '\n' {
		buf.WriteByte('\n')
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.out.Write(buf.Bytes())
	return err
}

// ── Template support ─────────────────────────────────────────────────────────

// TemplateFuncs contains the functions available inside message templates.
//
//   - rfc3339(t time.Time) string  – formats t as RFC 3339.
//   - timeFormat(layout string, t time.Time) string  – formats t with layout.
//   - oneline(s string) string  – collapses a multi-line string to one line.
var TemplateFuncs = template.FuncMap{
	"rfc3339": func(t time.Time) string {
		return t.Format(time.RFC3339)
	},
	"timeFormat": func(layout string, t time.Time) string {
		return t.Format(layout)
	},
	"oneline": func(s string) string {
		return collapseToOneLine(s)
	},
}

// buildTemplateData builds the map[string]any used as the dot (.) value when
// executing a message template.  Special keys "Level" and "Time" are always
// present; all current attributes and groups are merged in at their natural
// nesting depth.
func buildTemplateData(level slog.Level, t time.Time, goas []groupOrAttrs, r slog.Record) map[string]any {
	// Per slog contract: trim trailing empty groups when the record has no attrs.
	trimmed := trimTrailingGroups(goas, r)

	data := map[string]any{
		"Level": level.String(),
		"Time":  t,
	}
	cur := data
	for _, goa := range trimmed {
		if goa.group != "" {
			nested := make(map[string]any)
			cur[goa.group] = nested
			cur = nested
		} else {
			for _, a := range goa.attrs {
				setAttrInMap(cur, a)
			}
		}
	}
	r.Attrs(func(a slog.Attr) bool {
		setAttrInMap(cur, a)
		return true
	})
	return data
}

func setAttrInMap(m map[string]any, a slog.Attr) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}
	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			nested := make(map[string]any)
			for _, ga := range a.Value.Group() {
				setAttrInMap(nested, ga)
			}
			m[a.Key] = nested
		} else {
			// Inline empty-key group directly into parent map.
			for _, ga := range a.Value.Group() {
				setAttrInMap(m, ga)
			}
		}
	} else {
		m[a.Key] = a.Value.Any()
	}
}

func (h *Handler) handleTemplate(buf *bytes.Buffer, r slog.Record, tmplStr string) error {
	if level, ok := h.opts.AlwaysEmitLevel.GetOK(); ok && r.Level <= level {
		fmt.Fprintf(buf, "%s ", r.Level)
	}
	tmpl, err := template.New("msg").Funcs(TemplateFuncs).Parse(tmplStr)
	if err != nil {
		return err
	}
	data := buildTemplateData(r.Level, r.Time, h.goas, r)
	return tmpl.Execute(buf, data)
}

// ── Plain (non-template) rendering ───────────────────────────────────────────

func (h *Handler) handlePlain(buf *bytes.Buffer, r slog.Record) {
	fmt.Fprintf(buf, "%s", r.Level)
	if !h.opts.NoTime && !r.Time.IsZero() {
		fmt.Fprintf(buf, " %s", r.Time.Format(h.opts.TimeFormat))
	}
	fmt.Fprintf(buf, " %s", r.Message)
	if h.opts.MultiLine {
		buf.WriteByte('\n')
		h.renderAttrsMultiLine(buf, r)
	} else {
		h.renderAttrsSingleLine(buf, r)
		buf.WriteByte('\n')
	}
}

// ── Single-line attribute rendering ──────────────────────────────────────────

func (h *Handler) renderAttrsSingleLine(buf *bytes.Buffer, r slog.Record) {
	goas := trimTrailingGroups(h.goas, r)
	prefix := ""
	for _, goa := range goas {
		if goa.group != "" {
			prefix = qualifiedKey(prefix, goa.group)
		} else {
			for _, a := range goa.attrs {
				h.writeAttrSingleLine(buf, a, prefix)
			}
		}
	}
	r.Attrs(func(a slog.Attr) bool {
		h.writeAttrSingleLine(buf, a, prefix)
		return true
	})
}

func (h *Handler) writeAttrSingleLine(buf *bytes.Buffer, a slog.Attr, prefix string) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}
	key := qualifiedKey(prefix, a.Key)
	if a.Value.Kind() == slog.KindGroup {
		for _, ga := range a.Value.Group() {
			h.writeAttrSingleLine(buf, ga, key)
		}
		return
	}
	fmt.Fprintf(buf, " %s=%s", key, h.formatSingleLine(a.Value))
}

func (h *Handler) formatSingleLine(v slog.Value) string {
	switch v.Kind() {
	case slog.KindTime:
		return v.Time().Format(time.RFC3339)
	case slog.KindAny:
		if err, ok := v.Any().(error); ok {
			return quoteIfNeeded(formatErrorSingleLine(err))
		}
		s := fmt.Sprintf("%v", v.Any())
		if strings.ContainsAny(s, "\n\r") {
			s = collapseToOneLine(s)
		}
		return quoteIfNeeded(s)
	case slog.KindString:
		s := v.String()
		if strings.ContainsAny(s, "\n\r") {
			s = collapseToOneLine(s)
		}
		return quoteIfNeeded(s)
	default:
		return v.String()
	}
}

// formatErrorSingleLine formats an error as a single line.
// For [ferrors] joined errors, constituent messages are separated by "; ".
func formatErrorSingleLine(err error) string {
	parts := ferrors.Split(err)
	if len(parts) == 1 {
		return collapseToOneLine(err.Error())
	}
	msgs := make([]string, 0, len(parts))
	for _, e := range parts {
		msgs = append(msgs, formatErrorSingleLine(e))
	}
	return strings.Join(msgs, "; ")
}

// ── Multi-line attribute rendering ───────────────────────────────────────────

// renderAttrsMultiLine writes all attributes indented under the log header.
// Everything starts at indent level 1 (IndentWidth spaces): group headers,
// ungrouped attrs, and nested group headers each add one additional level.
func (h *Handler) renderAttrsMultiLine(buf *bytes.Buffer, r slog.Record) {
	goas := trimTrailingGroups(h.goas, r)
	level := 1
	for _, goa := range goas {
		if goa.group != "" {
			fmt.Fprintf(buf, "%s%s:\n", strings.Repeat(" ", level*h.opts.IndentWidth), goa.group)
			level++
		} else {
			for _, a := range goa.attrs {
				h.writeAttrMultiLine(buf, a, level)
			}
		}
	}
	r.Attrs(func(a slog.Attr) bool {
		h.writeAttrMultiLine(buf, a, level)
		return true
	})
}

func (h *Handler) writeAttrMultiLine(buf *bytes.Buffer, a slog.Attr, level int) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}
	pad := strings.Repeat(" ", level*h.opts.IndentWidth)
	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			fmt.Fprintf(buf, "%s%s:\n", pad, a.Key)
			for _, ga := range a.Value.Group() {
				h.writeAttrMultiLine(buf, ga, level+1)
			}
		} else {
			for _, ga := range a.Value.Group() {
				h.writeAttrMultiLine(buf, ga, level)
			}
		}
		return
	}
	valStr := h.formatMultiLine(a.Value, level+1)
	if strings.Contains(valStr, "\n") {
		fmt.Fprintf(buf, "%s%s:\n%s\n", pad, a.Key, valStr)
	} else {
		fmt.Fprintf(buf, "%s%s: %s\n", pad, a.Key, valStr)
	}
}

// formatMultiLine formats a value for multi-line output.
// Multi-line string/error values are stripped of their own indentation and
// re-indented at continuationLevel using IndentWidth.
func (h *Handler) formatMultiLine(v slog.Value, continuationLevel int) string {
	pad := strings.Repeat(" ", continuationLevel*h.opts.IndentWidth)
	switch v.Kind() {
	case slog.KindTime:
		return v.Time().Format(time.RFC3339)
	case slog.KindAny:
		if err, ok := v.Any().(error); ok {
			return reindentString(err.Error(), pad)
		}
		return reindentString(fmt.Sprintf("%v", v.Any()), pad)
	case slog.KindString:
		return reindentString(v.String(), pad)
	default:
		return v.String()
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// trimTrailingGroups removes trailing group-only entries from goas when the
// record has no attrs, satisfying the slog handler contract for empty groups.
func trimTrailingGroups(goas []groupOrAttrs, r slog.Record) []groupOrAttrs {
	if r.NumAttrs() == 0 {
		for len(goas) > 0 && goas[len(goas)-1].group != "" {
			goas = goas[:len(goas)-1]
		}
	}
	return goas
}

// qualifiedKey joins prefix and key with ".".  Either may be empty.
func qualifiedKey(prefix, key string) string {
	switch {
	case prefix == "":
		return key
	case key == "":
		return prefix
	default:
		return prefix + "." + key
	}
}

// collapseToOneLine replaces newlines with spaces and trims each segment.
func collapseToOneLine(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == '\r' })
	out := parts[:0]
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return strings.Join(out, " ")
}

// reindentString strips each line's leading/trailing whitespace and re-indents
// every line with pad.  If the result is a single non-empty line the pad is
// omitted (the caller writes the value inline).
func reindentString(s string, pad string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	out := lines[:0]
	for _, l := range lines {
		if t := strings.TrimSpace(l); t != "" {
			out = append(out, pad+t)
		}
	}
	switch len(out) {
	case 0:
		return ""
	case 1:
		return strings.TrimSpace(out[0])
	default:
		return strings.Join(out, "\n")
	}
}

// quoteIfNeeded wraps s in Go double-quote syntax if it contains whitespace,
// double-quotes, or equals signs that would break key=value parsing.
func quoteIfNeeded(s string) string {
	if strings.ContainsAny(s, " \t\"\n\r=") {
		return fmt.Sprintf("%q", s)
	}
	return s
}
