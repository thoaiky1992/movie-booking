package server

import (
	"log/slog"

	"go.uber.org/fx/fxevent"
)

type fxeventLogger struct{ logger *slog.Logger }

func newFxEventLogger(logger *slog.Logger) fxevent.Logger {
	return fxeventLogger{logger: logger}
}

func (l fxeventLogger) LogEvent(event fxevent.Event) {
	if failed, ok := event.(*fxevent.Started); ok && failed.Err != nil {
		l.logger.Error("dependency_graph_start_failed", "error", failed.Err)
	}
}
