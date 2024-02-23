package liberlogger

import (
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

const (
	fatalLevel = "fatal"
	errorLevel = "error"
	warnLevel  = "warn"
	infoLevel  = "info"
	debugLevel = "debug"
)

func Init(logLevel string) {
	var zeroLogLevel zerolog.Level

	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	level := strings.ToLower(logLevel)

	switch level {
	case fatalLevel:
		zeroLogLevel = zerolog.FatalLevel
	case errorLevel:
		zeroLogLevel = zerolog.ErrorLevel
	case warnLevel:
		zeroLogLevel = zerolog.WarnLevel
	case debugLevel:
		zeroLogLevel = zerolog.DebugLevel
	default:
		zeroLogLevel = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(zeroLogLevel)
}
