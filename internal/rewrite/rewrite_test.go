package rewrite

import (
	"strings"
	"testing"

	"github.com/v5tech/orca/internal/imageref"
)

func TestRewriteDockerfile(t *testing.T) {
	in := "FROM --platform=$BUILDPLATFORM node:20 AS build\nFROM scratch\nFROM ${BASE}\n"
	want := "FROM --platform=$BUILDPLATFORM m.daocloud.io/docker.io/library/node:20 AS build\nFROM scratch\nFROM ${BASE}\n"
	got := Rewrite(in, KindDockerfile, imageref.StrategyPrepend)
	if got.Text != want {
		t.Fatalf("Text = %q; want %q", got.Text, want)
	}
	if got.ChangeCount() != 1 || got.SkipCount() != 2 {
		t.Fatalf("changes=%d skips=%d", got.ChangeCount(), got.SkipCount())
	}
}

func TestRewriteComposeAndKubernetes(t *testing.T) {
	in := "services:\n  web:\n    image: \"nginx:1.27\"  # app\n"
	want := "services:\n  web:\n    image: \"m.daocloud.io/docker.io/library/nginx:1.27\"  # app\n"
	got := Rewrite(in, KindCompose, imageref.StrategyPrepend)
	if got.Text != want {
		t.Fatalf("Text = %q; want %q", got.Text, want)
	}

	k8s := "spec:\n  containers:\n    - image: quay.io/prometheus/prometheus:v2.55.0\n"
	got = Rewrite(k8s, KindKubernetes, imageref.StrategyPrepend)
	if !strings.Contains(got.Text, "m.daocloud.io/quay.io/prometheus/prometheus:v2.55.0") {
		t.Fatalf("Text = %q", got.Text)
	}
}

func TestRewritePreservesNewlines(t *testing.T) {
	crlf := "FROM nginx:1.27\r\n"
	got := Rewrite(crlf, KindDockerfile, imageref.StrategyPrepend)
	if got.Text != "FROM m.daocloud.io/docker.io/library/nginx:1.27\r\n" {
		t.Fatalf("CRLF Text = %q", got.Text)
	}

	noEOL := "FROM nginx:1.27"
	got = Rewrite(noEOL, KindDockerfile, imageref.StrategyPrepend)
	if got.Text != "FROM m.daocloud.io/docker.io/library/nginx:1.27" {
		t.Fatalf("no-EOL Text = %q", got.Text)
	}
}

func TestRewriteIsIdempotent(t *testing.T) {
	original := "FROM nginx:1.27\n"
	once := Rewrite(original, KindDockerfile, imageref.StrategyPrepend)
	twice := Rewrite(once.Text, KindDockerfile, imageref.StrategyPrepend)
	if twice.Text != once.Text || twice.ChangeCount() != 0 {
		t.Fatalf("idempotence failed: %q, changes=%d", twice.Text, twice.ChangeCount())
	}
}

func BenchmarkRewrite(b *testing.B) {
	text := strings.Repeat("FROM --platform=linux/amd64 nginx:1.27 AS build\nFROM scratch\n", 200)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Rewrite(text, KindDockerfile, imageref.StrategyPrepend)
	}
}
