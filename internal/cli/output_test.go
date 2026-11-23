package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeOutputFixture(t *testing.T, name string, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunDiffOutput(t *testing.T) {
	path := writeOutputFixture(t, "Dockerfile", "FROM nginx:1.27\n")

	stdout, _, err := execute("--diff", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{
		"--- " + path,
		"+++ " + path,
		"@@ -1,1 +1,1 @@",
		"-FROM nginx:1.27",
		"+FROM m.daocloud.io/docker.io/library/nginx:1.27",
	} {
		if !strings.Contains(stdout, part) {
			t.Fatalf("missing %q in %q", part, stdout)
		}
	}
}

func TestRunHelpUsesEnterpriseWording(t *testing.T) {
	_, stderr, err := execute("-h")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "\n策略\n") {
		t.Fatalf("strategy section missing: %q", stderr)
	}
	for _, part := range []string{
		"分析目标路径并输出变更计划",
		"输出统一差异",
		"应用变更至目标文件",
		"策略",
	} {
		if !strings.Contains(stderr, part) {
			t.Fatalf("missing %q in %q", part, stderr)
		}
	}
}
