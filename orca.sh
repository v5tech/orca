#!/bin/sh
# orca.sh（虎鲸镜像加速）
# 批量将 Dockerfile / Compose / K8s 镜像引用改写为 DaoCloud 加速地址。
# 纯 POSIX sh + awk 实现，无需 Python；默认 dry-run，--write 才写回。

set -eu

LC_ALL=C
export LC_ALL

usage() {
    cat <<'EOF'
orca.sh — 虎鲸镜像加速

用法: orca.sh [-w|--write] [-s|--strategy prepend|replace] <文件或目录>...

选项:
  -w, --write                 写回文件；默认只预览
  -s, --strategy STRATEGY     prepend=加 m.daocloud.io 前缀（默认，官方推荐）
                              replace=按官方 registry 前缀替换表
  -h, --help                  显示帮助

示例:
  orca.sh .
  orca.sh --write path/to/docker-compose.yml
  orca.sh --strategy replace --write path/to/manifests
EOF
}

mode='dry'
strategy='prepend'

tmpdir=$(mktemp -d "${TMPDIR:-/tmp}/image-mirror.XXXXXX")
trap 'rm -rf "$tmpdir"' EXIT HUP INT TERM
paths_file="$tmpdir/paths"
files="$tmpdir/files"
all_report="$tmpdir/all_report"
: >"$paths_file"
: >"$files"
: >"$all_report"

while [ "$#" -gt 0 ]; do
    case "$1" in
        -w | --write)
            mode='write'
            shift
            ;;
        -s | --strategy)
            [ "$#" -ge 2 ] || {
                printf '%s\n' '错误: --strategy 缺少参数' >&2
                usage >&2
                exit 2
            }
            strategy=$2
            shift 2
            ;;
        --strategy=*)
            strategy=${1#--strategy=}
            shift
            ;;
        -h | --help)
            usage
            exit 0
            ;;
        --)
            shift
            while [ "$#" -gt 0 ]; do
                printf '%s\n' "$1" >>"$paths_file"
                shift
            done
            ;;
        -*)
            printf '错误: 未知选项 %s\n' "$1" >&2
            usage >&2
            exit 2
            ;;
        *)
            printf '%s\n' "$1" >>"$paths_file"
            shift
            ;;
    esac
done

case "$strategy" in
    prepend | replace) ;;
    *)
        printf '错误: 未知策略 %s（可选 prepend|replace）\n' "$strategy" >&2
        exit 2
        ;;
esac

if [ ! -s "$paths_file" ]; then
    printf '%s\n' '错误: 至少指定一个文件或目录' >&2
    usage >&2
    exit 2
fi

while IFS= read -r path; do
    [ -n "$path" ] || continue
    if [ -f "$path" ]; then
        printf '%s\n' "$path" >>"$files"
    elif [ -d "$path" ]; then
        search_path="$path"
        case $search_path in
            -*) search_path=./$search_path ;;
        esac
        find "$search_path" \
            \( -name .git -o -name .venv -o -name venv -o -name node_modules -o -name __pycache__ \
            -o -name dist -o -name build -o -name outputs -o -name logs -o -name reports \
            -o -name .idea -o -name .pytest_cache -o -name .ruff_cache -o -name .playwright-cli -o -name .workbuddy \) -prune -o \
            -type f -print >>"$files"
    else
        printf '跳过不存在的路径: %s\n' "$path" >&2
    fi
done <"$paths_file"

sort -u "$files" >"$files.sorted"
mv "$files.sorted" "$files"

detect_type() {
    case "$1" in
        [Dd]ockerfile | [Dd]ockerfile.* | [Cc]ontainerfile | [Cc]ontainerfile.* | *.[Dd]ockerfile | *.[Cc]ontainerfile)
            printf 'dockerfile'
            ;;
        [Cc]ompose.yml | [Cc]ompose.yaml | [Cc]ompose.*.yml | [Cc]ompose.*.yaml | \
            [Dd]ocker-[Cc]ompose.yml | [Dd]ocker-[Cc]ompose.yaml | [Dd]ocker-[Cc]ompose.*.yml | [Dd]ocker-[Cc]ompose.*.yaml)
            printf 'compose'
            ;;
        *.yml | *.yaml)
            printf 'kubernetes'
            ;;
        *)
            printf ''
            ;;
    esac
}

scanned=0
changed_files=0
written_files=0
skipped_values=0

while IFS= read -r file; do
    [ -n "$file" ] || continue
    [ -f "$file" ] || continue

    file_type=$(detect_type "${file##*/}")
    [ -n "$file_type" ] || continue
    scanned=$((scanned + 1))

    report="$tmpdir/report"
    out="$tmpdir/out"
    : >"$report"

    trailing=0
    if [ -s "$file" ] && [ "$(tail -c 1 <"$file" | wc -l | tr -d '[:space:]')" -eq 1 ]; then
        trailing=1
    fi

    awk \
        -v strategy="$strategy" \
        -v filetype="$file_type" \
        -v trailing="$trailing" \
        -v report="$report" \
        '
        BEGIN {
            mirror_prefix = "m.daocloud.io"
            split("docker.elastic.co docker.io dhi.io gcr.io ghcr.io k8s.gcr.io registry.k8s.io mcr.microsoft.com nvcr.io quay.io registry.ollama.ai", repl_src, " ")
            split("elastic.m.daocloud.io docker.m.daocloud.io dhi.m.daocloud.io gcr.m.daocloud.io ghcr.m.daocloud.io k8s-gcr.m.daocloud.io k8s.m.daocloud.io mcr.m.daocloud.io nvcr.m.daocloud.io quay.m.daocloud.io ollama.m.daocloud.io", repl_dst, " ")
            for (i in repl_src) replace_map[repl_src[i]] = repl_dst[i]

            split("code.forgejo.org codeberg.org container-registry.oracle.com cr.agentgateway.dev cr.kgateway.dev cr.l5d.io cr.weaviate.io dhi.io docker.com docker.elastic.co docker.getcollate.io docker.io docker.osgeo.org docker.stirlingpdf.com gcr.io ghcr.io k8s.gcr.io lscr.io mcr.microsoft.com nvcr.io ollama.ai public.ecr.aws quay.io registry.developers.crunchydata.com registry.gitlab.com registry.heroiclabs.com registry.jujucharms.com registry.k8s.io registry.librechat.ai registry.ollama.ai registry.opensource.zalan.do registry.rocket.chat registry.supertokens.io rocks.canonical.com us-central1-docker.pkg.dev us.gcr.io", supported_list, " ")
            for (i in supported_list) supported[supported_list[i]] = 1
        }

        function skip(reason) {
            skip_reason = reason
            return ""
        }

        function already_accelerated(host) {
            return host == mirror_prefix || host ~ /\.m\.daocloud\.io$/
        }

        function first_host(ref) {
            pos = index(ref, "/")
            return pos == 0 ? ref : substr(ref, 1, pos - 1)
        }

        function normalize_ref(image,   candidate, bad, i, marker, host, parts, count, name_part, digest, last, tag) {
            candidate = image
            sub(/^\/+/, "", candidate)
            sub(/\/+$/, "", candidate)
            if (candidate == "" || index(candidate, "//") > 0) return skip("不支持或非法 registry")

            bad = " " sprintf("%c", 9) "$<>[]{}*?!|" "%" "&,"
            for (i = 1; i <= length(bad); i++) {
                if (index(candidate, substr(bad, i, 1)) > 0) return skip("模板变量或非法字符")
            }

            marker = "registry.hub.docker.com/r/"
            if (index(candidate, marker) == 1) candidate = "docker.io/" substr(candidate, length(marker) + 1)
            marker = "hub.docker.com/r/"
            if (index(candidate, marker) == 1) candidate = "docker.io/" substr(candidate, length(marker) + 1)

            if (index(candidate, "/") == 0) candidate = "library/" candidate
            host = first_host(candidate)
            if (host == "localhost") return skip("不支持或非法 registry")
            if (!(host in supported)) {
                if (index(host, ".") > 0 || index(host, ":") > 0) return skip("不支持或非法 registry")
                candidate = "docker.io/" candidate
            }

            count = split(candidate, parts, "@")
            if (count > 2) return skip("不支持或非法 registry")
            name_part = parts[1]
            digest = count == 2 ? parts[2] : ""
            if (count == 2 && (length(digest) < 39 || digest !~ /^sha256:[A-Za-z0-9=_-]+$/)) {
                return skip("不支持或非法 registry")
            }

            last = name_part
            if (index(last, "/") > 0) last = substr(last, index(last, "/") + 1)
            if (index(last, ":") > 0) {
                tag = substr(last, index(last, ":") + 1)
                if (count == 2 || length(tag) > 128 || tag !~ /^[A-Za-z0-9_][A-Za-z0-9_.-]*$/) {
                    return skip("不支持或非法 registry")
                }
            }

            return count == 2 ? name_part "@" digest : name_part
        }

        function accelerate(image,   candidate, host, path, mirror) {
            candidate = image
            if (candidate == "") return skip("空镜像引用")
            if (candidate == "scratch" || candidate == "docker.io/library/scratch") return skip("scratch 不是仓库镜像")
            if (already_accelerated(first_host(candidate))) return skip("已使用加速前缀")

            candidate = normalize_ref(candidate)
            if (candidate == "") return ""

            if (strategy == "replace") {
                host = first_host(candidate)
                path = substr(candidate, index(candidate, "/") + 1)
                mirror = replace_map[host]
                if (mirror == "") return skip("该 registry 无官方替换前缀")
                return mirror "/" path
            }
            return mirror_prefix "/" candidate
        }

        function report_change(line_no, before, after) {
            printf "CHANGE %d %s -> %s\n", line_no, before, after > report
        }

        function report_skip(line_no, image, reason) {
            printf "SKIP %d %s (%s)\n", line_no, image, reason > report
        }

        function transform_dockerfile(line, line_no,   cr, leading, body, lower, prefix_part, rest, flag, image, suffix, accelerated) {
            cr = ""
            if (length(line) > 0 && substr(line, length(line), 1) == "\r") {
                cr = "\r"
                line = substr(line, 1, length(line) - 1)
            }

            leading = line
            sub(/[^[:space:]].*$/, "", leading)
            body = substr(line, length(leading) + 1)
            lower = tolower(body)
            if (substr(lower, 1, 4) != "from") return line cr
            if (length(body) < 5 || substr(body, 5, 1) !~ /[[:space:]]/) return line cr

            prefix_part = substr(body, 1, 4)
            rest = substr(body, 5)
            while (match(rest, /^[[:space:]]+/)) {
                flag = substr(rest, 1, RLENGTH)
                prefix_part = prefix_part flag
                rest = substr(rest, RLENGTH + 1)
            }
            while (match(rest, /^--[A-Za-z0-9-]+=[^[:space:]]+/)) {
                flag = substr(rest, 1, RLENGTH)
                prefix_part = prefix_part flag
                rest = substr(rest, RLENGTH + 1)
                while (match(rest, /^[[:space:]]+/)) {
                    flag = substr(rest, 1, RLENGTH)
                    prefix_part = prefix_part flag
                    rest = substr(rest, RLENGTH + 1)
                }
            }
            if (rest == "") return line cr

            if (match(rest, /[[:space:]]/)) {
                image = substr(rest, 1, RSTART - 1)
                suffix = substr(rest, RSTART)
            } else {
                image = rest
                suffix = ""
            }

            accelerated = accelerate(image)
            if (accelerated == "") {
                report_skip(line_no, image, skip_reason)
                return line cr
            }
            report_change(line_no, image, accelerated)
            return leading prefix_part accelerated suffix cr
        }

        function transform_image_field(line, line_no,   cr, dq, sq, prefix_part, rest, quote, ch, i, image, suffix, accelerated) {
            cr = ""
            if (length(line) > 0 && substr(line, length(line), 1) == "\r") {
                cr = "\r"
                line = substr(line, 1, length(line) - 1)
            }
            if (line !~ /^[[:space:]]*(-[[:space:]]+)?image[[:space:]]*:[[:space:]]*/) return line cr

            dq = sprintf("%c", 34)
            sq = sprintf("%c", 39)
            match(line, /^[[:space:]]*(-[[:space:]]+)?image[[:space:]]*:[[:space:]]*/)
            prefix_part = substr(line, 1, RLENGTH)
            rest = substr(line, RLENGTH + 1)
            quote = ""
            if (substr(rest, 1, 1) == dq || substr(rest, 1, 1) == sq) {
                quote = substr(rest, 1, 1)
                rest = substr(rest, 2)
            }

            for (i = 1; i <= length(rest); i++) {
                ch = substr(rest, i, 1)
                if (ch == " " || ch == sprintf("%c", 9) || ch == "#" || ch == dq || ch == sq) break
            }
            if (i > length(rest)) {
                image = rest
                suffix = ""
            } else {
                image = substr(rest, 1, i - 1)
                suffix = substr(rest, i)
            }

            accelerated = accelerate(image)
            if (accelerated == "") {
                report_skip(line_no, image, skip_reason)
                return line cr
            }
            report_change(line_no, image, accelerated)
            return prefix_part quote accelerated suffix cr
        }

        {
            output = $0
            if (filetype == "dockerfile") {
                output = transform_dockerfile($0, NR)
            } else {
                output = transform_image_field($0, NR)
            }
            if (NR > 1) printf "\n"
            printf "%s", output
        }

        END {
            if (trailing == 1 && NR > 0) printf "\n"
        }
        ' <"$file" >"$out"

    changes=$(awk '/^CHANGE /{n++} END{print n+0}' "$report")
    skips=$(awk '/^SKIP /{n++} END{print n+0}' "$report")
    skipped_values=$((skipped_values + skips))

    while IFS= read -r line; do
        printf '%s: %s\n' "$file" "$line" >>"$all_report"
    done <"$report"

    if [ "$changes" -gt 0 ]; then
        changed_files=$((changed_files + 1))
        if [ "$mode" = write ]; then
            cat <"$out" >"$file"
            written_files=$((written_files + 1))
        fi
    fi
done <"$files"

label=DRY-RUN
[ "$mode" = write ] && label=WRITE
printf '%s strategy=%s: scanned=%d changed=%d written=%d skipped_values=%d\n' \
    "$label" "$strategy" "$scanned" "$changed_files" "$written_files" "$skipped_values"
while IFS= read -r line; do
    printf '  %s\n' "$line"
done <"$all_report"

if [ "$mode" != write ] && [ "$changed_files" -gt 0 ]; then
    printf '%s\n' '提示: 加 --write 才会真正写回文件'
fi
