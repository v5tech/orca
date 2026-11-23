// Package imageref resolves container image references to DaoCloud mirror addresses.
package imageref

//go:generate go run ../../cmd/update-allowlist -output allowlist_generated.go

import "strings"

// Strategy selects the official mirror syntax.
type Strategy string

const (
	// StrategyPrepend adds m.daocloud.io before the full upstream reference.
	StrategyPrepend Strategy = "prepend"
	// StrategyReplace replaces only the registry host with its mirror host.
	StrategyReplace Strategy = "replace"

	// DefaultPrefix is the official recommended prefix.
	DefaultPrefix = "m.daocloud.io"
)

const invalidChars = " \t$<>[]{}*?!|%&,"

// Accelerate returns the mirror reference and an empty reason on success.
func Accelerate(image string, strategy Strategy) (string, string) {
	candidate := strings.TrimSpace(image)
	if candidate == "" {
		return "", "空镜像引用"
	}
	if strings.ContainsAny(candidate, invalidChars) {
		return "", "模板变量或非法字符"
	}
	if candidate == "scratch" || candidate == "docker.io/library/scratch" {
		return "", "scratch 不是仓库镜像"
	}
	if isAccelerated(firstHost(candidate)) {
		return "", "已使用加速前缀"
	}

	ref, reason := normalize(candidate)
	if ref == "" {
		return "", reason
	}

	if strategy == StrategyReplace {
		host, path, ok := strings.Cut(ref, "/")
		if !ok {
			return "", "不支持或非法 registry"
		}
		mirror, ok := replacements[host]
		if !ok {
			return "", "该 registry 无官方替换前缀"
		}
		return mirror + "/" + path, ""
	}
	return DefaultPrefix + "/" + ref, ""
}

func firstHost(ref string) string {
	host, _, _ := strings.Cut(ref, "/")
	return host
}

func isAccelerated(host string) bool {
	return host == DefaultPrefix || strings.HasSuffix(host, ".m.daocloud.io")
}

func normalize(image string) (string, string) {
	candidate := strings.Trim(image, "/")
	if candidate == "" || strings.Contains(candidate, "//") {
		return "", "不支持或非法 registry"
	}

	if strings.HasPrefix(candidate, "registry.hub.docker.com/r/") {
		candidate = "docker.io/" + strings.TrimPrefix(candidate, "registry.hub.docker.com/r/")
	} else if strings.HasPrefix(candidate, "hub.docker.com/r/") {
		candidate = "docker.io/" + strings.TrimPrefix(candidate, "hub.docker.com/r/")
	}

	if !strings.Contains(candidate, "/") {
		candidate = "library/" + candidate
	}

	host := firstHost(candidate)
	if host == "localhost" {
		return "", "不支持或非法 registry"
	}
	if _, ok := supported[host]; !ok {
		if strings.ContainsAny(host, ".:") {
			return "", "不支持或非法 registry"
		}
		candidate = "docker.io/" + candidate
	}

	namePart, digest, hasDigest := strings.Cut(candidate, "@")
	if hasDigest && (strings.Contains(digest, "@") || !validDigest(digest)) {
		return "", "不支持或非法 registry"
	}

	last := namePart
	if index := strings.LastIndex(last, "/"); index >= 0 {
		last = last[index+1:]
	}
	if index := strings.LastIndex(last, ":"); index >= 0 {
		tag := last[index+1:]
		if hasDigest || !validTag(tag) {
			return "", "不支持或非法 registry"
		}
	}

	if hasDigest {
		return namePart + "@" + digest, ""
	}
	return namePart, ""
}

func validTag(tag string) bool {
	if tag == "" || len(tag) > 128 || !isTagStart(tag[0]) {
		return false
	}
	for i := 1; i < len(tag); i++ {
		if !isTagPart(tag[i]) {
			return false
		}
	}
	return true
}

func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func isTagStart(b byte) bool {
	return isAlphaNum(b) || b == '_'
}

func isTagPart(b byte) bool {
	return isTagStart(b) || b == '.' || b == '-'
}

func validDigest(digest string) bool {
	const alg = "sha256:"
	if len(digest) < len(alg)+32 || !strings.HasPrefix(digest, alg) {
		return false
	}
	for i := len(alg); i < len(digest); i++ {
		b := digest[i]
		if !isAlphaNum(b) && b != '=' && b != '_' && b != '-' {
			return false
		}
	}
	return true
}
