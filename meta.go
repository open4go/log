package log

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync/atomic"
)

type buildMeta struct {
	Image     string
	GitCommit string
	GitBranch string
	BuildTime string
	Instance  string
}

// Optional ldflags, set at compile time:
//
//	-X github.com/open4go/log.ldGitCommit=$(git rev-parse HEAD)
//	-X github.com/open4go/log.ldGitBranch=$(git branch --show-current)
//	-X github.com/open4go/log.ldBuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)
//	-X github.com/open4go/log.ldImage=v0.0.372
//
// Deploy-time environment variables (IMAGE_TAG, GIT_COMMIT, …) take precedence
// so a :pro image retagged by pullbot still reports the running release.
var (
	ldImage     string
	ldGitCommit string
	ldGitBranch string
	ldBuildTime string
)

var meta atomic.Pointer[buildMeta]

func currentMeta() buildMeta {
	if m := meta.Load(); m != nil {
		return *m
	}
	return refreshMeta()
}

func refreshMeta() buildMeta {
	m := loadMeta()
	meta.Store(&m)
	return m
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func loadMeta() buildMeta {
	m := buildMeta{
		// IMAGE_TAG is the contract. SERVER_VERSION is already used by
		// open4go/rest.MakeResponse for the HTTP "version" field; APP_VERSION
		// is a common alias. All of these yield the same "image" log field.
		Image:     firstNonEmpty(os.Getenv("IMAGE_TAG"), os.Getenv("SERVER_VERSION"), os.Getenv("APP_VERSION"), ldImage),
		GitCommit: firstNonEmpty(os.Getenv("GIT_COMMIT"), ldGitCommit),
		GitBranch: firstNonEmpty(os.Getenv("GIT_BRANCH"), ldGitBranch),
		BuildTime: firstNonEmpty(os.Getenv("BUILD_TIME"), ldBuildTime),
		Instance:  strings.TrimSpace(os.Getenv("HOSTNAME")),
	}
	applyBuildInfo(&m)
	if m.Image == "" {
		m.Image = readVersionFile()
	}
	return m
}

func applyBuildInfo(m *buildMeta) {
	info, ok := debug.ReadBuildInfo()
	if !ok || info == nil {
		return
	}
	applyBuildSettings(m, info.Settings)
}

func applyBuildSettings(m *buildMeta, settings []debug.BuildSetting) {
	modified := false
	usedVCS := false
	for _, s := range settings {
		switch s.Key {
		case "vcs.revision":
			if m.GitCommit == "" && strings.TrimSpace(s.Value) != "" {
				m.GitCommit = s.Value
				usedVCS = true
			}
		case "vcs.time":
			if m.BuildTime == "" && strings.TrimSpace(s.Value) != "" {
				m.BuildTime = s.Value
			}
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	// Only mark -dirty when the commit itself came from VCS; an env or
	// ldflags SHA is the snapshot the operator deployed and must stay exact.
	if modified && usedVCS && m.GitCommit != "" && !strings.HasSuffix(m.GitCommit, "-dirty") {
		m.GitCommit += "-dirty"
	}
}

func readVersionFile() string {
	candidates := []string{"VERSION", filepath.Join("/app", "VERSION")}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "VERSION"))
	}
	seen := make(map[string]struct{}, len(candidates))
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		b, err := os.ReadFile(p)
		if err != nil || len(b) == 0 || len(b) > 256 {
			continue
		}
		line := strings.TrimSpace(strings.SplitN(string(b), "\n", 2)[0])
		if line == "" || strings.ContainsRune(line, 0) {
			continue
		}
		return line
	}
	return ""
}
