package controlloop

import "fmt"

type Logger interface {
	Error(args ...interface{})
	Info(format string, args ...interface{})
}

type logger struct{}

func (l *logger) Error(args ...interface{}) {
	fmt.Println(args...)
}
func (l *logger) Info(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
}
