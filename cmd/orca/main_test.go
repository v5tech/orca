package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, name string, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunSuccess(t *testing.T) {
	path := writeFixture(t, "Dockerfile", "FROM m.daocloud.io/docker.io/library/nginx:1.27\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "changed=0") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunErrorExitCode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"--strategy", "bad", "path"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "orca:") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
