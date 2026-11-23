package cli

import (
	"encoding/json"
	"errors"
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

func TestRunJSONOutput(t *testing.T) {
	path := writeOutputFixture(t, "Dockerfile", "FROM nginx:1.27\n")

	stdout, _, err := execute("--json", path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stdout, "{") {
		t.Fatalf("stdout = %q", stdout)
	}

	var report jsonReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	if report.Mode != "DRY-RUN" || report.Scanned != 1 || report.Changed != 1 {
		t.Fatalf("report = %+v", report)
	}
	if len(report.Events) != 1 || report.Events[0].Kind != "change" {
		t.Fatalf("events = %+v", report.Events)
	}
	if len(report.Files) != 1 || !report.Files[0].Changed || report.Files[0].Written {
		t.Fatalf("files = %+v", report.Files)
	}
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

func TestRunJSONWithDiff(t *testing.T) {
	path := writeOutputFixture(t, "Dockerfile", "FROM nginx:1.27\n")

	stdout, _, err := execute("--json", "--diff", path)
	if err != nil {
		t.Fatal(err)
	}
	var report jsonReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Files) != 1 || !strings.Contains(report.Files[0].Diff, "@@ -1,1 +1,1 @@") {
		t.Fatalf("files = %+v", report.Files)
	}
}

func TestRunJSONCheckReturnsErrChangesNeeded(t *testing.T) {
	path := writeOutputFixture(t, "Dockerfile", "FROM nginx:1.27\n")

	stdout, _, err := execute("--json", "--check", path)
	if !errors.Is(err, ErrChangesNeeded) {
		t.Fatalf("error = %v; want ErrChangesNeeded", err)
	}
	var report jsonReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	if report.Mode != "CHECK" || report.Changed != 1 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunJSONWrite(t *testing.T) {
	path := writeOutputFixture(t, "Dockerfile", "FROM nginx:1.27\n")

	stdout, _, err := execute("--json", "--write", path)
	if err != nil {
		t.Fatal(err)
	}
	var report jsonReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	if report.Mode != "WRITE" || report.Written != 1 {
		t.Fatalf("report = %+v", report)
	}
	if len(report.Files) != 1 || !report.Files[0].Written {
		t.Fatalf("files = %+v", report.Files)
	}
}

func TestRunHelpShowsJSONAndDiffFlags(t *testing.T) {
	_, stderr, err := execute("-h")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "-j, --json") || !strings.Contains(stderr, "-d, --diff") {
		t.Fatalf("stderr = %q", stderr)
	}
}
