package controlloop

import "fmt"

type Logger interface {
	Error(err error)
	Info(format string, args ...interface{})
}

type logger struct{}

func (l *logger) Error(err error) {
	fmt.Println(err.Error())
}
func (l *logger) Info(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
}
