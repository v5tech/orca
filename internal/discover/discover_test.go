package discover

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFilesFiltersSkippedDirsAndSorts(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{
		"Dockerfile", "compose.yml", "notes.txt", "z.yaml",
		".hidden/deployment.yaml", "node_modules/Dockerfile", "dist/compose.yaml", "sub/deployment.yaml",
	} {
		writeTestFile(t, filepath.Join(root, name), "test\n")
	}

	got, err := Files([]string{root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(root, "Dockerfile"),
		filepath.Join(root, "compose.yml"),
		filepath.Join(root, "notes.txt"),
		filepath.Join(root, "sub/deployment.yaml"),
		filepath.Join(root, "z.yaml"),
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("files = %q; want %q", got, want)
	}
}

func TestFilesDeduplicatesPaths(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Dockerfile")
	writeTestFile(t, path, "test\n")

	got, err := Files([]string{root, path}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("files = %q; want one entry", got)
	}
}

func TestFilesWarnsMissingPath(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing")
	warnings := []string{}

	got, err := Files([]string{missing}, func(message string) {
		warnings = append(warnings, message)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("files = %q; want none", got)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], missing) {
		t.Fatalf("warnings = %q", warnings)
	}
}
