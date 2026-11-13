package log

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"
)

func TestInitRereadsImageTagAndGitCommit(t *testing.T) {
	// Package-level meta is computed at import, before this env is set.
	// Init must pick the new values up so compose-injected IMAGE_TAG is not sticky.
	t.Setenv("IMAGE_TAG", "v0.0.372")
	t.Setenv("GIT_COMMIT", "abc1234def")
	t.Setenv("GIT_BRANCH", "main")
	t.Setenv("BUILD_TIME", "2026-11-14T01:00:00Z")
	t.Setenv("HOSTNAME", "share-stack-feedback-1")

	var buf bytes.Buffer
	Init("debug", &buf)
	t.Cleanup(func() { Init("debug", nil) })

	Log(context.Background()).Info("hello")
	rec := decodeLog(t, &buf)
	if rec["image"] != "v0.0.372" {
		t.Fatalf("image=%v want v0.0.372", rec["image"])
	}
	if rec["git_commit"] != "abc1234def" {
		t.Fatalf("git_commit=%v", rec["git_commit"])
	}
	if rec["git_branch"] != "main" {
		t.Fatalf("git_branch=%v", rec["git_branch"])
	}
	if rec["build_time"] != "2026-11-14T01:00:00Z" {
		t.Fatalf("build_time=%v", rec["build_time"])
	}
	if rec["instance"] != "share-stack-feedback-1" {
		t.Fatalf("instance=%v", rec["instance"])
	}
}

func TestServerVersionFallbackWhenImageTagEmpty(t *testing.T) {
	t.Setenv("IMAGE_TAG", "")
	t.Setenv("SERVER_VERSION", "v0.0.359")
	t.Setenv("APP_VERSION", "should-not-win")
	old := ldImage
	ldImage = "ld-should-not-win"
	t.Cleanup(func() { ldImage = old })

	m := loadMeta()
	if m.Image != "v0.0.359" {
		t.Fatalf("image=%q want SERVER_VERSION", m.Image)
	}
}

func TestEnvWinsOverLdflags(t *testing.T) {
	t.Setenv("IMAGE_TAG", "v-env")
	t.Setenv("GIT_COMMIT", "env-sha")
	oldI, oldC := ldImage, ldGitCommit
	ldImage, ldGitCommit = "v-ld", "ld-sha"
	t.Cleanup(func() { ldImage, ldGitCommit = oldI, oldC })

	m := loadMeta()
	if m.Image != "v-env" {
		t.Fatalf("image=%q want v-env", m.Image)
	}
	if m.GitCommit != "env-sha" {
		t.Fatalf("git_commit=%q want env-sha", m.GitCommit)
	}
}

func TestLdflagsFallbackWhenEnvEmpty(t *testing.T) {
	t.Setenv("IMAGE_TAG", "")
	t.Setenv("SERVER_VERSION", "")
	t.Setenv("APP_VERSION", "")
	t.Setenv("GIT_COMMIT", "")
	t.Setenv("GIT_BRANCH", "")
	t.Setenv("BUILD_TIME", "")
	oldI, oldC, oldB, oldT := ldImage, ldGitCommit, ldGitBranch, ldBuildTime
	ldImage, ldGitCommit, ldGitBranch, ldBuildTime = "v-ld", "deadbeef", "release", "2026-01-01T00:00:00Z"
	t.Cleanup(func() {
		ldImage, ldGitCommit, ldGitBranch, ldBuildTime = oldI, oldC, oldB, oldT
	})

	m := loadMeta()
	if m.Image != "v-ld" {
		t.Fatalf("image=%q", m.Image)
	}
	if m.GitCommit != "deadbeef" {
		t.Fatalf("git_commit=%q", m.GitCommit)
	}
	if m.GitBranch != "release" {
		t.Fatalf("git_branch=%q", m.GitBranch)
	}
	if m.BuildTime != "2026-01-01T00:00:00Z" {
		t.Fatalf("build_time=%q", m.BuildTime)
	}
}

func TestGitCommitFallbackFromBuildInfo(t *testing.T) {
	t.Setenv("GIT_COMMIT", "")
	t.Setenv("BUILD_TIME", "")
	oldC, oldT := ldGitCommit, ldBuildTime
	ldGitCommit, ldBuildTime = "", ""
	t.Cleanup(func() { ldGitCommit, ldBuildTime = oldC, oldT })

	m := loadMeta()
	// go test from a git checkout typically embeds vcs.revision. If the
	// toolchain was invoked with -buildvcs=false this is a skip, not a fail.
	if m.GitCommit == "" {
		t.Log("BuildInfo has no vcs.revision (likely -buildvcs=false); fallback path is still unit-tested via applyBuildInfo settings")
		return
	}
	// Accept a raw SHA or SHA-dirty when the worktree has local edits.
	got := strings.TrimSuffix(m.GitCommit, "-dirty")
	if len(got) < 7 {
		t.Fatalf("git_commit=%q looks too short for a vcs.revision", m.GitCommit)
	}
	for _, c := range got {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			t.Fatalf("git_commit=%q is not a hex SHA", m.GitCommit)
		}
	}
}

func TestVersionFileFallback(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("VERSION", []byte("v9.9.9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IMAGE_TAG", "")
	t.Setenv("SERVER_VERSION", "")
	t.Setenv("APP_VERSION", "")
	old := ldImage
	ldImage = ""
	t.Cleanup(func() { ldImage = old })

	m := loadMeta()
	if m.Image != "v9.9.9" {
		t.Fatalf("image=%q want v9.9.9 from VERSION file (cwd=%s)", m.Image, dir)
	}
}

func TestVersionFileIgnoredWhenTooLargeOrEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("VERSION", []byte(strings.Repeat("x", 300)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IMAGE_TAG", "")
	t.Setenv("SERVER_VERSION", "")
	t.Setenv("APP_VERSION", "")
	old := ldImage
	ldImage = ""
	t.Cleanup(func() { ldImage = old })

	if got := readVersionFile(); got != "" {
		// readVersionFile also looks at /app/VERSION and the test binary dir,
		// which should not exist or should also be empty; if a leftover
		// VERSION sits next to the test binary this could be non-empty.
		exe, _ := os.Executable()
		t.Logf("readVersionFile=%q exe=%s", got, filepath.Dir(exe))
		if got == strings.Repeat("x", 300) {
			t.Fatalf("oversized VERSION file was accepted")
		}
	}
}

func TestIsKnownWrapper(t *testing.T) {
	tests := []struct {
		fn   string
		want bool
	}{
		{"github.com/open4go/rest.MakeError", true},
		{"github.com/open4go/rest.MakeError.func1", true},
		{"github.com/open4go/rest.MakeResponse", true},
		{"github.com/open4go/rest.MakeResponse.func1", true},
		{"github.com/open4go/rest.parseQuery", false},
		{"github.com/open4go/rest.(*Query).Parse", false},
		{"github.com/open4go/middle.JWTAuthMiddleware.func1", false},
		{"github.com/r2day/feedback-api/handler.Create", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isKnownWrapper(runtime.Frame{Function: tt.fn}); got != tt.want {
			t.Errorf("isKnownWrapper(%q)=%v want %v", tt.fn, got, tt.want)
		}
	}
}

func TestCallerReportsThisCallSite(t *testing.T) {
	_, _, line, _ := runtime.Caller(0)
	got := Caller(0)
	want := "log/meta_test.go:" + strconv.Itoa(line+1)
	if got != want {
		t.Fatalf("Caller(0)=%q want %q", got, want)
	}
}

func TestErrorHelpersReportServiceCallSite(t *testing.T) {
	var buf bytes.Buffer
	Init("debug", &buf)
	t.Cleanup(func() { Init("debug", nil) })

	err := errors.New("save failed")

	_, _, line, _ := runtime.Caller(0)
	Error(context.Background(), err, "save order")
	rec := decodeLog(t, &buf)
	want := "log/meta_test.go:" + strconv.Itoa(line+1)
	if rec["caller"] != want {
		t.Fatalf("Error caller=%q want %q", rec["caller"], want)
	}

	buf.Reset()
	_, _, line, _ = runtime.Caller(0)
	Errorf(context.Background(), err, "save %s", "order")
	rec = decodeLog(t, &buf)
	want = "log/meta_test.go:" + strconv.Itoa(line+1)
	if rec["caller"] != want {
		t.Fatalf("Errorf caller=%q want %q", rec["caller"], want)
	}

	buf.Reset()
	_, _, line, _ = runtime.Caller(0)
	ErrorWithStack(context.Background(), err, "save order")
	rec = decodeLog(t, &buf)
	want = "log/meta_test.go:" + strconv.Itoa(line+1)
	if rec["caller"] != want {
		t.Fatalf("ErrorWithStack caller=%q want %q", rec["caller"], want)
	}
	stack, _ := rec["stacktrace"].(string)
	if !strings.Contains(stack, "TestErrorHelpersReportServiceCallSite") {
		t.Fatalf("stack missing test function:\n%s", stack)
	}
}

func TestImageAndCallerOnErrorShape(t *testing.T) {
	t.Setenv("IMAGE_TAG", "v0.0.372")
	t.Setenv("GIT_COMMIT", "abc1234")
	var buf bytes.Buffer
	Init("debug", &buf)
	t.Cleanup(func() { Init("debug", nil) })

	Error(context.Background(), errors.New("disk full"), "save failed")
	line := strings.TrimSpace(buf.String())
	for _, w := range []string{
		`"level":"error"`,
		`"msg":"save failed"`,
		`"error":"disk full"`,
		`"caller":"log/meta_test.go:`,
		`"image":"v0.0.372"`,
		`"git_commit":"abc1234"`,
	} {
		if !strings.Contains(line, w) {
			t.Fatalf("missing %s in %s", w, line)
		}
	}
	if strings.Contains(line, "stacktrace") {
		t.Fatalf("plain error must not carry a stacktrace: %s", line)
	}
}

func TestApplyBuildSettingsFillsCommitAndTime(t *testing.T) {
	m := buildMeta{}
	applyBuildSettings(&m, []debug.BuildSetting{
		{Key: "vcs.revision", Value: "0123456789abcdef"},
		{Key: "vcs.time", Value: "2026-11-14T01:15:00Z"},
		{Key: "vcs.modified", Value: "true"},
	})
	if m.GitCommit != "0123456789abcdef-dirty" {
		t.Fatalf("git_commit=%q", m.GitCommit)
	}
	if m.BuildTime != "2026-11-14T01:15:00Z" {
		t.Fatalf("build_time=%q", m.BuildTime)
	}

	// Env / ldflags already set: BuildInfo must not overwrite, and must
	// not append -dirty to a SHA that did not come from VCS.
	filled := buildMeta{GitCommit: "env-sha", BuildTime: "env-time"}
	applyBuildSettings(&filled, []debug.BuildSetting{
		{Key: "vcs.revision", Value: "other"},
		{Key: "vcs.time", Value: "other-time"},
		{Key: "vcs.modified", Value: "true"},
	})
	if filled.GitCommit != "env-sha" || filled.BuildTime != "env-time" {
		t.Fatalf("overwrote existing: %+v", filled)
	}
}
