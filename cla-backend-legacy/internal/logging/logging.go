// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package logging

import (
	"log"
	"os"
	"strings"
	"unicode"
)

func isDebug() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("LOG_LEVEL")))
	return v == "debug" || v == "trace"
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

func Debugf(format string, args ...any) {
	if isDebug() {
		// Sanitize format string and args for log injection prevention
		safeFormat := sanitizeForLog(format)
		var safeArgs []any
		for _, arg := range args {
			if s, ok := arg.(string); ok {
				safeArgs = append(safeArgs, sanitizeForLog(s))
			} else {
				safeArgs = append(safeArgs, arg)
			}
		}
		// Use a safe logging approach to prevent injection
		log.Print("DEBUG " + safeFormat)
		if len(safeArgs) > 0 {
			for i, arg := range safeArgs {
				log.Printf("  arg[%d]: %v", i, arg)
			}
		}
	}
}

func Infof(format string, args ...any) {
	safeFormat := sanitizeForLog(format)
	var safeArgs []any
	for _, arg := range args {
		if s, ok := arg.(string); ok {
			safeArgs = append(safeArgs, sanitizeForLog(s))
		} else {
			safeArgs = append(safeArgs, arg)
		}
	}
	// Use a safe logging approach to prevent injection
	log.Print("INFO " + safeFormat)
	if len(safeArgs) > 0 {
		for i, arg := range safeArgs {
			log.Printf("  arg[%d]: %v", i, arg)
		}
	}
}

func Warnf(format string, args ...any) {
	safeFormat := sanitizeForLog(format)
	var safeArgs []any
	for _, arg := range args {
		if s, ok := arg.(string); ok {
			safeArgs = append(safeArgs, sanitizeForLog(s))
		} else {
			safeArgs = append(safeArgs, arg)
		}
	}
	// Use a safe logging approach to prevent injection
	log.Print("WARN " + safeFormat)
	if len(safeArgs) > 0 {
		for i, arg := range safeArgs {
			log.Printf("  arg[%d]: %v", i, arg)
		}
	}
}

func Errorf(format string, args ...any) {
	safeFormat := sanitizeForLog(format)
	var safeArgs []any
	for _, arg := range args {
		if s, ok := arg.(string); ok {
			safeArgs = append(safeArgs, sanitizeForLog(s))
		} else {
			safeArgs = append(safeArgs, arg)
		}
	}
	// Use a safe logging approach to prevent injection
	log.Print("ERROR " + safeFormat)
	if len(safeArgs) > 0 {
		for i, arg := range safeArgs {
			log.Printf("  arg[%d]: %v", i, arg)
		}
	}
}
