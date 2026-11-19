# orca.sh（虎鲸镜像加速）

[![CI](https://github.com/v5tech/orca.sh/actions/workflows/ci.yml/badge.svg)](https://github.com/v5tech/orca.sh/actions/workflows/ci.yml)
![macOS 12+](https://img.shields.io/badge/macOS-12%2B-blue)
![Ubuntu](https://img.shields.io/badge/Ubuntu-22.04%20%7C%2024.04-orange)
![CentOS Stream 9](https://img.shields.io/badge/CentOS%20Stream-9-blue)
![RHEL UBI 9](https://img.shields.io/badge/RHEL%20UBI-9-red)
![POSIX](https://img.shields.io/badge/POSIX-sh%20%2B%20awk-orange)

`orca.sh`（虎鲸镜像加速）是一个命令行工具，批量把 **Dockerfile / Docker Compose / Kubernetes YAML** 中的镜像引用改写为 DaoCloud 公共镜像加速地址。

它不依赖 Python、yq 或 jq。运行需要 `sh`、`awk`、`find`、`sort`、`tail`、`wc`、`tr`、`mktemp`、`mv`、`rm`；脚本主体是 POSIX sh + awk，并固定使用 C locale 保证行为一致，`mktemp` 非 POSIX 但常见。

## 快速开始

```bash
# 预览：默认使用官方推荐的 prepend 策略，不会改文件
sh orca.sh .

# 写回：只有 --write 才会修改文件
sh orca.sh --write deploy/prod/docker-compose.yml

# 使用官方 registry 前缀替换表（官方标注为不推荐）
sh orca.sh --strategy replace --write ./manifests
```

参数可以是文件或目录。目录会递归扫描，但会跳过 `.git`、`.venv`、`venv`、`node_modules`、`__pycache__`、`dist`、`build`、`outputs`、`logs`、`reports`、`.idea`、`.pytest_cache`、`.ruff_cache`、`.playwright-cli`、`.workbuddy`。

以 `-` 开头的路径需要用 `--` 分隔：`sh orca.sh --write -- -weird-dir/`。

## 它是怎么工作的

DaoCloud 官方提供了两种镜像加速写法：

1. **`prepend`：在原完整镜像引用前加 `m.daocloud.io/`**（官方推荐，本项目默认）

   ```text
   输入:  docker.io/library/busybox:1
   输出: m.daocloud.io/docker.io/library/busybox:1
   ```

2. **`replace`：只把 registry 主机换成专用加速域名**（官方标注“不推荐、人工配置”）

   ```text
   输入:  docker.io/library/busybox:1
   输出: docker.m.daocloud.io/library/busybox:1
   ```

非 Docker Hub 镜像同样适用：

- `ghcr.io/foo/bar:v1`
  - `prepend` → `m.daocloud.io/ghcr.io/foo/bar:v1`
  - `replace` → `ghcr.m.daocloud.io/foo/bar:v1`

准确地说：

- `prepend` 对**已经完整**的引用只加前缀，不改原引用。
- 为了得到完整引用，Docker Hub 简写会先规范化：`nginx` → `docker.io/library/nginx`，`library/busybox` → `docker.io/library/busybox`；`docker.io/python:3.13` 这类已带 `docker.io` 的写法保持原样。
- `replace` 是把镜像地址里的第一段（registry 主机）按官方映射表换掉。
- 两者都是 DaoCloud 官方列出的写法，但官方明确推荐 `prepend`。
- tag 和 `@sha256:` digest 原样保留，工具不会主动添加 `:latest`。

来源：[DaoCloud public-image-mirror README](https://github.com/DaoCloud/public-image-mirror/blob/a195786892746d03f11d3ef1ed360b703e7ea91b/README.md)、[hack/correct-image.sh](https://github.com/DaoCloud/public-image-mirror/blob/a195786892746d03f11d3ef1ed360b703e7ea91b/hack/correct-image.sh)。

## 选项

```
用法: orca.sh [-w|--write] [-s|--strategy prepend|replace] <文件或目录>...

  -w, --write                 写回文件；默认只预览
  -s, --strategy STRATEGY     prepend=加 m.daocloud.io 前缀（默认，官方推荐）
                              replace=按官方 registry 前缀替换表
  -h, --help                  显示帮助
```

## 支持的文件和位置

| 文件 | 识别规则（大小写不敏感） | 改写位置 |
| --- | --- | --- |
| Dockerfile | `Dockerfile*`、`Containerfile*`、`*.dockerfile`、`*.containerfile` | `FROM` 的镜像参数 |
| Docker Compose | `compose*.yml/yaml`、`docker-compose*.yml/yaml` | `image:` 字段 |
| 其他 YAML | 其余 `*.yml` / `*.yaml` | `image:` 字段 |

“其他 YAML”包括 Kubernetes manifest，也可能包括 CI 配置或 Helm values 中恰好名为 `image:` 的字段。工具只按文本规则改写 `image:` 的值，不解析 YAML 语义。

输出示例：

```text
DRY-RUN strategy=prepend: scanned=3 changed=2 written=0 skipped_values=1
  Dockerfile:1: nginx:1.27 -> m.daocloud.io/docker.io/library/nginx:1.27
  deployment.yaml:8: quay.io/prometheus/prometheus:v2.55.0 -> m.daocloud.io/quay.io/prometheus/prometheus:v2.55.0
提示: 加 --write 才会真正写回文件
```

## 改写规则与安全性

- 默认 dry-run；只有 `--write` 才会写文件。
- 幂等：已经带 DaoCloud 加速前缀的镜像会被跳过。
- 跳过 `FROM scratch`、`${VAR}`、`{{ ... }}` 等模板值。
- 保留原文件的缩进、引号、行尾注释、CRLF 和末尾换行状态。

registry 判定规则：

- 第一段带 `.` 或 `:`、且不在内置公开 registry 列表中的主机，会作为未知/私有 registry 跳过，例如 `registry.internal.example.com/team/app`。
- `localhost` 会被跳过。
- 第一段不带点号或冒号时，按 Docker Hub 命名空间处理，例如 `myteam/app` 会被当作 Docker Hub 镜像。
- `replace` 模式下，官方替换表里没有的 registry 会跳过，例如 `public.ecr.aws`。

## `replace` 模式支持的官方映射

`replace` 模式只覆盖官方 README 列出的 11 项：

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

来源：[DaoCloud public-image-mirror README](https://github.com/DaoCloud/public-image-mirror/blob/a195786892746d03f11d3ef1ed360b703e7ea91b/README.md)。

## 已知限制

- 改写地址不等于一定能拉取：镜像还需要在 DaoCloud [白名单](https://github.com/DaoCloud/public-image-mirror/blob/a195786892746d03f11d3ef1ed360b703e7ea91b/allows.txt)中，且服务有缓存和限流，见[官方说明](https://github.com/DaoCloud/public-image-mirror/issues/2328)。
- 脚本内置的公开 registry 列表是上述 `allows.txt` 的一次快照；官方新增 registry 后，需要更新脚本才会识别。
- 官方更推荐用 `@sha256:`，其次是明确 tag，最后才是 `latest`；本工具只保留原有 tag/digest，不主动添加 `:latest`。
- 这是静态文件改写工具，不是运行时 Webhook；Helm 模板里的动态值不会被改写。
- 官方未说明公共 `m.daocloud.io` 是否支持源 registry 自带端口（如 `example.com:5000/repo`），因此这类引用会被跳过。参考：[官方本地缓存文档](https://github.com/DaoCloud/public-image-mirror/tree/a195786892746d03f11d3ef1ed360b703e7ea91b/docs/local-cache)。

## 测试

```bash
sh orca_test.sh
```

测试覆盖 11 项官方替换前缀、Docker Hub 简写规范化、tag 与 `@sha256:` digest 保留、`scratch`/模板/未知 registry 跳过、两种策略幂等、CRLF 保留和 `--write` 写回。

## 参考

- [DaoCloud public-image-mirror](https://github.com/DaoCloud/public-image-mirror)
- [DaoCloud 官方文档：镜像加速站](https://docs.daocloud.io/community/mirror/)
- [白名单 & 限流说明](https://github.com/DaoCloud/public-image-mirror/issues/2328)
