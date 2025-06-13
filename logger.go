package controlloop

import "fmt"

type Logger interface {
	Error(args ...interface{})
	Info(args ...interface{})
}

type logger struct{}

func (l *logger) Error(args ...interface{}) {
	fmt.Println(args...)
}
func (l *logger) Info(args ...interface{}) {
	fmt.Println(args...)
}
