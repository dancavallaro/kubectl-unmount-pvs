package logger

import (
	"fmt"

	"github.com/fatih/color"
)

type Logger struct {
}

func NewLogger() *Logger {
	return &Logger{}
}

func (l *Logger) Info(msg string, args ...interface{}) {
	log(color.FgHiCyan, msg, args...)
}

func (l *Logger) Warn(msg string, args ...interface{}) {
	log(color.FgHiYellow, msg, args...)
}

func log(col color.Attribute, msg string, args ...interface{}) {
	if msg == "" {
		fmt.Println("")
		return
	}

	c := color.New(col)
	c.Println(fmt.Sprintf(msg, args...))
}

func (l *Logger) Error(err error) {
	c := color.New(color.FgHiRed)
	c.Println(fmt.Sprintf("%#v", err))
}

func (l *Logger) Instructions(msg string, args ...interface{}) {
	white := color.New(color.FgHiWhite)
	white.Println("")
	white.Println(fmt.Sprintf(msg, args...))
}
