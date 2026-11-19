#!/bin/sh
# 纯 sh 测试：orca.sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
script="$root/orca.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

fail() {
    printf 'FAIL: %s\n' "$*" >&2
    exit 1
}

assert_eq() {
    [ "$1" = "$2" ] || fail "expected <$2>, got <$1>"
}

assert_contains() {
    printf '%s\n' "$1" | grep -Fq "$2" || fail "output does not contain <$2>"
}

run() {
    sh "$script" "$@" >"$tmp/stdout" 2>"$tmp/stderr" || fail "script failed: $*"
}

cat >"$tmp/Dockerfile" <<'EOF'
FROM --platform=$BUILDPLATFORM node:20 AS build
FROM nginx
FROM scratch
FROM ${BASE_IMAGE}
EOF
run "$tmp/Dockerfile"
assert_contains "$(cat "$tmp/stdout")" "DRY-RUN"
assert_contains "$(cat "$tmp/stdout")" "node:20 -> m.daocloud.io/docker.io/library/node:20"
assert_contains "$(cat "$tmp/stdout")" "CHANGE 1 node:20 ->"
assert_eq "$(cat "$tmp/Dockerfile")" "$(cat <<'EOF'
FROM --platform=$BUILDPLATFORM node:20 AS build
FROM nginx
FROM scratch
FROM ${BASE_IMAGE}
EOF
)"

cat >"$tmp/docker-compose.yml" <<'EOF'
services:
  web:
    image: "nginx:1.27"  # app
  worker:
    image: ${IMAGE}
EOF
run "$tmp/docker-compose.yml" --write
assert_eq "$(cat "$tmp/docker-compose.yml")" "$(cat <<'EOF'
services:
  web:
    image: "m.daocloud.io/docker.io/library/nginx:1.27"  # app
  worker:
    image: ${IMAGE}
EOF
)"

cat >"$tmp/deployment.yaml" <<'EOF'
apiVersion: apps/v1
kind: Deployment
spec:
  containers:
    - image: quay.io/prometheus/prometheus:v2.55.0
    - image: m.daocloud.io/docker.io/library/busybox
    - image: registry.internal.example.com/team/app:1
EOF
run "$tmp/deployment.yaml" --write
assert_eq "$(cat "$tmp/deployment.yaml")" "$(cat <<'EOF'
apiVersion: apps/v1
kind: Deployment
spec:
  containers:
    - image: m.daocloud.io/quay.io/prometheus/prometheus:v2.55.0
    - image: m.daocloud.io/docker.io/library/busybox
    - image: registry.internal.example.com/team/app:1
EOF
)"

printf 'image: public.ecr.aws/nginx:1.27\n' >"$tmp/prepend.yaml"
run "$tmp/prepend.yaml" --write
assert_eq "$(cat "$tmp/prepend.yaml")" "image: m.daocloud.io/public.ecr.aws/nginx:1.27"

cat >"$tmp/Dockerfile" <<'EOF'
FROM docker.io/library/busybox:1
FROM docker.elastic.co/elasticsearch/elasticsearch:8.0.0
FROM dhi.io/foo/bar:1
FROM gcr.io/google-containers/pause:3.9
FROM ghcr.io/foo/bar:1
FROM k8s.gcr.io/coredns/coredns:v1.8.0
FROM registry.k8s.io/pause:3.9
FROM mcr.microsoft.com/dotnet/runtime:8.0
FROM nvcr.io/nvidia/pytorch:24.01
FROM quay.io/prometheus/prometheus:v2.55.0
FROM registry.ollama.ai/library/deepseek-r1:1.5b
FROM public.ecr.aws/nginx:1.27
FROM scratch
EOF
run "$tmp/Dockerfile" --strategy replace --write
assert_eq "$(cat "$tmp/Dockerfile")" "$(cat <<'EOF'
FROM docker.m.daocloud.io/library/busybox:1
FROM elastic.m.daocloud.io/elasticsearch/elasticsearch:8.0.0
FROM dhi.m.daocloud.io/foo/bar:1
FROM gcr.m.daocloud.io/google-containers/pause:3.9
FROM ghcr.m.daocloud.io/foo/bar:1
FROM k8s-gcr.m.daocloud.io/coredns/coredns:v1.8.0
FROM k8s.m.daocloud.io/pause:3.9
FROM mcr.m.daocloud.io/dotnet/runtime:8.0
FROM nvcr.m.daocloud.io/nvidia/pytorch:24.01
FROM quay.m.daocloud.io/prometheus/prometheus:v2.55.0
FROM ollama.m.daocloud.io/library/deepseek-r1:1.5b
FROM public.ecr.aws/nginx:1.27
FROM scratch
EOF
)"

run "$tmp/Dockerfile" --strategy replace --write
assert_contains "$(cat "$tmp/stdout")" "changed=0"
assert_contains "$(cat "$tmp/stdout")" "(已使用加速前缀)"

printf 'FROM --platform=linux/amd64 --network=none nginx:1.27 AS build\n' >"$tmp/multiflags.Dockerfile"
run "$tmp/multiflags.Dockerfile" --write
assert_eq "$(cat "$tmp/multiflags.Dockerfile")" "FROM --platform=linux/amd64 --network=none m.daocloud.io/docker.io/library/nginx:1.27 AS build"

digest="sha256:$(printf '%064d' 0 | tr '0' 'a')"
printf 'image: nginx@%s\n' "$digest" >"$tmp/digest.yaml"
run "$tmp/digest.yaml" --write
assert_eq "$(cat "$tmp/digest.yaml")" "image: m.daocloud.io/docker.io/library/nginx@$digest"

printf 'FROM nginx:1.27' >"$tmp/noeol.Dockerfile"
run "$tmp/noeol.Dockerfile" --write
assert_eq "$(od -An -tx1 "$tmp/noeol.Dockerfile" | tr -d ' \n')" "$(printf 'FROM m.daocloud.io/docker.io/library/nginx:1.27' | od -An -tx1 | tr -d ' \n')"

printf 'FROM nginx:1.27\r\n' >"$tmp/crlf.Dockerfile"
run "$tmp/crlf.Dockerfile" --write
assert_eq "$(od -An -tx1 "$tmp/crlf.Dockerfile" | tr -d ' \n')" "$(printf 'FROM m.daocloud.io/docker.io/library/nginx:1.27\r\n' | od -An -tx1 | tr -d ' \n')"

mkdir -p "$tmp/-weird"
printf 'FROM nginx:1.27\n' >"$tmp/-weird/Dockerfile"
oldpwd=$(pwd)
cd "$tmp"
run --write -- "-weird"
cd "$oldpwd"
assert_eq "$(cat "$tmp/-weird/Dockerfile")" "FROM m.daocloud.io/docker.io/library/nginx:1.27"

printf 'PASS: %s\n' "$0"
