package log

import (
	"log"
	"os"
)

type Level int

const (
	Debug Level = iota
	Info
	Warn
	Error
)

type Logger interface {
	SetLevel(level Level)
	Debug(v ...interface{})
	Debugf(format string, v ...interface{})
	Info(v ...interface{})
	Infof(format string, v ...interface{})
	Warn(v ...interface{})
	Warnf(format string, v ...interface{})
	Error(v ...interface{})
	Errorf(format string, v ...interface{})
}

type logger struct {
	level Level

	d *log.Logger
	i *log.Logger
	w *log.Logger
	e *log.Logger
}

var _ Logger = (*logger)(nil)

func New(level Level) Logger {
	return &logger{
		level: level,
		d:     log.New(os.Stdout, "[D] ", log.Ldate|log.Lmicroseconds|log.Lmsgprefix),
		i:     log.New(os.Stdout, "[I] ", log.Ldate|log.Lmicroseconds|log.Lmsgprefix),
		w:     log.New(os.Stderr, "[W] ", log.Ldate|log.Lmicroseconds|log.Lmsgprefix),
		e:     log.New(os.Stderr, "[E] ", log.Ldate|log.Lmicroseconds|log.Lmsgprefix),
	}
}

func (l *logger) SetLevel(level Level) {
	l.level = level
}

func (l *logger) Debug(v ...interface{}) {
	l.log(Debug, l.d, v...)
}

func (l *logger) Debugf(format string, v ...interface{}) {
	l.logf(Debug, l.d, format, v...)
}

func (l *logger) Info(v ...interface{}) {
	l.log(Info, l.i, v...)
}

func (l *logger) Infof(format string, v ...interface{}) {
	l.logf(Info, l.i, format, v...)
}

func (l *logger) Warn(v ...interface{}) {
	l.log(Warn, l.w, v...)
}

func (l *logger) Warnf(format string, v ...interface{}) {
	l.logf(Warn, l.w, format, v...)
}

func (l *logger) Error(v ...interface{}) {
	l.log(Error, l.e, v...)
}

func (l *logger) Errorf(format string, v ...interface{}) {
	l.logf(Error, l.e, format, v...)
}

func (l *logger) log(level Level, logger *log.Logger, v ...interface{}) {
	if level < l.level {
		return
	}

	logger.Print(v...)
}

func (l *logger) logf(level Level, logger *log.Logger, format string, v ...interface{}) {
	if level < l.level {
		return
	}

	logger.Printf(format, v...)
}

type nopLogger struct{}

var _ Logger = nopLogger{}

func NewNop() Logger {
	return nopLogger{}
}

func (l nopLogger) SetLevel(level Level)                   {}
func (l nopLogger) Debug(v ...interface{})                 {}
func (l nopLogger) Debugf(format string, v ...interface{}) {}
func (l nopLogger) Info(v ...interface{})                  {}
func (l nopLogger) Infof(format string, v ...interface{})  {}
func (l nopLogger) Warn(v ...interface{})                  {}
func (l nopLogger) Warnf(format string, v ...interface{})  {}
func (l nopLogger) Error(v ...interface{})                 {}
func (l nopLogger) Errorf(format string, v ...interface{}) {}
