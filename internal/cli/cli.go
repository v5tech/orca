// Package cli implements the orca command-line interface.
package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/v5tech/orca/internal/discover"
	"github.com/v5tech/orca/internal/imageref"
	"github.com/v5tech/orca/internal/rewrite"
)

const terminalLogo = `  ___    ____     ____      _
 / _ \  |  _ \   / ___|    / \
| | | | | |_) | | |       / _ \
| |_| | |  _ <  | |___   / ___ \    https://github.com/v5tech/orca
 \___/  |_| \_\  \____| /_/   \_\   虎鲸镜像加速 · 跨平台镜像地址重写工具`

// ErrChangesNeeded indicates that check mode found image references to rewrite.
var ErrChangesNeeded = errors.New("changes needed")

const (
	colorReset = "\x1b[0m"
	colorLogo  = "\x1b[96m"
	colorHead  = "\x1b[1;36m"
	colorBody  = "\x1b[36m"
)

func readFileWithMode(path string) ([]byte, os.FileMode, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, 0, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, 0, err
	}
	return data, info.Mode(), nil
}

func colorEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func paint(enabled bool, code string, text string) string {
	if !enabled {
		return text
	}
	return code + text + colorReset
}

type summary struct {
	scanned int
	changed int
	written int
	skips   int
}

type jsonEvent struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Kind   string `json:"kind"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
	Image  string `json:"image,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type jsonFile struct {
	Path    string `json:"path"`
	Changed bool   `json:"changed"`
	Written bool   `json:"written"`
	Diff    string `json:"diff,omitempty"`
}

type jsonReport struct {
	Mode          string      `json:"mode"`
	Strategy      string      `json:"strategy"`
	Scanned       int         `json:"scanned"`
	Changed       int         `json:"changed"`
	Written       int         `json:"written"`
	SkippedValues int         `json:"skipped_values"`
	Events        []jsonEvent `json:"events"`
	Files         []jsonFile  `json:"files"`
}

func splitDiffLines(text string) []string {
	lines := strings.SplitAfter(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	}
	return lines
}

func renderUnifiedDiff(path string, original string, updated string, events []rewrite.Event) string {
	changed := []int{}
	for _, event := range events {
		if event.IsChange() {
			changed = append(changed, event.Line)
		}
	}
	if len(changed) == 0 {
		return ""
	}

	before := splitDiffLines(original)
	after := splitDiffLines(updated)
	var output strings.Builder
	output.WriteString("--- " + path + "\n")
	output.WriteString("+++ " + path + "\n")

	for i := 0; i < len(changed); {
		start := changed[i]
		end := start
		for i+1 < len(changed) && changed[i+1] == end+1 {
			i++
			end = changed[i]
		}
		i++
		count := end - start + 1
		fmt.Fprintf(&output, "@@ -%d,%d +%d,%d @@\n", start, count, start, count)
		for line := start; line <= end; line++ {
			if line-1 < len(before) {
				output.WriteString("-" + before[line-1] + "\n")
			}
			if line-1 < len(after) {
				output.WriteString("+" + after[line-1] + "\n")
			}
		}
	}
	return output.String()
}

// Run executes the CLI with the given arguments.
func Run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("orca", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var showVersion bool
	var showHelp bool
	var write bool
	var check bool
	var asJSON bool
	var showDiff bool
	var strategyValue string

	flags.BoolVar(&showVersion, "version", false, "显示版本并退出")
	flags.BoolVar(&showVersion, "v", false, "显示版本并退出")
	flags.BoolVar(&showHelp, "help", false, "显示帮助")
	flags.BoolVar(&showHelp, "h", false, "显示帮助")
	flags.BoolVar(&write, "write", false, "写回文件")
	flags.BoolVar(&write, "w", false, "写回文件")
	flags.BoolVar(&check, "check", false, "检查是否存在需要改写的镜像")
	flags.BoolVar(&check, "c", false, "检查是否存在需要改写的镜像")
	flags.BoolVar(&asJSON, "json", false, "以 JSON 格式输出结果")
	flags.BoolVar(&asJSON, "j", false, "以 JSON 格式输出结果")
	flags.BoolVar(&showDiff, "diff", false, "显示改写差异")
	flags.BoolVar(&showDiff, "d", false, "显示改写差异")
	flags.StringVar(&strategyValue, "strategy", string(imageref.StrategyPrepend), "镜像策略：prepend 或 replace")
	flags.StringVar(&strategyValue, "s", string(imageref.StrategyPrepend), "镜像策略：prepend 或 replace")

	flags.Usage = func() {
		colored := colorEnabled(stderr)
		fmt.Fprintln(stderr, paint(colored, colorLogo, terminalLogo))
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, paint(colored, colorHead, "用法"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  orca <路径>...                      预览镜像改写，不修改文件"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  orca -c <路径>...                   检查是否需要改写"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  orca -d <路径>...                   显示改写差异"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  orca -j <路径>...                   JSON 输出"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  orca -w <路径>...                   写回修改"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  orca -s replace <路径>...           使用官方前缀替换表"))
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, paint(colored, colorHead, "选项"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  -w, --write                         写回文件；默认只预览"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  -c, --check                         检查模式；发现改动时返回非零退出码"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  -d, --diff                          显示改写差异"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  -j, --json                          输出 JSON 报告"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  -s, --strategy prepend|replace      镜像策略：prepend 或 replace"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  -v, --version                       显示版本"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  -h, --help                          显示帮助"))
	}

	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if showHelp {
		flags.Usage()
		return nil
	}

	if showVersion {
		fmt.Fprintf(stdout, "orca %s\n", VersionString())
		return nil
	}

	if check && write {
		return fmt.Errorf("check cannot be combined with write")
	}

	strategy := imageref.Strategy(strategyValue)
	if strategy != imageref.StrategyPrepend && strategy != imageref.StrategyReplace {
		return fmt.Errorf("unknown strategy %q: use prepend or replace", strategyValue)
	}

	if flags.NArg() == 0 {
		flags.Usage()
		return fmt.Errorf("missing path")
	}

	missingPaths := 0
	files, err := discover.Files(flags.Args(), func(message string) {
		missingPaths++
		fmt.Fprintln(stderr, message)
	})
	if err != nil {
		return err
	}

	total := summary{}
	label := "DRY-RUN"
	if check {
		label = "CHECK"
	}
	if write {
		label = "WRITE"
	}
	report := jsonReport{
		Mode:     label,
		Strategy: string(strategy),
		Events:   []jsonEvent{},
		Files:    []jsonFile{},
	}
	details := &strings.Builder{}
	diffs := &strings.Builder{}
	for _, path := range files {
		kind := rewrite.DetectKind(filepath.Base(path))
		if kind == "" {
			continue
		}

		original, mode, err := readFileWithMode(path)
		if err != nil {
			return err
		}

		result := rewrite.Rewrite(string(original), kind, strategy)
		total.scanned++
		total.skips += result.SkipCount()
		fileReport := jsonFile{Path: path}

		for _, event := range result.Events {
			report.Events = append(report.Events, jsonEvent{
				Path:   path,
				Line:   event.Line,
				Kind:   event.Kind,
				Before: event.Before,
				After:  event.After,
				Image:  event.Image,
				Reason: event.Reason,
			})
			if event.IsChange() {
				fmt.Fprintf(details, "  %s: CHANGE %d %s -> %s\n", path, event.Line, event.Before, event.After)
				continue
			}
			fmt.Fprintf(details, "  %s: SKIP %d %s (%s)\n", path, event.Line, event.Image, event.Reason)
		}

		if result.ChangeCount() > 0 {
			fileReport.Changed = true
			total.changed++
			if showDiff {
				fileReport.Diff = renderUnifiedDiff(path, string(original), result.Text, result.Events)
				diffs.WriteString(fileReport.Diff)
			}
			if write {
				if err := os.WriteFile(path, []byte(result.Text), mode); err != nil {
					return err
				}
				fileReport.Written = true
				total.written++
			}
		}
		report.Files = append(report.Files, fileReport)
	}

	report.Scanned = total.scanned
	report.Changed = total.changed
	report.Written = total.written
	report.SkippedValues = total.skips

	if asJSON {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(stdout, "%s strategy=%s: scanned=%d changed=%d written=%d skipped_values=%d\n",
			label, strategy, total.scanned, total.changed, total.written, total.skips)
		io.WriteString(stdout, details.String())
		if diffs.Len() > 0 {
			fmt.Fprintln(stdout)
			io.WriteString(stdout, diffs.String())
		}
		if !write && !check && total.changed > 0 {
			fmt.Fprintln(stdout, "提示: 加 --write 才会真正写回文件")
		}
	}

	if check && missingPaths > 0 {
		return fmt.Errorf("missing path")
	}
	if check && total.changed > 0 {
		return ErrChangesNeeded
	}
	return nil
}
