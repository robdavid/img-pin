package clilog

import "log/slog"

type LevelFlag struct{ l *slog.Level }

func (f LevelFlag) String() string     { return f.l.String() }
func (f LevelFlag) Set(s string) error { return f.l.UnmarshalText([]byte(s)) }
func (f LevelFlag) Type() string       { return "string" }

func MakeLevelFlag(l *slog.Level) LevelFlag { return LevelFlag{l: l} }
