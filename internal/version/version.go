package version

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

// Build-time metadata injected via -ldflags "-X walrus/internal/version.<Var>=...".
var (
	Version    = "dev"
	Commit     = "none"
	BuildTime  = "unknown"
	GitHubRepo = "keyborad-man/walrus"
	ImageRepo  = "ghcr.io/keyborad-man/walrus"
)

// Info describes the running Walrus binary build.
type Info struct {
	Version    string `json:"version"`
	Commit     string `json:"commit"`
	BuildTime  string `json:"build_time"`
	GoVersion  string `json:"go_version"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	GitHubRepo string `json:"github_repo"`
	ImageRepo  string `json:"image_repo"`
}

// Current returns the build metadata of the running process.
func Current() Info {
	v := strings.TrimSpace(Version)
	if v == "" {
		v = "dev"
	}
	c := strings.TrimSpace(Commit)
	if c == "" {
		c = "none"
	}
	bt := strings.TrimSpace(BuildTime)
	if bt == "" {
		bt = "unknown"
	}
	return Info{
		Version:    v,
		Commit:     c,
		BuildTime:  bt,
		GoVersion:  runtime.Version(),
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		GitHubRepo: strings.TrimSpace(GitHubRepo),
		ImageRepo:  strings.TrimSpace(ImageRepo),
	}
}

// Summary returns a single-line human-readable version banner.
func Summary() string {
	info := Current()
	return fmt.Sprintf("walrus %s (commit %s, built %s, %s/%s)", info.Version, info.Commit, info.BuildTime, info.OS, info.Arch)
}

type semver struct {
	major int
	minor int
	patch int
	pre   string
}

func parseSemver(raw string) (semver, bool) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")
	if s == "" {
		return semver{}, false
	}
	if idx := strings.IndexByte(s, '+'); idx >= 0 {
		s = s[:idx]
	}
	var pre string
	if idx := strings.IndexByte(s, '-'); idx >= 0 {
		pre = s[idx+1:]
		s = s[:idx]
	}
	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return semver{}, false
	}
	maj, err := strconv.Atoi(parts[0])
	if err != nil || maj < 0 {
		return semver{}, false
	}
	min, err := strconv.Atoi(parts[1])
	if err != nil || min < 0 {
		return semver{}, false
	}
	patch := 0
	if len(parts) == 3 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil || patch < 0 {
			return semver{}, false
		}
	}
	return semver{major: maj, minor: min, patch: patch, pre: pre}, true
}

// IsSemver reports whether raw is a valid semantic version string (with or without leading "v").
func IsSemver(raw string) bool {
	_, ok := parseSemver(raw)
	return ok
}

// CompareSemver compares two semantic versions a and b.
// It returns -1 if a < b, 0 if a == b, +1 if a > b, and ok=false if either is not semver.
func CompareSemver(a, b string) (int, bool) {
	sa, okA := parseSemver(a)
	sb, okB := parseSemver(b)
	if !okA || !okB {
		return 0, false
	}
	if sa.major != sb.major {
		if sa.major < sb.major {
			return -1, true
		}
		return 1, true
	}
	if sa.minor != sb.minor {
		if sa.minor < sb.minor {
			return -1, true
		}
		return 1, true
	}
	if sa.patch != sb.patch {
		if sa.patch < sb.patch {
			return -1, true
		}
		return 1, true
	}
	// Release > prerelease
	if sa.pre == sb.pre {
		return 0, true
	}
	if sa.pre == "" {
		return 1, true
	}
	if sb.pre == "" {
		return -1, true
	}
	if sa.pre < sb.pre {
		return -1, true
	}
	return 1, true
}
