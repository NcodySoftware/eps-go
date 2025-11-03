package log

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	LVL_TRACE byte = iota + 1
	LVL_DEBUG
	LVL_INFO
	LVL_NOTIFY
	LVL_WARN
	LVL_ERR
	LVL_FATAL
)

type logHandlerFunc = func(level byte, message string)

type Logger struct {
	handlers []logHandlerFunc
	prefix   string
	level    byte
	sem      chan struct{}
}

func New(level byte, prefix string, handlers ...logHandlerFunc) *Logger {
	const maxGo = 1
	var l *Logger
	if level == 0 {
		level = LVL_INFO
	}
	if len(handlers) == 0 {
		handlers = append(handlers, StderrHandler)
	}

	l = &Logger{
		level:    level,
		prefix:   prefix,
		handlers: handlers,
		sem:      make(chan struct{}, maxGo),
	}
	return l
}

func (l *Logger) Trace(v ...any) {
	l.log(LVL_TRACE, fmt.Sprint(v...))
}

func (l *Logger) Tracef(format string, v ...any) {
	l.log(LVL_TRACE, fmt.Sprintf(format, v...))
}

func (l *Logger) Debug(v ...any) {
	l.log(LVL_DEBUG, fmt.Sprint(v...))
}

func (l *Logger) Debugf(format string, v ...any) {
	l.log(LVL_DEBUG, fmt.Sprintf(format, v...))
}

func (l *Logger) Info(v ...any) {
	l.log(LVL_INFO, fmt.Sprint(v...))
}

func (l *Logger) Infof(format string, v ...any) {
	l.log(LVL_INFO, fmt.Sprintf(format, v...))
}

func (l *Logger) Notify(v ...any) {
	l.log(LVL_NOTIFY, fmt.Sprint(v...))
}

func (l *Logger) Notifyf(format string, v ...any) {
	l.log(LVL_NOTIFY, fmt.Sprintf(format, v...))
}

func (l *Logger) Warn(v ...any) {
	l.log(LVL_WARN, fmt.Sprint(v...))
}

func (l *Logger) Warnf(format string, v ...any) {
	l.log(LVL_WARN, fmt.Sprintf(format, v...))
}

func (l *Logger) Err(v ...any) {
	l.log(LVL_ERR, fmt.Sprint(v...))
}

func (l *Logger) Errf(format string, v ...any) {
	l.log(LVL_ERR, fmt.Sprintf(format, v...))
}

func (l *Logger) Fatal(v ...any) {
	l.log(LVL_FATAL, fmt.Sprint(v...))
}

func (l *Logger) Fatalf(format string, v ...any) {
	l.log(LVL_FATAL, fmt.Sprintf(format, v...))
}

func (l *Logger) log(lvl byte, message string) {
	if lvl < l.level {
		return
	}
	buf := bytes.NewBuffer(make([]byte, 0, len(message)))
	buf.WriteString(time.Now().Format(time.DateTime))
	buf.WriteString(" |")
	buf.WriteString(lvlToString(lvl))
	buf.WriteString("| ")
	if l.prefix != "" {
		buf.WriteString(l.prefix)
		buf.WriteString(": ")
	}
	buf.WriteString(message)
	l.sem <- struct{}{}
	go func() {
		for _, h := range l.handlers {
			h(lvl, buf.String())
		}
		<-l.sem
		if lvl == LVL_FATAL {
			panic(buf.String())
		}
	}()
}

func StderrHandler(lvl byte, message string) {
	fmt.Fprintln(os.Stderr, message)
}

func LevelFromString(level string) byte {
	switch strings.ToUpper(level) {
	case "TRACE":
		return LVL_TRACE
	case "DEBUG":
		return LVL_DEBUG
	case "INFO":
		return LVL_INFO
	case "NOTIFY":
		return LVL_NOTIFY
	case "WARN":
		return LVL_WARN
	case "ERR":
		return LVL_ERR
	case "FATAL":
		return LVL_FATAL
	}
	return 0
}

func lvlToString(level byte) string {
	switch level {
	case LVL_TRACE:
		return "TRACE"
	case LVL_DEBUG:
		return "DEBUG"
	case LVL_INFO:
		return "INFO"
	case LVL_NOTIFY:
		return "NOTIFY"
	case LVL_WARN:
		return "WARN"
	case LVL_ERR:
		return "ERR"
	case LVL_FATAL:
		return "FATAL"
	}
	return ""
}
