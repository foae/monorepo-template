package observability

import (
	"runtime"
	"strconv"
	"strings"
)

// PanicStack returns the current goroutine's static function, file and line
// metadata only. Unlike runtime/debug.Stack it never includes argument values,
// which may contain credentials or other sensitive runtime data.
func PanicStack() string {
	var pcs [64]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	var stack strings.Builder
	for {
		frame, more := frames.Next()
		stack.WriteString(frame.Function)
		stack.WriteByte('\n')
		stack.WriteString(frame.File)
		stack.WriteByte(':')
		stack.WriteString(strconv.Itoa(frame.Line))
		stack.WriteByte('\n')
		if !more {
			return stack.String()
		}
	}
}
