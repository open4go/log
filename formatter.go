package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/sirupsen/logrus"
)

// Field names that open the line, in this order, so the important parts of
// every record can be read left to right without hunting for them.
var headFields = []string{
	"server",
	"trace",
}

// Fields placed right after msg / error / caller.
var contextFields = []string{
	"method",
	"route",
	"path",
	"status",
	"latency_ms",
	"ip",
	"merchantId",
	"operator",
	"user_id",
}

// Fields placed at the end of the line: large payloads, stack traces and
// static build metadata.
var tailFields = []string{
	"query",
	"req_body",
	"resp_body",
	fieldStack,
	"instance",
	"image",
	"git_commit",
	"git_branch",
	"build_time",
}

var reservedKeys = map[string]struct{}{"time": {}, "level": {}, "msg": {}}

// orderedJSONFormatter writes one JSON object per line with a stable key
// order: time, level, server, trace, msg, error, caller, request
// context, remaining fields (sorted), then payloads / stack / build meta.
type orderedJSONFormatter struct {
	TimestampFormat string
}

func (f *orderedJSONFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	b := entry.Buffer
	if b == nil {
		b = &bytes.Buffer{}
	}

	data := make(map[string]any, len(entry.Data))
	for k, v := range entry.Data {
		if _, clash := reservedKeys[k]; clash {
			k = "fields." + k
		}
		data[k] = v
	}

	used := make(map[string]struct{}, len(data)+3)
	first := true
	write := func(k string, v any) {
		if first {
			b.WriteByte('{')
			first = false
		} else {
			b.WriteByte(',')
		}
		writeJSONString(b, k)
		b.WriteByte(':')
		writeJSONValue(b, v)
	}
	take := func(k string) {
		if _, done := used[k]; done {
			return
		}
		v, ok := data[k]
		if !ok {
			return
		}
		used[k] = struct{}{}
		write(k, v)
	}

	format := f.TimestampFormat
	if format == "" {
		format = "2006-01-02T15:04:05.000Z07:00"
	}
	write("time", entry.Time.Format(format))
	write("level", entry.Level.String())
	for _, k := range headFields {
		take(k)
	}
	write("msg", entry.Message)
	take(logrus.ErrorKey)
	take(fieldCaller)
	take(fieldFunc)
	for _, k := range contextFields {
		take(k)
	}

	tail := make(map[string]struct{}, len(tailFields))
	for _, k := range tailFields {
		tail[k] = struct{}{}
	}
	rest := make([]string, 0, len(data))
	for k := range data {
		if _, done := used[k]; done {
			continue
		}
		if _, isTail := tail[k]; isTail {
			continue
		}
		rest = append(rest, k)
	}
	sort.Strings(rest)
	for _, k := range rest {
		take(k)
	}
	for _, k := range tailFields {
		take(k)
	}
	b.WriteString("}\n")
	return b.Bytes(), nil
}

func writeJSONString(b *bytes.Buffer, s string) {
	writeJSONValue(b, s)
}

func writeJSONValue(b *bytes.Buffer, v any) {
	switch x := v.(type) {
	case error:
		v = x.Error()
	case json.RawMessage:
		if json.Valid(x) {
			var c bytes.Buffer
			if err := json.Compact(&c, x); err == nil {
				b.Write(c.Bytes())
				return
			}
		}
		v = string(x)
	}
	var enc bytes.Buffer
	e := json.NewEncoder(&enc)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		enc.Reset()
		_ = e.Encode(fmt.Sprintf("%+v", v))
	}
	b.Write(bytes.TrimRight(enc.Bytes(), "\n"))
}
