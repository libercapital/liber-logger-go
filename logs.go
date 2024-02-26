package liberlogger

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// LogFieldsKey Key used for access the Value in the context.Context
type LogFieldsKey struct{}

func Info(ctx context.Context) *zerolog.Event {
	fields := ctx.Value(LogFieldsKey{})
	return log.Ctx(log.Logger.WithContext(ctx)).Info().Fields(fields)
}

func Debug(ctx context.Context) *zerolog.Event {
	fields := ctx.Value(LogFieldsKey{})
	return log.Ctx(log.Logger.WithContext(ctx)).Debug().Fields(fields)
}

func Warn(ctx context.Context) *zerolog.Event {
	fields := ctx.Value(LogFieldsKey{})
	return log.Ctx(log.Logger.WithContext(ctx)).Warn().Fields(fields)
}

func Error(ctx context.Context, err error) *zerolog.Event {
	fields := ctx.Value(LogFieldsKey{})
	return log.Ctx(log.Logger.WithContext(ctx)).Error().Fields(fields).Stack().Err(err)
}

func Panic(ctx context.Context, err error) *zerolog.Event {
	fields := ctx.Value(LogFieldsKey{})
	return log.Ctx(log.Logger.WithContext(ctx)).Panic().Fields(fields).Stack().Err(err)
}

func Fatal(ctx context.Context, err error) *zerolog.Event {
	fields := ctx.Value(LogFieldsKey{})
	return log.Ctx(log.Logger.WithContext(ctx)).Fatal().Fields(fields).Stack().Err(err)
}
