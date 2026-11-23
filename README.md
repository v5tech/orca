<p align="center">
  <img src="assets/orca-logo.svg" alt="orca（虎鲸镜像加速）" width="640">
</p>

[![CI](https://github.com/v5tech/orca/actions/workflows/ci.yml/badge.svg)](https://github.com/v5tech/orca/actions/workflows/ci.yml)
![CI Matrix](https://img.shields.io/badge/CI-ubuntu%20%7C%20macOS%20%7C%20Windows%20latest-blue)
![Release](https://img.shields.io/badge/release-darwin%20%7C%20linux%20%7C%20windows-green)
![Go](https://img.shields.io/badge/Go-single%20binary-00ADD8)

`orca` 是跨平台的镜像地址批量改写工具。它扫描 Dockerfile、Docker Compose 和 Kubernetes YAML，把镜像引用改为 DaoCloud 公共镜像加速地址。

## 快速开始

从 [GitHub Releases](https://github.com/v5tech/orca/releases) 下载对应平台二进制：

```bash
# 预览，不修改文件
orca .

# 检查模式：发现需要改写时返回退出码 1
orca -c .

# 写回修改
orca -w .

# 使用官方 registry 前缀替换表
orca -s replace -w ./manifests

# 查看版本
orca -v
```

## 选项

```text
用法
  orca <路径>...                      预览镜像改写
  orca -c <路径>...                   检查是否需要改写
  orca -w <路径>...                   写回修改
  orca -s replace <路径>...           使用官方前缀替换表

选项
  -w, --write                         写回文件；默认只预览
  -c, --check                         检查模式；发现改动时返回非零退出码
  -s, --strategy prepend|replace      镜像策略：prepend 或 replace
  -v, --version                       显示版本
  -h, --help                          显示帮助
```

终端支持 ANSI 颜色时，帮助信息会彩色显示。`NO_COLOR=1` 禁用颜色，`FORCE_COLOR=1` 强制启用。

短参数与长参数等价：`-w` = `--write`，`-s` = `--strategy`，`-c` = `--check`。

退出码：`0` 表示无需修改或写回成功；`1` 表示检查模式发现改动；`2` 表示参数或 IO 错误。

## 镜像策略

### `prepend`：加前缀（默认，官方推荐）

在原完整镜像引用前加 `m.daocloud.io/`：

```text
docker.io/library/busybox:1
m.daocloud.io/docker.io/library/busybox:1
```

### `replace`：替换 registry 前缀（官方标注不推荐）

只替换 registry 主机：

```text
docker.io/library/busybox:1
docker.m.daocloud.io/library/busybox:1
```

Docker Hub 简写会先规范化：`nginx` → `docker.io/library/nginx`。tag 和 `@sha256:` digest 原样保留。

## 支持的文件

| 文件 | 识别规则 | 改写位置 |
| --- | --- | --- |
| Dockerfile | `Dockerfile*`、`Containerfile*`、`*.dockerfile`、`*.containerfile` | `FROM` 的镜像参数 |
| Compose | `compose*.yml/yaml`、`docker-compose*.yml/yaml` | `image` 字段 |
| 其他 YAML | 其余 `*.yml` / `*.yaml` | `image` 字段 |

其他 YAML 也包括 CI 配置和 Helm values。工具只按文本规则改写 `image` 字段的值。目录会递归扫描，并跳过隐藏目录和常见构建目录。

## 改写规则

- 默认 dry-run；只有 `-w` / `--write` 才写文件。
- 幂等：已加速的镜像会被跳过。
- 跳过 `FROM scratch`、`${VAR}`、`{{ ... }}` 等动态值。
- 保留缩进、引号、注释、CRLF 和末尾换行状态。
- 第一段带点号或端口、且不在内置列表中的 registry 会跳过。
- 第一段不带点号或冒号时，按 Docker Hub 命名空间处理。
- `replace` 模式下，官方替换表外的 registry 会跳过。
- 源 registry 自带端口的引用会被跳过。

## `replace` 官方映射

| 源站 | 替换为 |
| --- | --- |
| `docker.elastic.co` | `elastic.m.daocloud.io` |
| `docker.io` | `docker.m.daocloud.io` |
| `dhi.io` | `dhi.m.daocloud.io` |
| `gcr.io` | `gcr.m.daocloud.io` |
| `ghcr.io` | `ghcr.m.daocloud.io` |
| `k8s.gcr.io` | `k8s-gcr.m.daocloud.io` |
| `registry.k8s.io` | `k8s.m.daocloud.io` |
| `mcr.microsoft.com` | `mcr.m.daocloud.io` |
| `nvcr.io` | `nvcr.m.daocloud.io` |
| `quay.io` | `quay.m.daocloud.io` |
| `registry.ollama.ai` | `ollama.m.daocloud.io` |

官方提示 `k8s.gcr.io` 已迁移到 `registry.k8s.io`；本工具按官方表格保留两种映射。

## 限制

- 镜像需要在 DaoCloud [白名单](https://github.com/DaoCloud/public-image-mirror/blob/a195786892746d03f11d3ef1ed360b703e7ea91b/allows.txt)中；服务有缓存和限流，见[官方说明](https://github.com/DaoCloud/public-image-mirror/issues/2328)。
- 内置公开 registry 列表是 `allows.txt` 的一次快照；官方新增 registry 后，需要更新代码。
- 这是静态文件改写工具，不是运行时 Webhook；Helm 模板里的动态值不会被改写。

## 开发

```bash
go test ./...
go vet ./...
gofmt -l .
```

构建：

```bash
go build -o orca ./cmd/orca
```

发布构建注入版本：

```bash
go build -ldflags "-X github.com/v5tech/orca/internal/cli.version=v2.0.0" -o orca ./cmd/orca
```

未注入版本时，CLI 会显示基于 Git commit 的 `dev-<short-sha>`；没有可用信息时显示 `dev`。

## 参考

- [DaoCloud public-image-mirror](https://github.com/DaoCloud/public-image-mirror)
- [DaoCloud README](https://github.com/DaoCloud/public-image-mirror/blob/a195786892746d03f11d3ef1ed360b703e7ea91b/README.md)
- [hack/correct-image.sh](https://github.com/DaoCloud/public-image-mirror/blob/a195786892746d03f11d3ef1ed360b703e7ea91b/hack/correct-image.sh)
- [白名单 & 限流说明](https://github.com/DaoCloud/public-image-mirror/issues/2328)
