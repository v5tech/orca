// Package cli implements the orca command-line interface.
package cli

import (
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
	var showDiff bool
	var strategyValue string

	flags.BoolVar(&showVersion, "version", false, "显示版本并退出")
	flags.BoolVar(&showVersion, "v", false, "显示版本并退出")
	flags.BoolVar(&showHelp, "help", false, "显示帮助")
	flags.BoolVar(&showHelp, "h", false, "显示帮助")
	flags.BoolVar(&write, "write", false, "写回文件")
	flags.BoolVar(&write, "w", false, "写回文件")
	flags.BoolVar(&showDiff, "diff", false, "显示改写差异")
	flags.BoolVar(&showDiff, "d", false, "显示改写差异")
	flags.StringVar(&strategyValue, "strategy", string(imageref.StrategyPrepend), "镜像策略：prepend 或 replace")
	flags.StringVar(&strategyValue, "s", string(imageref.StrategyPrepend), "镜像策略：prepend 或 replace")

	flags.Usage = func() {
		colored := colorEnabled(stderr)
		fmt.Fprintln(stderr, paint(colored, colorLogo, terminalLogo))
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, paint(colored, colorHead, "用法"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  orca <路径>...                      分析目标路径并输出变更计划（dry-run）"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  orca -d <路径>...                   输出改写差异"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  orca -s replace -d <路径>...        使用前缀替换策略输出差异"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  orca -w <路径>...                   应用变更至目标文件"))
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, paint(colored, colorHead, "选项"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  -d, --diff                          输出统一差异"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  -s, --strategy prepend|replace      指定镜像加速策略"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  -w, --write                         应用变更至目标文件"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  -v, --version                       显示版本信息"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  -h, --help                          显示帮助信息"))
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, paint(colored, colorHead, "策略"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  prepend   在原完整引用前添加 m.daocloud.io/（默认）"))
		fmt.Fprintln(stderr, paint(colored, colorBody, "  replace   按官方映射替换 registry 前缀"))
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

	strategy := imageref.Strategy(strategyValue)
	if strategy != imageref.StrategyPrepend && strategy != imageref.StrategyReplace {
		return fmt.Errorf("unknown strategy %q: use prepend or replace", strategyValue)
	}

	if flags.NArg() == 0 {
		flags.Usage()
		return fmt.Errorf("missing path")
	}

	files, err := discover.Files(flags.Args(), func(message string) {
		fmt.Fprintln(stderr, message)
	})
	if err != nil {
		return err
	}

	total := summary{}
	label := "DRY-RUN"
	if write {
		label = "WRITE"
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

		for _, event := range result.Events {
			if event.IsChange() {
				fmt.Fprintf(details, "  %s: CHANGE %d %s -> %s\n", path, event.Line, event.Before, event.After)
				continue
			}
			fmt.Fprintf(details, "  %s: SKIP %d %s (%s)\n", path, event.Line, event.Image, event.Reason)
		}

		if result.ChangeCount() == 0 {
			continue
		}
		total.changed++
		if showDiff {
			diffs.WriteString(renderUnifiedDiff(path, string(original), result.Text, result.Events))
		}
		if write {
			if err := os.WriteFile(path, []byte(result.Text), mode); err != nil {
				return err
			}
			total.written++
		}
	}

	fmt.Fprintf(stdout, "%s strategy=%s: scanned=%d changed=%d written=%d skipped_values=%d\n",
		label, strategy, total.scanned, total.changed, total.written, total.skips)
	io.WriteString(stdout, details.String())
	if diffs.Len() > 0 {
		fmt.Fprintln(stdout)
		io.WriteString(stdout, diffs.String())
	}
	if !write && total.changed > 0 {
		fmt.Fprintln(stdout, "提示: 加 --write 才会真正写回文件")
	}
	return nil
}
