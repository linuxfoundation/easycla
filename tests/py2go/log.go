package main

import (
	"fmt"
)

func Debugf(format string, args ...interface{}) {
	if DEBUG {
		fmt.Printf(format, args...)
	}
}
