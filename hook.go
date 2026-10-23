package log

import "github.com/sirupsen/logrus"

// locationHook attaches a short `caller` (e.g. "middle/jwt.go:68") to every
// line and a cleaned stacktrace only when it is worth the noise: panic /
// fatal by default, error too when log.stacktrace=error, or when the call
// site explicitly asked for one (ErrorWithStack, WarnWithStack).
// Location is captured at write time so Log(ctx) stored on a Gin context
// still reports the real call site, not the middleware that created it.
type locationHook struct{}

func (h locationHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h locationHook) Fire(entry *logrus.Entry) error {
	if entry.Data == nil {
		entry.Data = logrus.Fields{}
	}

	_, skipStack := entry.Data[skipStackField]
	delete(entry.Data, skipStackField)
	_, wantStack := entry.Data[wantStackField]
	delete(entry.Data, wantStackField)
	_, skipCaller := entry.Data[skipCallerField]
	delete(entry.Data, skipCallerField)

	_, hasCaller := entry.Data[fieldCaller]
	_, hasFile := entry.Data[fieldFile]
	if !skipCaller && !hasCaller && !hasFile {
		if f, ok := firstAppFrame(); ok {
			entry.Data[fieldCaller] = formatFileLine(f.File, f.Line)
			if settings().callerFunc {
				entry.Data[fieldFunc] = shortFunc(f.Function)
			}
		}
	}

	if skipStack {
		return nil
	}
	if _, exists := entry.Data[fieldStack]; exists {
		return nil
	}
	if !wantStack && !settings().stackEnabled(entry.Level) {
		return nil
	}
	if stack := formatStack(maxStackFrames); stack != "" {
		entry.Data[fieldStack] = stack
	}
	return nil
}
