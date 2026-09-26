package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

func New(env string) zerolog.Logger {
	if env == "development" {
		writer := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}
		return zerolog.New(writer).With().Timestamp().Logger()
	}
	return zerolog.New(os.Stderr).With().Timestamp().Logger()
}
