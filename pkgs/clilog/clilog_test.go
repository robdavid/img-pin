package clilog_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/robdavid/genutil-go/opt"
	"github.com/robdavid/img-pin/pkgs/clilog"
	"github.com/robdavid/img-pin/pkgs/ferrors"
)

// fixed timestamp used across tests to keep output deterministic
var testTime = time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

// newH returns a Handler writing to buf with ProgramName and NoTime set for
// deterministic output.  Additional options can be overridden via opts.
// Time output is only enabled when opts.TimeFormat is non-empty.
func newH(buf *bytes.Buffer, opts *clilog.Options) *clilog.Handler {
	o := clilog.Options{
		ProgramName: "prog",
		NoTime:      true,
	}
	if opts != nil {
		if opts.Level != nil {
			o.Level = opts.Level
		}
		o.MultiLine = opts.MultiLine
		if opts.IndentWidth != 0 {
			o.IndentWidth = opts.IndentWidth
		}
		if opts.ProgramName != "" {
			o.ProgramName = opts.ProgramName
		}
		// Only enable time when an explicit format is requested.
		if opts.TimeFormat != "" {
			o.NoTime = false
			o.TimeFormat = opts.TimeFormat
		}
	}
	return clilog.New(buf, &o)
}

// record builds a slog.Record with the given level, message and attrs.
func record(level slog.Level, msg string, attrs ...slog.Attr) slog.Record {
	r := slog.NewRecord(testTime, level, msg, 0)
	r.AddAttrs(attrs...)
	return r
}

// ── Basic single-line ────────────────────────────────────────────────────────

func TestSingleLinePlain(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, nil)
	_ = h.Handle(context.Background(), record(slog.LevelInfo, "hello"))
	want := "prog: INFO hello\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestSingleLineWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, nil)
	_ = h.Handle(context.Background(), record(slog.LevelInfo, "msg",
		slog.String("key", "value"),
		slog.Int("count", 42),
	))
	want := "prog: INFO msg key=value count=42\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestSingleLineStringWithSpacesQuoted(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, nil)
	_ = h.Handle(context.Background(), record(slog.LevelInfo, "msg", slog.String("desc", "hello world")))
	want := `prog: INFO msg desc="hello world"` + "\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestSingleLineMultilineStringCollapsed(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, nil)
	_ = h.Handle(context.Background(), record(slog.LevelInfo, "msg", slog.String("v", "line1\nline2\nline3")))
	want := `prog: INFO msg v="line1 line2 line3"` + "\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// ── Time in non-template output ───────────────────────────────────────────────

func TestTimeIncluded(t *testing.T) {
	var buf bytes.Buffer
	h := clilog.New(&buf, &clilog.Options{
		ProgramName: "prog",
		TimeFormat:  time.RFC3339,
	})
	_ = h.Handle(context.Background(), record(slog.LevelInfo, "msg"))
	want := "prog: INFO 2024-01-15T10:30:00Z msg\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestTimeOmittedWhenNoTime(t *testing.T) {
	var buf bytes.Buffer
	h := clilog.New(&buf, &clilog.Options{ProgramName: "prog", NoTime: true})
	_ = h.Handle(context.Background(), record(slog.LevelInfo, "msg"))
	want := "prog: INFO msg\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// ── Enabled / level filtering ─────────────────────────────────────────────────

func TestEnabledFiltersLevel(t *testing.T) {
	var buf bytes.Buffer
	h := clilog.New(&buf, &clilog.Options{
		ProgramName: "prog",
		Level:       slog.LevelWarn,
		NoTime:      true,
	})
	_ = h.Handle(context.Background(), record(slog.LevelInfo, "should not appear"))
	if buf.Len() != 0 {
		t.Errorf("expected no output for level below threshold, got %q", buf.String())
	}
}

// ── WithAttrs / WithGroup ─────────────────────────────────────────────────────

func TestWithAttrsSingleLine(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, nil)
	h2 := h.WithAttrs([]slog.Attr{slog.String("svc", "api")})
	_ = h2.Handle(context.Background(), record(slog.LevelInfo, "msg", slog.Int("code", 200)))
	want := "prog: INFO msg svc=api code=200\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestWithGroupSingleLine(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, nil)
	h2 := h.WithGroup("server").WithAttrs([]slog.Attr{slog.String("host", "localhost"), slog.Int("port", 8080)})
	_ = h2.Handle(context.Background(), record(slog.LevelInfo, "started"))
	want := "prog: INFO started server.host=localhost server.port=8080\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestNestedGroupSingleLine(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, nil)
	h2 := h.WithGroup("a").WithAttrs([]slog.Attr{slog.String("x", "1")}).WithGroup("b").WithAttrs([]slog.Attr{slog.String("y", "2")})
	_ = h2.Handle(context.Background(), record(slog.LevelInfo, "msg"))
	want := "prog: INFO msg a.x=1 a.b.y=2\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestTrailingEmptyGroupOmitted(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, nil)
	h2 := h.WithGroup("empty")
	_ = h2.Handle(context.Background(), record(slog.LevelInfo, "msg"))
	want := "prog: INFO msg\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// ── Multi-line mode ───────────────────────────────────────────────────────────

func TestMultiLinePlain(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, &clilog.Options{MultiLine: true})
	_ = h.Handle(context.Background(), record(slog.LevelInfo, "hello",
		slog.String("key", "value"),
		slog.Int("count", 42),
	))
	want := "prog: INFO hello\n  key: value\n  count: 42\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestMultiLineWithGroup(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, &clilog.Options{MultiLine: true})
	h2 := h.WithGroup("server").WithAttrs([]slog.Attr{slog.String("host", "localhost"), slog.Int("port", 8080)})
	_ = h2.Handle(context.Background(), record(slog.LevelInfo, "started"))
	// group header and its attrs are both indented (group at level 1, attrs at level 2)
	want := "prog: INFO started\n  server:\n    host: localhost\n    port: 8080\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestMultiLineMultilineString(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, &clilog.Options{MultiLine: true})
	_ = h.Handle(context.Background(), record(slog.LevelInfo, "msg", slog.String("output", "line1\nline2\nline3")))
	want := "prog: INFO msg\n  output:\n    line1\n    line2\n    line3\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestMultiLineCustomIndent(t *testing.T) {
	var buf bytes.Buffer
	h := clilog.New(&buf, &clilog.Options{ProgramName: "prog", NoTime: true, MultiLine: true, IndentWidth: 4})
	_ = h.Handle(context.Background(), record(slog.LevelInfo, "msg", slog.String("k", "v")))
	want := "prog: INFO msg\n    k: v\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// ── Template messages ─────────────────────────────────────────────────────────

func TestTemplateSimple(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, nil)
	_ = h.Handle(context.Background(), record(slog.LevelWarn, "status={{ .code }}", slog.Int("code", 404)))
	want := "prog: status=404\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestTemplateLevelAndTime(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, nil)
	_ = h.Handle(context.Background(), record(slog.LevelError, "{{ .Level }} at {{ rfc3339 .Time }}: boom"))
	want := "prog: ERROR at 2024-01-15T10:30:00Z: boom\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestTemplateTimeFormat(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, nil)
	_ = h.Handle(context.Background(), record(slog.LevelInfo, `{{ timeFormat "2006-01-02" .Time }}`))
	want := "prog: 2024-01-15\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestTemplateWithGroupAttr(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, nil)
	h2 := h.WithGroup("req").WithAttrs([]slog.Attr{slog.String("method", "GET"), slog.Int("status", 200)})
	_ = h2.Handle(context.Background(), record(slog.LevelInfo, "{{ .req.method }} {{ .req.status }}"))
	want := "prog: GET 200\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestTemplateOneline(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, nil)
	_ = h.Handle(context.Background(), record(slog.LevelInfo, "{{ oneline .msg }}", slog.String("msg", "line1\n  line2\n  line3")))
	want := "prog: line1 line2 line3\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestTemplateError(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, nil)
	// Has {{ and }} but references an unknown function — parse fails gracefully.
	_ = h.Handle(context.Background(), record(slog.LevelInfo, "{{ unknownFunc .Level }}"))
	got := buf.String()
	if !strings.HasPrefix(got, "prog: ") || !strings.Contains(got, "template error") {
		t.Errorf("expected fallback error line, got %q", got)
	}
}

// ── Error attribute rendering ─────────────────────────────────────────────────

func TestErrorSingleLine(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, nil)
	err := errors.New("something went wrong")
	_ = h.Handle(context.Background(), record(slog.LevelError, "failed", slog.Any("err", err)))
	want := "prog: ERROR failed err=\"something went wrong\"\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestFerrorsJoinedErrorSingleLine(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, nil)
	err := ferrors.Join(errors.New("err1"), errors.New("err2"), errors.New("err3"))
	_ = h.Handle(context.Background(), record(slog.LevelError, "failed", slog.Any("err", err)))
	want := `prog: ERROR failed err="err1; err2; err3"` + "\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestFerrorsJoinedErrorMultiLine(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, &clilog.Options{MultiLine: true})
	err := ferrors.Join(errors.New("err1"), errors.New("err2"))
	_ = h.Handle(context.Background(), record(slog.LevelError, "failed", slog.Any("err", err)))
	// err at level 1 (2 spaces), its lines at level 2 (4 spaces)
	want := "prog: ERROR failed\n  err:\n    err1\n    err2\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestWrappedFerrorsErrorMultiLine(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, &clilog.Options{MultiLine: true})
	inner := ferrors.Join(errors.New("cause1"), errors.New("cause2"))
	outer := errors.New("outer context: " + inner.Error())
	_ = h.Handle(context.Background(), record(slog.LevelError, "oops", slog.Any("err", outer)))
	got := buf.String()
	if !strings.HasPrefix(got, "prog: ERROR oops\n  err:\n") {
		t.Errorf("unexpected output: %q", got)
	}
	if !strings.Contains(got, "cause1") || !strings.Contains(got, "cause2") {
		t.Errorf("expected error content in output: %q", got)
	}
}

// ── Inline group attr ─────────────────────────────────────────────────────────

func TestInlineGroupAttr(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, nil)
	// slog.Group with empty key inlines attrs at current level
	_ = h.Handle(context.Background(), record(slog.LevelInfo, "msg",
		slog.Group("", slog.String("a", "1"), slog.String("b", "2")),
	))
	want := "prog: INFO msg a=1 b=2\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestNamedGroupAttr(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, nil)
	_ = h.Handle(context.Background(), record(slog.LevelInfo, "msg",
		slog.Group("grp", slog.String("x", "1"), slog.String("y", "2")),
	))
	want := "prog: INFO msg grp.x=1 grp.y=2\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// ── ProgramName default ───────────────────────────────────────────────────────

func TestDefaultProgramNameIsSet(t *testing.T) {
	var buf bytes.Buffer
	// Don't override ProgramName — it defaults to filepath.Base(os.Args[0])
	h := clilog.New(&buf, &clilog.Options{NoTime: true})
	_ = h.Handle(context.Background(), record(slog.LevelInfo, "hi"))
	got := buf.String()
	if !strings.Contains(got, ": INFO hi\n") {
		t.Errorf("expected 'progname: INFO hi', got %q", got)
	}
}

// ── AlwaysEmitLevel ───────────────────────────────────────────────────────────

func ptr[T any](v T) *T { return &v }

// Template messages below or at the threshold get the level prepended.
func TestAlwaysEmitLevelBelowThreshold(t *testing.T) {
	var buf bytes.Buffer
	h := clilog.New(&buf, &clilog.Options{
		ProgramName:     "prog",
		NoTime:          true,
		AlwaysEmitLevel: opt.Value(slog.LevelWarn),
	})
	// INFO (0) <= WARN (4): level should be prepended
	_ = h.Handle(context.Background(), record(slog.LevelInfo, "request {{ .method }}", slog.String("method", "GET")))
	want := "prog: INFO request GET\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestAlwaysEmitLevelAtThreshold(t *testing.T) {
	var buf bytes.Buffer
	h := clilog.New(&buf, &clilog.Options{
		ProgramName:     "prog",
		NoTime:          true,
		AlwaysEmitLevel: opt.Value(slog.LevelWarn),
	})
	// WARN (4) <= WARN (4): level should be prepended
	_ = h.Handle(context.Background(), record(slog.LevelWarn, "disk at {{ .pct }}%", slog.Int("pct", 90)))
	want := "prog: WARN disk at 90%\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestAlwaysEmitLevelAboveThreshold(t *testing.T) {
	var buf bytes.Buffer
	h := clilog.New(&buf, &clilog.Options{
		ProgramName:     "prog",
		NoTime:          true,
		AlwaysEmitLevel: opt.Value(slog.LevelWarn),
	})
	// ERROR (8) > WARN (4): level should NOT be prepended
	_ = h.Handle(context.Background(), record(slog.LevelError, "{{ .msg }}", slog.String("msg", "boom")))
	want := "prog: boom\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestAlwaysEmitLevelNilDefault(t *testing.T) {
	var buf bytes.Buffer
	h := newH(&buf, nil) // AlwaysEmitLevel is nil
	_ = h.Handle(context.Background(), record(slog.LevelError, "{{ .msg }}", slog.String("msg", "boom")))
	// No level prepended for templates by default
	want := "prog: boom\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// Non-template messages always emit the level regardless of AlwaysEmitLevel.
func TestAlwaysEmitLevelDoesNotAffectPlain(t *testing.T) {
	var buf bytes.Buffer
	h := clilog.New(&buf, &clilog.Options{
		ProgramName:     "prog",
		NoTime:          true,
		AlwaysEmitLevel: opt.Value(slog.LevelWarn),
	})
	_ = h.Handle(context.Background(), record(slog.LevelError, "plain message"))
	want := "prog: ERROR plain message\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}
