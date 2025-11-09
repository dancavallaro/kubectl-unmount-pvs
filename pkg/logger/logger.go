package logger

import (
	"fmt"
	"io"

	"github.com/fatih/color"
)

type Logger struct {
	w io.Writer
}

func NewLogger(w io.Writer) *Logger {
	return &Logger{
		w: w,
	}
}

func (l *Logger) Info(msg string, args ...any) {
	l.log(color.FgHiCyan, msg+"\n", args...)
}

func (l *Logger) Warn(msg string, args ...any) {
	l.log(color.FgHiYellow, msg+"\n", args...)
}

func (l *Logger) Error(err error) {
	l.log(color.FgHiRed, "%v\n", err)
}

func (l *Logger) Instructions(msg string, args ...any) {
	l.log(color.FgHiWhite, "\n"+msg, args...)
}

func (l *Logger) log(col color.Attribute, msg string, args ...any) {
	if msg == "" {
		fmt.Println("")
		return
	}

	c := color.New(col)
	_, _ = c.Fprint(l.w, fmt.Sprintf(msg, args...))
}
