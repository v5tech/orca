// Package rewrite rewrites image references in container configuration text.
package rewrite

import (
	"regexp"
	"strings"

	"github.com/v5tech/orca/internal/imageref"
)

// Kind identifies the syntax used by a file.
type Kind string

const (
	// KindDockerfile rewrites FROM instructions.
	KindDockerfile Kind = "dockerfile"
	// KindCompose rewrites image fields in Compose files.
	KindCompose Kind = "compose"
	// KindKubernetes rewrites image fields in other YAML files.
	KindKubernetes Kind = "kubernetes"
)

// Event records one report entry in line order.
type Event struct {
	Line   int
	Kind   string
	Before string
	After  string
	Image  string
	Reason string
}

// Result holds rewritten text and its report entries.
type Result struct {
	Text   string
	Events []Event
}

// IsChange reports whether the event describes a rewrite.
func (e Event) IsChange() bool { return e.Kind == eventChange }

// ChangeCount returns the number of rewritten image references.
func (r Result) ChangeCount() int {
	count := 0
	for _, event := range r.Events {
		if event.IsChange() {
			count++
		}
	}
	return count
}

// SkipCount returns the number of skipped image references.
func (r Result) SkipCount() int {
	return len(r.Events) - r.ChangeCount()
}

func (r *Result) addChange(lineNo int, image string, accelerated string) {
	r.Events = append(r.Events, Event{Line: lineNo, Kind: eventChange, Before: image, After: accelerated})
}

func (r *Result) addSkip(lineNo int, image string, reason string) {
	r.Events = append(r.Events, Event{Line: lineNo, Kind: eventSkip, Image: image, Reason: reason})
}

const (
	eventChange    = "change"
	eventSkip      = "skip"
	imageStopChars = " \t#'\""
)

var (
	dockerfileNameRe = regexp.MustCompile(`^(dockerfile|containerfile)([._-].*)?$|.*\.(dockerfile|containerfile)$`)
	composeNameRe    = regexp.MustCompile(`^(docker-)?compose([._-][^.]*)?\.ya?ml$`)
)

// DetectKind classifies a file by name.
func DetectKind(name string) Kind {
	name = strings.ToLower(name)
	if dockerfileNameRe.MatchString(name) {
		return KindDockerfile
	}
	if composeNameRe.MatchString(name) {
		return KindCompose
	}
	if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
		return KindKubernetes
	}
	return Kind("")
}

// Rewrite applies the strategy to all recognized image references in text.
func Rewrite(text string, kind Kind, strategy imageref.Strategy) Result {
	result := Result{Events: make([]Event, 0, strings.Count(text, "\n")+1)}
	var output strings.Builder
	output.Grow(len(text))

	lineNo := 0
	for len(text) > 0 {
		index := strings.IndexByte(text, '\n')
		line := text
		if index >= 0 {
			line = text[:index+1]
			text = text[index+1:]
		} else {
			text = ""
		}
		lineNo++
		rewriteLineTo(line, kind, strategy, lineNo, &result, &output)
	}

	result.Text = output.String()
	return result
}

func rewriteLineTo(line string, kind Kind, strategy imageref.Strategy, lineNo int, result *Result, output *strings.Builder) {
	body, newline := splitNewline(line)
	cr := ""
	if strings.HasSuffix(body, "\r") {
		cr = "\r"
		body = strings.TrimSuffix(body, "\r")
	}
	if kind == KindDockerfile {
		rewriteFromTo(body, strategy, lineNo, result, output)
	} else {
		rewriteImageFieldTo(body, strategy, lineNo, result, output)
	}
	output.WriteString(cr)
	output.WriteString(newline)
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\f' || b == '\r'
}

func skipSpaces(s string, i int) int {
	for i < len(s) && isSpace(s[i]) {
		i++
	}
	return i
}

func isFlagNameByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '-'
}

func emitRewrite(result *Result, output *strings.Builder, body, prefix, image, suffix string, lineNo int, accelerated, reason string, quote byte) {
	if accelerated == "" {
		result.addSkip(lineNo, image, reason)
		output.WriteString(body)
		return
	}
	result.addChange(lineNo, image, accelerated)
	output.WriteString(prefix)
	if quote != 0 {
		output.WriteByte(quote)
	}
	output.WriteString(accelerated)
	output.WriteString(suffix)
}

func rewriteFromTo(body string, strategy imageref.Strategy, lineNo int, result *Result, output *strings.Builder) {
	i := skipSpaces(body, 0)
	if i+4 > len(body) || !strings.EqualFold(body[i:i+4], "FROM") {
		output.WriteString(body)
		return
	}
	i += 4
	if i >= len(body) || !isSpace(body[i]) {
		output.WriteString(body)
		return
	}
	i = skipSpaces(body, i)

	for i+2 < len(body) && body[i] == '-' && body[i+1] == '-' {
		j := i + 2
		for j < len(body) && isFlagNameByte(body[j]) {
			j++
		}
		if j == i+2 || j >= len(body) || body[j] != '=' {
			break
		}
		j++
		valueStart := j
		for j < len(body) && !isSpace(body[j]) {
			j++
		}
		if j == valueStart || j >= len(body) || !isSpace(body[j]) {
			break
		}
		i = skipSpaces(body, j)
	}

	imageStart := i
	for imageStart < len(body) && !isSpace(body[imageStart]) {
		imageStart++
	}
	if imageStart == i {
		output.WriteString(body)
		return
	}
	image := body[i:imageStart]

	accelerated, reason := imageref.Accelerate(image, strategy)
	emitRewrite(result, output, body, body[:i], image, body[imageStart:], lineNo, accelerated, reason, 0)
}

func rewriteImageFieldTo(body string, strategy imageref.Strategy, lineNo int, result *Result, output *strings.Builder) {
	i := skipSpaces(body, 0)
	if i < len(body) && body[i] == '-' {
		i++
		next := skipSpaces(body, i)
		if next == i {
			output.WriteString(body)
			return
		}
		i = next
	}
	if i+5 > len(body) || body[i:i+5] != "image" {
		output.WriteString(body)
		return
	}
	i += 5
	i = skipSpaces(body, i)
	if i >= len(body) || body[i] != ':' {
		output.WriteString(body)
		return
	}
	i++
	i = skipSpaces(body, i)

	prefix := body[:i]
	rest := body[i:]
	quote := byte(0)
	if len(rest) > 0 && (rest[0] == '"' || rest[0] == '\'') {
		quote = rest[0]
		rest = rest[1:]
	}

	end := strings.IndexAny(rest, imageStopChars)
	image := rest
	suffix := ""
	if end >= 0 {
		image = rest[:end]
		suffix = rest[end:]
	}

	accelerated, reason := imageref.Accelerate(image, strategy)
	emitRewrite(result, output, body, prefix, image, suffix, lineNo, accelerated, reason, quote)
}

func splitNewline(line string) (string, string) {
	if body, ok := strings.CutSuffix(line, "\n"); ok {
		return body, "\n"
	}
	return line, ""
}
