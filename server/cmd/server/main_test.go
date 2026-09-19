package main

import (
	"strings"
	"testing"

	"github.com/jerion/wecom-auth-center/server/internal/buildinfo"
)

func TestVersionText(t *testing.T) {
	text := versionText()
	for _, want := range []string{"wecom-auth-center", buildinfo.Version, buildinfo.Commit, buildinfo.BuildDate} {
		if !strings.Contains(text, want) {
			t.Errorf("版本信息缺少 %q: %q", want, text)
		}
	}
}

func TestVersionTextWithInjectedBuildInfo(t *testing.T) {
	origin := buildinfo.Version
	buildinfo.Version = "v9.9.9"
	defer func() { buildinfo.Version = origin }()

	text := versionText()
	if !strings.Contains(text, "v9.9.9") {
		t.Errorf("版本信息未包含注入的版本号: %q", text)
	}
}
