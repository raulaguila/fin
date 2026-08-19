package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Config controls logger initialization.
type Config struct {
	Level      string
	Format     string
	TimeFormat string
	Fields     map[string]any
	Out        io.Writer
}

// Logger wraps zerolog with a small API compatible with project usage.
type Logger struct {
	z zerolog.Logger
}

func defaultConfig(config ...Config) Config {
	cfg := Config{
		Level:      "info",
		Format:     "text",
		TimeFormat: time.RFC3339Nano,
		Fields:     map[string]any{},
		Out:        os.Stdout,
	}

	if len(config) > 0 {
		in := config[0]
		if in.Level != "" {
			cfg.Level = in.Level
		}
		if in.Format != "" {
			cfg.Format = in.Format
		}
		if in.TimeFormat != "" {
			cfg.TimeFormat = in.TimeFormat
		}
		if in.Fields != nil {
			cfg.Fields = in.Fields
		}
		if in.Out != nil {
			cfg.Out = in.Out
		}
	}

	cfg.Fields["pid"] = os.Getpid()
	return cfg
}

// New creates a logger with sane defaults.
func New(config ...Config) *Logger {
	cfg := defaultConfig(config...)

	zerolog.TimeFieldFormat = cfg.TimeFormat
	zerolog.SetGlobalLevel(ParseLevel(cfg.Level))

	writer := cfg.Out
	if strings.EqualFold(cfg.Format, "text") {
		writer = newConsoleWriter(cfg.Out, cfg.TimeFormat)
	}

	ctx := zerolog.New(writer).With().Timestamp()
	for k, v := range cfg.Fields {
		ctx = ctx.Interface(k, v)
	}

	return &Logger{z: ctx.Logger()}
}

// ParseLevel parses string level into zerolog level.
func ParseLevel(level string) zerolog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "trace":
		return zerolog.TraceLevel
	case "debug":
		return zerolog.DebugLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	default:
		return zerolog.InfoLevel
	}
}

// Zerolog returns the underlying zerolog logger.
func (l *Logger) Zerolog() *zerolog.Logger {
	if l == nil {
		return nil
	}
	return &l.z
}

// With returns a new logger with additional fields.
func (l *Logger) With(kv ...any) *Logger {
	if l == nil {
		return New()
	}

	ctx := l.z.With()
	for i := 0; i < len(kv); i++ {
		if attr, ok := kv[i].(slog.Attr); ok {
			appendSlogAttrToContext(&ctx, attr)
			continue
		}

		if i+1 >= len(kv) {
			break
		}
		key, ok := kv[i].(string)
		if !ok {
			continue
		}
		ctx = ctx.Interface(key, kv[i+1])
		i++
	}

	return &Logger{z: ctx.Logger()}
}

// Debug logs at debug level.
func (l *Logger) Debug(msg string, attrs ...slog.Attr) {
	if l == nil {
		return
	}
	ev := l.z.Debug()
	appendAttrs(ev, attrs)
	ev.Msg(msg)
}

// Info logs at info level.
func (l *Logger) Info(msg string, attrs ...slog.Attr) {
	if l == nil {
		return
	}
	ev := l.z.Info()
	appendAttrs(ev, attrs)
	ev.Msg(msg)
}

// Warn logs at warn level.
func (l *Logger) Warn(msg string, attrs ...slog.Attr) {
	if l == nil {
		return
	}
	ev := l.z.Warn()
	appendAttrs(ev, attrs)
	ev.Msg(msg)
}

// Error logs at error level.
func (l *Logger) Error(msg string, attrs ...slog.Attr) {
	if l == nil {
		return
	}
	ev := l.z.Error()
	appendAttrs(ev, attrs)
	ev.Msg(msg)
}

// Shutdown emits a standard shutdown message.
func (l *Logger) Shutdown(reason string) {
	l.Info("shutting down", slog.String("reason", reason))
}

func appendAttrs(ev *zerolog.Event, attrs []slog.Attr) {
	for _, attr := range attrs {
		appendSlogAttr(ev, attr)
	}
}

func appendSlogAttrToContext(ctx *zerolog.Context, attr slog.Attr) {
	if attr.Equal(slog.Attr{}) {
		return
	}
	ctx.Interface(attr.Key, attrToAny(attr.Value))
}

func appendSlogAttr(ev *zerolog.Event, attr slog.Attr) {
	if attr.Equal(slog.Attr{}) {
		return
	}

	switch attr.Value.Kind() {
	case slog.KindString:
		ev.Str(attr.Key, attr.Value.String())
	case slog.KindInt64:
		ev.Int64(attr.Key, attr.Value.Int64())
	case slog.KindUint64:
		ev.Uint64(attr.Key, attr.Value.Uint64())
	case slog.KindFloat64:
		ev.Float64(attr.Key, attr.Value.Float64())
	case slog.KindBool:
		ev.Bool(attr.Key, attr.Value.Bool())
	case slog.KindDuration:
		ev.Dur(attr.Key, attr.Value.Duration())
	case slog.KindTime:
		ev.Time(attr.Key, attr.Value.Time())
	default:
		ev.Interface(attr.Key, attrToAny(attr.Value))
	}
}

func attrToAny(value slog.Value) any {
	switch value.Kind() {
	case slog.KindAny:
		return value.Any()
	case slog.KindGroup:
		group := value.Group()
		out := make(map[string]any, len(group))
		for _, item := range group {
			out[item.Key] = attrToAny(item.Value)
		}
		return out
	default:
		return value.Any()
	}
}

// ANSI SGR color codes used when rendering "method" and "status" inline in
// the console writer. Values cover the standard 4-bit palette supported by
// every reasonable terminal emulator.
const (
	ansiReset   = "\x1b[0m"
	ansiRed     = "\x1b[31m"
	ansiGreen   = "\x1b[32m"
	ansiYellow  = "\x1b[33m"
	ansiBlue    = "\x1b[34m"
	ansiMagenta = "\x1b[35m"
	ansiCyan    = "\x1b[36m"
	ansiGray    = "\x1b[90m"
	ansiBoldOn  = "\x1b[1m"
)

// newConsoleWriter returns a zerolog.ConsoleWriter that renders HTTP method
// and status inline on the main log line, with ANSI colour based on the HTTP
// semantics. Colour is automatically suppressed when stdout is not a TTY
// (handled by zerolog internally via NoColor).
func newConsoleWriter(out io.Writer, timeFormat string) zerolog.ConsoleWriter {
	return zerolog.ConsoleWriter{
		Out:        out,
		TimeFormat: timeFormat,
		// method and status are promoted to "parts" so FormatPartValueByName
		// can style them. When absent on a log event (e.g. a regular app log
		// line) zerolog silently skips the part.
		PartsOrder: []string{
			zerolog.TimestampFieldName,
			zerolog.LevelFieldName,
			zerolog.CallerFieldName,
			"method",
			"status",
			zerolog.MessageFieldName,
		},
		// Suppress trailing "method=..." / "status=..." key=value pairs
		// — those values are already shown inline via the parts above.
		FieldsExclude: []string{"method", "status"},
		FormatPartValueByName: func(i any, fieldName string) string {
			switch fieldName {
			case "method":
				return formatHTTPMethod(i)
			case "status":
				return formatHTTPStatus(i)
			default:
				if i == nil {
					return ""
				}
				return fmt.Sprintf("%v", i)
			}
		},
	}
}

func formatHTTPMethod(v any) string {
	if v == nil {
		return ""
	}
	method := strings.ToUpper(strings.TrimSpace(fmt.Sprintf("%v", v)))
	if method == "" {
		return ""
	}
	// Left-justified to 7 chars (OPTIONS is the longest) so subsequent
	// columns align regardless of verb.
	padded := fmt.Sprintf("%-7s", method)
	var colour string
	switch method {
	case "GET":
		colour = ansiGreen
	case "POST":
		colour = ansiYellow
	case "PUT":
		colour = ansiBlue
	case "PATCH":
		colour = ansiCyan
	case "DELETE":
		colour = ansiRed
	case "HEAD":
		colour = ansiMagenta
	case "OPTIONS":
		colour = ansiGray
	default:
		return padded
	}
	return colour + ansiBoldOn + padded + ansiReset
}

func formatHTTPStatus(v any) string {
	if v == nil {
		return ""
	}
	status := strings.TrimSpace(fmt.Sprintf("%v", v))
	if len(status) == 0 {
		return ""
	}
	var colour string
	switch status[0] {
	case '2':
		colour = ansiGreen
	case '3':
		colour = ansiCyan
	case '4':
		colour = ansiYellow
	case '5':
		colour = ansiRed
	default:
		return status
	}
	return colour + status + ansiReset
}
