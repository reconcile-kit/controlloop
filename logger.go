package controlloop

import "fmt"

type Logger interface {
	Error(args ...interface{})
	Info(args ...interface{})
}

type SimpleLogger struct{}

func (l *SimpleLogger) Error(args ...interface{}) {
	fmt.Println(args...)
}
func (l *SimpleLogger) Info(args ...interface{}) {
	fmt.Println(args...)
}
