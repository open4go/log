package log

import "github.com/sirupsen/logrus"

// locationHook attaches caller file/func/line on every enabled log line,
// and a cleaned stacktrace on error (or when the caller asked for one).
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

	if _, ok := entry.Data[fieldFile]; !ok {
		if f, ok := firstAppFrame(); ok {
			entry.Data[fieldFile] = formatFileLine(f.File, f.Line)
			entry.Data[fieldFunc] = shortFunc(f.Function)
			entry.Data[fieldLine] = f.Line
		}
	}

	if skipStack {
		return nil
	}
	if _, exists := entry.Data[fieldStack]; exists {
		return nil
	}
	if !wantStack && entry.Level > logrus.ErrorLevel {
		return nil
	}
	if stack := formatStack(maxStackFrames); stack != "" {
		entry.Data[fieldStack] = stack
	}
	return nil
}
