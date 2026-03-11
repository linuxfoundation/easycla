package logging

import (
	"log"
	"os"
	"strings"
)

func isDebug() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("LOG_LEVEL")))
	return v == "debug" || v == "trace"
}

func Debugf(format string, args ...any) {
	if isDebug() {
		log.Printf("DEBUG "+format, args...)
	}
}

func Infof(format string, args ...any) {
	log.Printf("INFO "+format, args...)
}

func Warnf(format string, args ...any) {
	log.Printf("WARN "+format, args...)
}

func Errorf(format string, args ...any) {
	log.Printf("ERROR "+format, args...)
}
