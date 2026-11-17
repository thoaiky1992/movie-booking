package interfaces

import "context"

// Logger is the logging capability use cases need. *slog.Logger satisfies it,
// so use cases do not import a logging library.
type Logger interface {
	WarnContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
}
