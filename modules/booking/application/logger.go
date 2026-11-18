package application

import "context"

type discardLogger struct{}

func (discardLogger) WarnContext(context.Context, string, ...any)  {}
func (discardLogger) ErrorContext(context.Context, string, ...any) {}

func loggerOrDiscard(logger Logger) Logger {
	if logger == nil {
		return discardLogger{}
	}
	return logger
}
