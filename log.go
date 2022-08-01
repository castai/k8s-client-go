package client

import (
	"fmt"
	"os"
)

type Logger interface {
	Infof(format string, args ...any)
}

type DefaultLogger struct {
}

func (l *DefaultLogger) Infof(format string, args ...any) {
	fmt.Fprintf(os.Stdout, format, args...)
}
