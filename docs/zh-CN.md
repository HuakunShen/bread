# bread 简体中文使用与发布指南

本文档面向第一次安装 bread 的使用者，以及负责维护 CI/CD、Release 和
Homebrew tap 的维护者。

## 1. bread 解决什么问题？

当代码阅读目标已经明确时，通常不需要：

```text
读取 A → 模型回合 → 读取 B → 模型回合 → 读取 C
```

可以让 bread 在一个命令中并行读取：

```text
Agent → bread(A, B, C) → 一个有界结果
```

但依赖关系不能强行 batch。如果必须先看 A 的结果才能知道 B 在哪里，
应先读 A，再读 B。未知路径也应先用搜索工具定位，再把已知的相关范围
交给 bread。

## 2. 安装

### Go

```bash
go install github.com/HuakunShen/bread/cmd/bread@latest
```

如果提示找不到 `bread`，把 `$(go env GOPATH)/bin` 加入 `PATH`。

### macOS/Linux

```bash
curl -fsSL https://raw.githubusercontent.com/HuakunShen/bread/main/install.sh | bash
```

可以用 `BREAD_INSTALL_DIR` 选择其它目录：

```bash
BREAD_INSTALL_DIR="$HOME/bin" \
  curl -fsSL https://raw.githubusercontent.com/HuakunShen/bread/main/install.sh | bash
```

脚本识别 Darwin/Linux 和 amd64/arm64，默认不使用 `sudo`。

### Windows

```powershell
irm https://raw.githubusercontent.com/HuakunShen/bread/main/install.ps1 | iex
```

默认目录是 `%LOCALAPPDATA%\bread\bin`，也可传入 `-InstallDir`。

### Homebrew

推荐使用 fully-qualified formula：

```bash
brew install HuakunShen/tap/bread
```

这个命令会自动添加 `HuakunShen/tap`，并只信任 `bread` 这个 formula。
如果你的 Homebrew 要求显式信任，使用：

```bash
brew tap HuakunShen/tap
brew trust --formula HuakunShen/tap/bread
brew install bread
```

`HuakunShen/tap` 对应 GitHub 仓库 `HuakunShen/homebrew-tap`。如果希望
没有 tap 的用户直接执行 `brew install bread`，公式还需要提交到
`Homebrew/homebrew-core` 并通过审核。

## 3. 基本读取

```bash
bread src/a.go:1-160 src/b.go:300-420
```

范围从 1 开始且两端包含：

| 写法 | 含义 |
| --- | --- |
| `path` | 整个文件 |
| `path:20` | 第 20 行到末尾 |
| `path:20-80` | 第 20 到第 80 行 |

默认每个文件最多返回 2000 行，整个 batch 最多返回 200000 个内容字节。
取消限制：

```bash
bread --max-lines 0 --max-bytes 0 src/a.go src/b.go
```

JSON 模式：

```json
{
  "files": [
    {"path": "src/a.go", "start": 1, "end": 160},
    {"path": "src/b.go", "start": 300, "end": 420}
  ]
}
```

```bash
bread --format json --request request.json
```

## 4. 自升级

```bash
bread upgrade --check
bread upgrade
```

Updater 会获取最新稳定 GitHub Release，按当前平台选择压缩包，下载独立的
`checksums.txt` 并验证 SHA-256，然后只提取 `bread` 可执行文件。
Unix 使用原子替换；Windows 等待当前进程退出后再替换。

`go upgrade` 不能作为本项目的命令：Go 模块不能修改官方 `go` 二进制。
正确命令是 `bread upgrade`。

## 5. Skills 安装

`bread` 自带 `skill` 子命令，不需要额外的 Skills CLI：

```bash
bread skill            # 打印完整 Agent 指南
bread skill --add      # 安装（默认 global）
```

默认 global 安装到 `~/.agents/skills/`，并在 Claude Code 配置目录存在时写入
`~/.claude/skills/`（`$CLAUDE_CONFIG_DIR/skills` 优先），在 Codex 配置目录存在时
写入 `~/.codex/skills/`（`$CODEX_HOME/skills` 优先）。需要把 Skill 提交到
仓库时用 `--project`，需要指定目录时用 `--target agents|claude|codex` 或 `--dir DIR`。

也可以用 Skills CLI 安装仓库里的 Skill：

```bash
npx skills@latest add HuakunShen/bread
```

Skill 的决策树：

```text
不知道路径       → 搜索
一个已知文件     → 原生 read
多个独立文件     → bread 或同一回合的 parallel read
前一个结果决定后一个 → 顺序 read
```

如果 Agent 环境没有 `bread`，Skill 会回退到原生并行读取或一次 shell 命令。

## 6. CI/CD

### CI

Pull Request 和 `main` push 在 Ubuntu、macOS、Windows 上执行格式检查、vet、
race test、普通测试和 CLI 构建。

### Release

```bash
git tag v0.2.0
git push origin v0.2.0
```

GoReleaser 生成 Darwin/Linux/Windows 的 amd64/arm64 archives 和
`checksums.txt`。Updater、安装脚本和 Homebrew 公式都依赖这些稳定的资产名。

### Pages

Pages workflow 发布 `site/`。第一次启用仓库时，将 Pages 的 Source 设置为
GitHub Actions；之后 push 到 `main` 会自动部署。

### Homebrew 自动更新

本仓库当前已启用 Homebrew workflow。每次 Release 发布后，它会把 release
中的 `bread.rb` 同步到 `HuakunShen/homebrew-tap`。token 只需要对 tap 有
Contents 写权限，不需要读取 `bread` repo。

Fork 本仓库时，只有在配置好以下内容后才应启用 workflow：

1. 创建公开的 `HuakunShen/homebrew-tap`；
2. 添加仓库变量 `HOMEBREW_TAP_ENABLED=true`；
3. 添加只对 tap 有写权限的 `HOMEBREW_TAP_TOKEN` secret；
4. 发布一个 Release，确认公式通过安装测试。

## 7. 本地验证

```bash
go test ./...
go test -race ./...
go vet ./...
gofmt -l .
go build -o bread ./cmd/bread
```

验证跨平台编译：

```bash
GOOS=darwin GOARCH=amd64 go build ./cmd/bread
GOOS=darwin GOARCH=arm64 go build ./cmd/bread
GOOS=linux GOARCH=amd64 go build ./cmd/bread
GOOS=linux GOARCH=arm64 go build ./cmd/bread
GOOS=windows GOARCH=amd64 go build ./cmd/bread
GOOS=windows GOARCH=arm64 go build ./cmd/bread
```

## 8. 常见问题

- 找不到 `bread`：检查 Go bin 或 `~/.local/bin` 是否在 `PATH`；
- latest release 失败：仓库还没有稳定 Release，或 GitHub API 暂时不可用；
- checksum 不匹配：停止安装，不要跳过校验；
- 没有对应 archive：当前版本尚未发布该平台/架构；
- Windows 升级未立即变化：关闭当前进程后重新启动；
- Homebrew 找不到公式：优先运行 `brew install HuakunShen/tap/bread`；如果已经手动 tap，则运行 `brew trust --formula HuakunShen/tap/bread` 后再安装。

当前 bread 是本地文件 batch reader，不是 MCP server、AST/LSP 服务、远程文件系统
或编辑器。
