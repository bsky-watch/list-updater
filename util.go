package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

func setupLogging(ctx context.Context) context.Context {
	var output io.Writer

	basedir := ""
	_, sourceFile, _, ok := runtime.Caller(0)
	if ok {
		basedir = filepath.Dir(sourceFile) + "/"
	}

	output = zerolog.ConsoleWriter{
		Out:        os.Stderr,
		NoColor:    true,
		TimeFormat: time.RFC3339,
		PartsOrder: []string{
			zerolog.LevelFieldName,
			zerolog.CallerFieldName,
			zerolog.TimestampFieldName,
			zerolog.MessageFieldName,
		},
		FieldsExclude:    []string{"config"},
		FormatFieldName:  func(i interface{}) string { return fmt.Sprintf("%s:", i) },
		FormatFieldValue: func(i interface{}) string { return fmt.Sprintf("%s", i) },
		FormatCaller: func(i interface{}) string {
			s := strings.TrimPrefix(i.(string), basedir)
			if s == "" {
				return "::"
			}
			parts := strings.SplitN(s, ":", 2)
			if len(parts) == 1 {
				return fmt.Sprintf(" filename=%s::", parts[0])
			}
			return fmt.Sprintf(" filename=%s,line=%s::", parts[0], parts[1])
		},
		FormatLevel: func(i interface{}) string {
			if i == nil {
				return "::notice"
			}
			lvl, _ := zerolog.ParseLevel(i.(string))
			ghLevel := "notice"
			switch {
			case lvl <= zerolog.InfoLevel || lvl == zerolog.NoLevel:
				ghLevel = "notice"
			case lvl == zerolog.WarnLevel:
				ghLevel = "warning"
			default:
				ghLevel = "error"
			}
			return fmt.Sprintf("::%s", ghLevel)
		},
	}

	logger := zerolog.New(output).Level(zerolog.Level(zerolog.DebugLevel)).With().Caller().Timestamp().Logger()

	ctx = logger.WithContext(ctx)

	zerolog.DefaultContextLogger = &logger
	log.SetOutput(logger)

	return ctx
}
