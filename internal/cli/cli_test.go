package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func execute(args ...string) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(args, &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func TestRunVersion(t *testing.T) {
	stdout, _, err := execute("--version")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.HasPrefix(stdout, "orca ") {
		t.Fatalf("stdout = %q", stdout)
	}
	if strings.Contains(stdout, "(devel)") {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestRunShortVersion(t *testing.T) {
	stdout, _, err := execute("-v")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.HasPrefix(stdout, "orca ") {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestRunShortHelp(t *testing.T) {
	_, stderr, err := execute("-h")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(stderr, "用法") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestRunHelpColorWhenForced(t *testing.T) {
	t.Setenv("FORCE_COLOR", "1")
	_, stderr, err := execute("-h")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(stderr, colorLogo) || !strings.Contains(stderr, colorReset) {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestRunHelpNoColorWins(t *testing.T) {
	t.Setenv("FORCE_COLOR", "1")
	t.Setenv("NO_COLOR", "1")
	_, stderr, err := execute("-h")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.Contains(stderr, "\x1b[") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestRunShortWriteAndStrategy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Dockerfile")
	if err := os.WriteFile(path, []byte("FROM nginx:1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := execute("-w", "-s", "replace", path); err != nil {
		t.Fatal(err)
	}
	want := "FROM docker.m.daocloud.io/library/nginx:1.27\n"
	if got, _ := os.ReadFile(path); string(got) != want {
		t.Fatalf("file = %q; want %q", got, want)
	}
}

func TestVersionStringUsesInjectedValue(t *testing.T) {
	old := version
	version = "v9.9.9-test"
	defer func() { version = old }()

	if got := VersionString(); got != "v9.9.9-test" {
		t.Fatalf("VersionString() = %q", got)
	}
}

func TestRunHelpShowsLogo(t *testing.T) {
	_, stderr, err := execute("--help")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(stderr, "  ___    ____") {
		t.Fatalf("stderr = %q", stderr)
	}
	if !strings.Contains(stderr, "虎鲸镜像加速") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestTerminalLogoTagsAligned(t *testing.T) {
	positions := []int{}
	for _, line := range strings.Split(terminalLogo, "\n") {
		if index := strings.Index(line, "https://"); index >= 0 {
			positions = append(positions, index)
		}
		if index := strings.Index(line, "虎鲸镜像加速"); index >= 0 {
			positions = append(positions, index)
		}
	}
	if len(positions) != 2 || positions[0] != positions[1] {
		t.Fatalf("tag positions = %v", positions)
	}
}

func TestRunRejectsUnknownStrategy(t *testing.T) {
	_, _, err := execute("--strategy", "unknown", "path")
	if err == nil || !strings.Contains(err.Error(), "unknown strategy") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunRequiresPath(t *testing.T) {
	_, stderr, err := execute()
	if err == nil || !strings.Contains(err.Error(), "missing path") {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(stderr, "用法") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestRunDryRunThenWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Dockerfile")
	original := "FROM nginx:1.27\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := execute(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "DRY-RUN strategy=prepend: scanned=1 changed=1 written=0") {
		t.Fatalf("stdout = %q", stdout)
	}
	if got, _ := os.ReadFile(path); string(got) != original {
		t.Fatalf("dry-run changed file: %q", got)
	}

	stdout, _, err = execute("--write", path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "WRITE strategy=prepend: scanned=1 changed=1 written=1") {
		t.Fatalf("stdout = %q", stdout)
	}
	want := "FROM m.daocloud.io/docker.io/library/nginx:1.27\n"
	if got, _ := os.ReadFile(path); string(got) != want {
		t.Fatalf("file = %q; want %q", got, want)
	}

	stdout, _, err = execute("--write", path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "changed=0 written=0") {
		t.Fatalf("second run = %q", stdout)
	}
}

func TestRunReplaceStrategy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "compose.yaml")
	original := "services:\n  web:\n    image: docker.io/library/nginx:1.27\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := execute("--strategy", "replace", "--write", path); err != nil {
		t.Fatal(err)
	}
	want := "services:\n  web:\n    image: docker.m.daocloud.io/library/nginx:1.27\n"
	if got, _ := os.ReadFile(path); string(got) != want {
		t.Fatalf("file = %q; want %q", got, want)
	}
}

func TestRunSkipsScratchAndTemplates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Dockerfile")
	original := "FROM scratch\nFROM ${BASE_IMAGE}\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := execute("--write", path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "scratch (scratch 不是仓库镜像)") {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stdout, "${BASE_IMAGE} (模板变量或非法字符)") {
		t.Fatalf("stdout = %q", stdout)
	}
	if got, _ := os.ReadFile(path); string(got) != original {
		t.Fatalf("file changed: %q", got)
	}
}

func TestRunCheckReturnsErrChangesNeeded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Dockerfile")
	original := "FROM nginx:1.27\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := execute("--check", path)
	if !errors.Is(err, ErrChangesNeeded) {
		t.Fatalf("error = %v; want ErrChangesNeeded", err)
	}
	if !strings.Contains(stdout, "CHECK strategy=prepend: scanned=1 changed=1 written=0") {
		t.Fatalf("stdout = %q", stdout)
	}
	if got, _ := os.ReadFile(path); string(got) != original {
		t.Fatalf("check changed file: %q", got)
	}
}

func TestRunCheckFailsOnMissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	_, stderr, err := execute("--check", missing)
	if err == nil || !strings.Contains(err.Error(), "missing path") {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(stderr, "跳过不存在的路径") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestRunCheckClean(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Dockerfile")
	original := "FROM m.daocloud.io/docker.io/library/nginx:1.27\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := execute("-c", path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "CHECK strategy=prepend: scanned=1 changed=0 written=0") {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestRunCheckRejectsWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Dockerfile")
	if err := os.WriteFile(path, []byte("FROM nginx:1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := execute("--check", "--write", path)
	if err == nil || !strings.Contains(err.Error(), "check cannot be combined with write") {
		t.Fatalf("error = %v", err)
	}
}
