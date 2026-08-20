package log

import (
	"runtime"
	"strconv"
	"strings"
	"sync"
)

const (
	fieldFile  = "file"
	fieldFunc  = "func"
	fieldLine  = "line"
	fieldStack = "stacktrace"

	// Internal field names stripped before the line is written.
	skipStackField = "_skip_stack"
	wantStackField = "_want_stack"

	maxWalkDepth   = 32
	maxStackFrames = 8
)

var (
	pcsPool = sync.Pool{
		New: func() any {
			pcs := make([]uintptr, maxWalkDepth)
			return &pcs
		},
	}

	thisPackage     string
	thisPackageOnce sync.Once
)

func currentPackage() string {
	thisPackageOnce.Do(func() {
		pc, _, _, ok := runtime.Caller(0)
		if !ok {
			thisPackage = "github.com/open4go/log"
			return
		}
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			thisPackage = "github.com/open4go/log"
			return
		}
		thisPackage = packageName(fn.Name())
	})
	return thisPackage
}

func packageName(fn string) string {
	for {
		lastPeriod := strings.LastIndex(fn, ".")
		lastSlash := strings.LastIndex(fn, "/")
		if lastPeriod > lastSlash {
			fn = fn[:lastPeriod]
			continue
		}
		return fn
	}
}

func isInternalFrame(f runtime.Frame) bool {
	if f.Function == "" {
		return true
	}
	pkg := packageName(f.Function)
	switch {
	case pkg == "runtime" || strings.HasPrefix(pkg, "runtime/"):
		return true
	case pkg == "github.com/sirupsen/logrus" || strings.HasPrefix(pkg, "github.com/sirupsen/logrus/"):
		return true
	case pkg != currentPackage():
		return false
	}
	// Call sites inside this package (middleware, tests) should still show up.
	if strings.HasSuffix(f.File, "_test.go") || strings.HasSuffix(f.File, "/middle.go") {
		return false
	}
	return true
}

func collectAppFrames(max int) []runtime.Frame {
	if max <= 0 {
		max = 1
	}
	pcsPtr := pcsPool.Get().(*[]uintptr)
	pcs := *pcsPtr
	defer pcsPool.Put(pcsPtr)

	n := runtime.Callers(2, pcs)
	if n == 0 {
		return nil
	}
	frames := runtime.CallersFrames(pcs[:n])
	out := make([]runtime.Frame, 0, max)
	for {
		f, more := frames.Next()
		if !isInternalFrame(f) {
			out = append(out, f)
			if len(out) >= max {
				break
			}
		}
		if !more {
			break
		}
	}
	return out
}

func firstAppFrame() (runtime.Frame, bool) {
	frames := collectAppFrames(1)
	if len(frames) == 0 {
		return runtime.Frame{}, false
	}
	return frames[0], true
}

// shortFunc keeps package.Type.Method (or package.Func.func1 for closures)
// instead of only the last identifier.
func shortFunc(full string) string {
	if full == "" {
		return "unknown"
	}
	if i := strings.LastIndex(full, "/"); i >= 0 {
		return full[i+1:]
	}
	return full
}

// shortFile keeps the last two path segments so handler.go in different
// packages stay distinguishable.
func shortFile(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	if path == "" {
		return "unknown"
	}
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return parts[len(parts)-1]
}

func formatFileLine(file string, line int) string {
	return shortFile(file) + ":" + strconv.Itoa(line)
}

func formatStack(max int) string {
	frames := collectAppFrames(max)
	if len(frames) == 0 {
		return ""
	}
	var b strings.Builder
	for i, f := range frames {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(shortFunc(f.Function))
		b.WriteByte('\n')
		b.WriteByte('\t')
		b.WriteString(formatFileLine(f.File, f.Line))
	}
	return b.String()
}
