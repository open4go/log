package log

import (
	"os"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/spf13/viper"
)

// Service kind values attached to every log line as the "kind" field.
const (
	KindAPI  = "api"
	KindRPC  = "rpc"
	KindTask = "task"
)

var (
	kindValue atomic.Pointer[string]

	// hostnameLookup is os.Hostname in production. Tests replace it so
	// inference does not depend on the machine running the suite.
	hostnameLookup = os.Hostname

	rpcHostSuffix  = regexp.MustCompile(`-rpc-\d+$`)
	taskHostSuffix = regexp.MustCompile(`-task-\d+$`)
)

// Kind returns the cached service kind: api, rpc, or task.
// The value is resolved once at package init and again on Init.
func Kind() string {
	if k := kindValue.Load(); k != nil && *k != "" {
		return *k
	}
	return refreshKind()
}

func refreshKind() string {
	host := ""
	if hostnameLookup != nil {
		h, err := hostnameLookup()
		if err == nil {
			host = h
		}
	}
	k := resolveKind(os.Getenv, host)
	kindValue.Store(&k)
	return k
}

// resolveKind infers the service kind. Order:
//  1. Explicit SERVER_KIND / LOG_KIND, then viper server.kind / log.kind
//     (trim, lower case; invalid values are ignored).
//  2. COMPOSE_PROJECT_NAME: rpc-publish or rpc-* → rpc; task-publish or task-* → task.
//  3. Hostname (Docker container name): rpc-publish / -rpc- / rpc- prefix /
//     -rpc or -rpc-<digits> suffix → rpc; the same patterns with "task" → task.
//  4. Default api.
func resolveKind(getenv func(string) string, hostname string) string {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	for _, v := range []string{
		getenv("SERVER_KIND"),
		getenv("LOG_KIND"),
		viper.GetString("server.kind"),
		viper.GetString("log.kind"),
	} {
		if k := parseKind(v); k != "" {
			return k
		}
	}
	if k := kindFromCompose(getenv("COMPOSE_PROJECT_NAME")); k != "" {
		return k
	}
	if k := kindFromHostname(hostname); k != "" {
		return k
	}
	return KindAPI
}

func parseKind(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case KindAPI, KindRPC, KindTask:
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return ""
	}
}

func kindFromCompose(project string) string {
	p := strings.ToLower(strings.TrimSpace(project))
	if p == "" {
		return ""
	}
	switch {
	case p == "rpc-publish" || strings.HasPrefix(p, "rpc-"):
		return KindRPC
	case p == "task-publish" || strings.HasPrefix(p, "task-"):
		return KindTask
	default:
		return ""
	}
}

func kindFromHostname(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return ""
	}
	if hostMatchesKind(h, KindRPC, rpcHostSuffix) {
		return KindRPC
	}
	if hostMatchesKind(h, KindTask, taskHostSuffix) {
		return KindTask
	}
	return ""
}

func hostMatchesKind(h, kind string, digitSuffix *regexp.Regexp) bool {
	if strings.Contains(h, kind+"-publish") {
		return true
	}
	if strings.Contains(h, "-"+kind+"-") {
		return true
	}
	if strings.HasPrefix(h, kind+"-") {
		return true
	}
	if strings.HasSuffix(h, "-"+kind) {
		return true
	}
	return digitSuffix != nil && digitSuffix.MatchString(h)
}
