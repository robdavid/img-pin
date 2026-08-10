package types

import (
	"log/slog"
	"os"
	"strings"

	"github.com/robdavid/genutil-go/opt"
	"github.com/robdavid/img-pin/pkgs/run"
)

// EnvOpt holds a value that is defaulted to an environment variable.
type EnvOpt struct {
	Env          string
	value        opt.Val[string]
	DefaultValue string
}

// Value returns the current value, or if not present, the environment variable is
// looked up (if the name is not empty), and if the result is non-empty, the current value
// is set to that value, and returned. The default value is returned if other values are
// not available.
func (e *EnvOpt) Value() string {
	return e.value.GetOrF(func() string {
		if e.Env != "" {
			if v, ok := os.LookupEnv(e.Env); ok {
				e.value.Set(v)
				return v
			}
		}
		e.value.Set(e.DefaultValue)
		return e.DefaultValue
	})
}

func (e *EnvOpt) Opt() opt.Val[string] {
	e.Value()
	return e.value
}

// Sets the name of the environment variable. The current value is reset. A reference
// the receiver is returned.
func (e *EnvOpt) SetEnv(name string) *EnvOpt {
	e.value.Unset()
	e.Env = name
	return e
}

// SetDefault sets the default value. The current value is reset. A reference to the receiver
// is returned.
func (e *EnvOpt) SetDefault(value string) *EnvOpt {
	e.value.Unset()
	e.DefaultValue = value
	return e
}

func (e *EnvOpt) Unset() *EnvOpt {
	e.value.Unset()
	return e
}

// UnsetEnvVar causes the environment variable that this [EnvOpt] refers to is reset.
func (e *EnvOpt) UnsetEnvVar() {
	if e.Env != "" {
		os.Unsetenv(e.Env)
	}
}

type HelmOpt struct {
	EnvOpt
	version opt.Val[string]
}

func (h *HelmOpt) SetEnv(env string) *HelmOpt {
	h.version.Unset()
	h.EnvOpt.SetEnv(env)
	return h
}

func (h *HelmOpt) SetDefault(def string) *HelmOpt {
	h.version.Unset()
	h.EnvOpt.SetDefault(def)
	return h
}

func (h *HelmOpt) Unset() *HelmOpt {
	h.EnvOpt.Unset()
	h.version.Unset()
	return h
}

func (h *HelmOpt) Version() opt.Val[string] {
	if h.version.IsEmpty() {
		helm := h.Value()
		slog := slog.With("helm", helm)
		if version, err := run.Run(helm, "version", "--short"); err != nil {
			slog.Warn("Cannot determine version of '{{.helm}}': {{.err}}", "err", err.Error())
		} else {
			h.version.Set(strings.TrimSpace(string(version)))
		}
		slog.Debug(`Using helm binary "{{.helm}} version {{.version}}"`, "version", h.version.GetOr("unknown"))
	}
	return h.version
}
