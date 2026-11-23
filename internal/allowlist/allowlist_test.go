package allowlist

import (
	"reflect"
	"strings"
	"testing"
)

const sampleAllows = `# comment
docker.io/*
library/nginx
ghcr.io/foo/**
quay.io/prometheus/*
`

const sampleREADME = `## 支持前缀替换的 Registry (不推荐)

| 源站 | 替换为 | 备注 |
| --- | --- | --- |
| docker.io | docker.m.daocloud.io | |
| ghcr.io | ghcr.m.daocloud.io | |
`

func TestParseAllowsTxt(t *testing.T) {
	got := ParseAllowsTxt(sampleAllows)
	want := []string{"docker.io", "ghcr.io", "quay.io"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("hosts = %q; want %q", got, want)
	}
}

func TestParseReplacements(t *testing.T) {
	got := ParseReplacements(sampleREADME)
	want := map[string]string{
		"docker.io": "docker.m.daocloud.io",
		"ghcr.io":   "ghcr.m.daocloud.io",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("replacements = %q; want %q", got, want)
	}
}

func TestParseReplacementsOnlyOfficialSection(t *testing.T) {
	content := `## 不相关表格

| example.net | fake.m.daocloud.io |

## 支持前缀替换的 Registry (不推荐)

| 源站 | 替换为 |
| --- | --- |
| docker.io | docker.m.daocloud.io |
| example.net | other.example.com |

## 另一个表格

| example.org | fake.m.daocloud.io |
`
	got := ParseReplacements(content)
	want := map[string]string{"docker.io": "docker.m.daocloud.io"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("replacements = %q; want %q", got, want)
	}
}

func TestBuildAddsReplacementSources(t *testing.T) {
	data := Build(sampleAllows, sampleREADME, "testref")
	if _, ok := data.Supported["ghcr.io"]; !ok {
		t.Fatalf("ghcr.io missing from supported: %q", data.Supported)
	}
	if data.Replacements["docker.io"] != "docker.m.daocloud.io" {
		t.Fatalf("replacements = %q", data.Replacements)
	}
}

func TestRenderGo(t *testing.T) {
	data := Build(sampleAllows, sampleREADME, "testref")
	code := RenderGo(data)
	for _, part := range []string{
		"package imageref",
		`"docker.io": "docker.m.daocloud.io"`,
		`"docker.io": {},`,
		"testref",
	} {
		if !strings.Contains(code, part) {
			t.Fatalf("missing %q in generated code:\n%s", part, code)
		}
	}
}
