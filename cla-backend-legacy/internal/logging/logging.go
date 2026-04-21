// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package logging

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/sirupsen/logrus"
)

const (
	LogFormatText    = "text"
	LogFormatJSON    = "json"
	LogFormatDefault = LogFormatText
)

var (
	logger    = logrus.New()
	logFormat = ""
)

// UTCFormatter matches cla-backend-go/logging.UTCFormatter.
type UTCFormatter struct {
	logrus.Formatter
}

func (u UTCFormatter) Format(e *logrus.Entry) ([]byte, error) {
	e.Time = e.Time.UTC()
	return u.Formatter.Format(e)
}

func init() {
	logFormat = strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT")))
	if logFormat == "" {
		logFormat = LogFormatDefault
	}
	if logFormat != LogFormatJSON && logFormat != LogFormatText {
		fmt.Printf("Unsupported logging format: %q - using default: %q\n", logFormat, LogFormatDefault)
		logFormat = LogFormatDefault
	}

	if logFormat == LogFormatJSON {
		logger.SetFormatter(UTCFormatter{
			Formatter: &logrus.JSONFormatter{
				TimestampFormat: time.RFC3339,
				PrettyPrint:     false,
			},
		})
	} else {
		logger.SetFormatter(UTCFormatter{
			Formatter: &logrus.TextFormatter{
				DisableColors:   false,
				TimestampFormat: time.RFC3339,
				FullTimestamp:   true,
			},
		})
	}

	logger.SetLevel(logrus.DebugLevel)
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "panic":
		logger.SetLevel(logrus.PanicLevel)
	case "fatal":
		logger.SetLevel(logrus.FatalLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	case "warn", "warning":
		logger.SetLevel(logrus.WarnLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "trace":
		logger.SetLevel(logrus.TraceLevel)
	}

	logger.Infof("Logger Format : %s", logFormat)
	logger.Infof("Logger Level : %s", logger.GetLevel())
}

// sanitizeForLog removes control characters and newlines to prevent log injection
// This function acts as a security barrier against log injection attacks
func sanitizeForLog(s string) string {
	// Remove all control characters except tab to prevent log injection
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\t' {
			return -1 // Remove control characters
		}
		return r
	}, s)
}

func sanitizeArgs(args []any) []any {
	out := make([]any, 0, len(args))
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			out = append(out, sanitizeForLog(v))
		case error:
			out = append(out, sanitizeForLog(v.Error()))
		default:
			out = append(out, v)
		}
	}
	return out
}

func safe(format string, args ...any) (string, []any) {
	return sanitizeForLog(format), sanitizeArgs(args)
}

func GetLogger() *logrus.Logger {
	return logger
}

func GetLogFormat() string {
	return logFormat
}

func IsJSONLogFormat() bool {
	return logFormat == LogFormatJSON
}

func IsTextLogFormat() bool {
	return logFormat == LogFormatText
}

func WithField(key string, value interface{}) *logrus.Entry {
	return logger.WithField(key, value)
}

func WithFields(fields logrus.Fields) *logrus.Entry {
	return logger.WithFields(fields)
}

func WithError(err error) *logrus.Entry {
	return logger.WithField("error", err)
}

func Debug(msg string) {
	logger.Debug(sanitizeForLog(msg))
}

func Debugf(msg string, args ...any) {
	f, a := safe(msg, args...)
	logger.Debugf(f, a...)
}

func Info(msg string) {
	logger.Info(sanitizeForLog(msg))
}

func Infof(msg string, args ...any) {
	f, a := safe(msg, args...)
	logger.Infof(f, a...)
}

func Warn(msg string) {
	logger.Warn(sanitizeForLog(msg))
}

func Warnf(msg string, args ...any) {
	f, a := safe(msg, args...)
	logger.Warnf(f, a...)
}

func Error(trace string, err error) {
	logger.WithField("line", sanitizeForLog(trace)).Error(err)
}

func Errorf(msg string, args ...any) {
	f, a := safe(msg, args...)
	logger.Errorf(f, a...)
}

func Fatal(args ...interface{}) {
	logger.Fatal(args...)
}

func Fatalf(msg string, args ...interface{}) {
	logger.Fatalf(msg, args...)
}

func Panic(args ...interface{}) {
	logger.Panic(args...)
}
func Panicf(msg string, args ...interface{}) {
	logger.Panicf(msg, args...)
}

func Println(args ...interface{}) {
	logger.Println(args...)
}

func Printf(msg string, args ...interface{}) {
	logger.Printf(msg, args...)
}

func Trace() string {
	pc := make([]uintptr, 15)
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	return fmt.Sprintf("%s,:%d %s\n", frame.File, frame.Line, frame.Function)
}
