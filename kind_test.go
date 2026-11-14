package log

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestResolveKind(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		viper map[string]string
		host  string
		want  string
	}{
		{name: "default api", want: KindAPI},
		{name: "empty host and env", host: "", want: KindAPI},

		// Explicit env
		{name: "SERVER_KIND rpc", env: map[string]string{"SERVER_KIND": "rpc"}, want: KindRPC},
		{name: "SERVER_KIND task trimmed case", env: map[string]string{"SERVER_KIND": " Task "}, want: KindTask},
		{name: "LOG_KIND api", env: map[string]string{"LOG_KIND": "API"}, want: KindAPI},
		{name: "SERVER_KIND wins over LOG_KIND", env: map[string]string{"SERVER_KIND": "rpc", "LOG_KIND": "task"}, want: KindRPC},
		{name: "invalid SERVER_KIND falls through to LOG_KIND", env: map[string]string{"SERVER_KIND": "worker", "LOG_KIND": "task"}, want: KindTask},
		{name: "invalid explicit ignored, default api", env: map[string]string{"SERVER_KIND": "worker", "LOG_KIND": "foo"}, want: KindAPI},

		// Viper
		{name: "viper server.kind", viper: map[string]string{"server.kind": "rpc"}, want: KindRPC},
		{name: "viper log.kind", viper: map[string]string{"log.kind": "task"}, want: KindTask},
		{name: "viper server.kind wins over log.kind", viper: map[string]string{"server.kind": "api", "log.kind": "rpc"}, want: KindAPI},
		{name: "env wins over viper", env: map[string]string{"LOG_KIND": "task"}, viper: map[string]string{"server.kind": "rpc"}, want: KindTask},
		{name: "invalid viper ignored", viper: map[string]string{"server.kind": "worker"}, want: KindAPI},

		// Compose project
		{name: "compose rpc-publish", env: map[string]string{"COMPOSE_PROJECT_NAME": "rpc-publish"}, want: KindRPC},
		{name: "compose rpc- prefix", env: map[string]string{"COMPOSE_PROJECT_NAME": "rpc-foo"}, want: KindRPC},
		{name: "compose task-publish", env: map[string]string{"COMPOSE_PROJECT_NAME": "task-publish"}, want: KindTask},
		{name: "compose task- prefix", env: map[string]string{"COMPOSE_PROJECT_NAME": "task-jobs"}, want: KindTask},
		{name: "compose share-stack is api", env: map[string]string{"COMPOSE_PROJECT_NAME": "share-stack"}, want: KindAPI},
		{name: "compose mini is api", env: map[string]string{"COMPOSE_PROJECT_NAME": "mini"}, want: KindAPI},
		{name: "compose static-stack is api", env: map[string]string{"COMPOSE_PROJECT_NAME": "static-stack"}, want: KindAPI},
		{name: "compose case insensitive", env: map[string]string{"COMPOSE_PROJECT_NAME": "RPC-Publish"}, want: KindRPC},
		{name: "explicit overrides compose", env: map[string]string{"SERVER_KIND": "api", "COMPOSE_PROJECT_NAME": "rpc-publish"}, want: KindAPI},

		// Hostnames (production container names)
		{name: "host share-stack-order-1", host: "share-stack-order-1", want: KindAPI},
		{name: "host mini-order-4-1", host: "mini-order-4-1", want: KindAPI},
		{name: "host static-stack-fileserver-1", host: "static-stack-fileserver-1", want: KindAPI},
		{name: "host share-stack-auth-1 is not task", host: "share-stack-auth-1", want: KindAPI},
		{name: "host rpc-publish-order-rpc-1", host: "rpc-publish-order-rpc-1", want: KindRPC},
		{name: "host rpc-publish-pay-rpc-1", host: "rpc-publish-pay-rpc-1", want: KindRPC},
		{name: "host rpc-publish-ai-rpc-1", host: "rpc-publish-ai-rpc-1", want: KindRPC},
		{name: "host task-publish-order-task-1", host: "task-publish-order-task-1", want: KindTask},
		{name: "host task-publish-auth-task-1", host: "task-publish-auth-task-1", want: KindTask},
		{name: "host contains -rpc-", host: "order-rpc-worker", want: KindRPC},
		{name: "host prefix rpc-", host: "rpc-order-1", want: KindRPC},
		{name: "host suffix -rpc", host: "order-rpc", want: KindRPC},
		{name: "host suffix -rpc-digits", host: "order-rpc-1", want: KindRPC},
		{name: "host contains -task-", host: "order-task-worker", want: KindTask},
		{name: "host prefix task-", host: "task-order-1", want: KindTask},
		{name: "host suffix -task", host: "order-task", want: KindTask},
		{name: "host suffix -task-digits", host: "order-task-2", want: KindTask},
		{name: "host case insensitive", host: "RPC-Publish-Order-RPC-1", want: KindRPC},
		{name: "compose wins over hostname", env: map[string]string{"COMPOSE_PROJECT_NAME": "task-publish"}, host: "rpc-publish-order-rpc-1", want: KindTask},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range []string{"server.kind", "log.kind"} {
				viper.Set(k, nil)
			}
			for k, v := range tt.viper {
				viper.Set(k, v)
			}
			t.Cleanup(func() {
				for _, k := range []string{"server.kind", "log.kind"} {
					viper.Set(k, nil)
				}
			})

			getenv := func(key string) string {
				if tt.env == nil {
					return ""
				}
				return tt.env[key]
			}
			got := resolveKind(getenv, tt.host)
			if got != tt.want {
				t.Fatalf("resolveKind()=%q want %q", got, tt.want)
			}
		})
	}
}

func TestKindCachedOnInit(t *testing.T) {
	t.Setenv("SERVER_KIND", "rpc")
	t.Setenv("COMPOSE_PROJECT_NAME", "share-stack")
	prev := hostnameLookup
	hostnameLookup = func() (string, error) { return "task-publish-order-task-1", nil }
	t.Cleanup(func() {
		hostnameLookup = prev
		_ = os.Unsetenv("SERVER_KIND")
		_ = os.Unsetenv("COMPOSE_PROJECT_NAME")
		refreshKind()
	})

	var buf bytes.Buffer
	Init("info", &buf)
	if Kind() != KindRPC {
		t.Fatalf("Kind()=%q want rpc", Kind())
	}
	Log(context.Background()).Info("hello")
	rec := decodeLog(t, &buf)
	if rec["kind"] != KindRPC {
		t.Fatalf("kind field=%v want rpc, line=%s", rec["kind"], buf.String())
	}

	// A later env change is picked up on the next Init, same as image meta.
	t.Setenv("SERVER_KIND", "task")
	buf.Reset()
	Init("info", &buf)
	if Kind() != KindTask {
		t.Fatalf("Kind after re-Init=%q want task", Kind())
	}
	Log(context.Background()).Info("hello")
	rec = decodeLog(t, &buf)
	if rec["kind"] != KindTask {
		t.Fatalf("kind after re-Init=%v", rec["kind"])
	}
}

func TestKindFromHostnameHookDoesNotUseRealHost(t *testing.T) {
	// Clear every explicit source so only the injected hostname is used.
	t.Setenv("SERVER_KIND", "")
	t.Setenv("LOG_KIND", "")
	t.Setenv("COMPOSE_PROJECT_NAME", "")
	viper.Set("server.kind", nil)
	viper.Set("log.kind", nil)
	prev := hostnameLookup
	hostnameLookup = func() (string, error) { return "share-stack-auth-1", nil }
	t.Cleanup(func() {
		hostnameLookup = prev
		refreshKind()
	})
	refreshKind()
	if Kind() != KindAPI {
		t.Fatalf("share-stack-auth-1 must be api, got %q", Kind())
	}

	hostnameLookup = func() (string, error) { return "rpc-publish-order-rpc-1", nil }
	refreshKind()
	if Kind() != KindRPC {
		t.Fatalf("injected rpc host: Kind()=%q", Kind())
	}
}
