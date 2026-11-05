# 发布

## 版本

版本号在两处保持一致：

- `mygo.json` 的 `version`（构建时写入可执行文件，并为安装包、磁盘镜像命名）。
- `CHANGELOG.md` 的 `## [<version>]` 小节。

发布新版本：改这两处，提交，打 tag `v<version>`，推送 tag。`release.yml` 只在 `v*.*.*` tag 上运行。

## 本地构建

```sh
go tool mygo build -platform windows/amd64
go tool mygo build -platform darwin/universal
```

产物在 `build/<os>-<arch>/`：

| 平台 | 文件 |
|------|------|
| Windows | `Peekdown.exe`、`peekdown-setup-<version>-amd64.exe` |
| macOS | `Peekdown.app`、`peekdown-<version>.dmg` |
| Linux | `peekdown`、`peekdown_<version>_amd64.deb`、`peekdown-<version>-linux-amd64.tar.gz` |

Windows 安装包由 NSIS 生成。Windows 上 `mygo build` 会自动下载 NSIS 到 `%LOCALAPPDATA%\mygo`；macOS 和 Linux 需要先 `brew install makensis` 或 `apt install nsis`，没有时跳过安装包。`-skip-installer` 可以强制跳过。

release 构建统一使用：

```sh
go build -trimpath -tags mygo_noinspector \
  -ldflags "-s -w -H=windowsgui -X github.com/egoist/mygo.production=1"
```

`go tool mygo build` 内部已经使用等价的生产参数。

## CI 与发布产物

`.github/workflows/ci.yml` 在 Windows、macOS、Linux 上跑 `go build ./...`、`go vet ./...`、`go test ./...`。测试用 mygo 的 headless tester，不需要窗口或 WebView。

`.github/workflows/release.yml` 在 `v*.*.*` tag 上：

- Windows：测试、构建、把 `Peekdown.exe` 打成 `peekdown-v<version>-windows-x64.zip`，写同名的 `.sha256`（只含 hash），连同 `peekdown-setup-<version>-amd64.exe` 上传到 GitHub Release。
- macOS：测试、构建 `darwin/universal`，把 `Peekdown.app` 打成 `peekdown-v<version>-macos-universal.zip`，连同 `.dmg` 上传。

未启用代码签名与公证：macOS 是 ad hoc 签名，Windows 安装包未签名。需要分发时再补 `macos.signingIdentity`、`macos.notarize`、`windows.certificate`，并把证书放进仓库 Secrets。

## Scoop

Scoop 清单在 `windedge/ladle-bucket` 仓库的 `bucket/peekdown.json`。原来的清单指向 Rust 版的 `peekdown-v<version>-windows-x64.zip`，新仓库沿用同一个文件名，所以只需要把 `checkver` 指向新仓库，并更新描述与 `bin`。

模板在 `packaging/scoop/peekdown.json.template`。发布后：

1. 从 Release 取 zip 的 SHA256，或直接运行 `scoop checkver -u peekdown` 让它自动填。
2. 把清单提交到 `windedge/ladle-bucket`。
3. 用户可以 `scoop bucket add ladle https://github.com/windedge/ladle-bucket` 后 `scoop install ladle/peekdown`。

## 性能验收

测量脚本在 `scripts/perf/`，来自研究分支 `research/perf-baseline`：

```powershell
go build -trimpath -tags mygo_noinspector -ldflags "-s -w -H=windowsgui -X github.com/egoist/mygo.production=1" -o C:/tmp/perf/peekdown.exe .
./scripts/perf/Measure-ColdStart.ps1 -Exe C:/tmp/perf/peekdown.exe -Runs 10
./scripts/perf/Measure-Memory.ps1 -Exe C:/tmp/perf/peekdown.exe -AppArgs C:/tmp/perf/doc-50k.md -Runs 3 -IdleSec 6
./scripts/perf/Invoke-ScrollTest.ps1 -Exe C:/tmp/perf/peekdown.exe -AppArgs C:/tmp/perf/doc-10k.md -Seconds 4
```

原始基线数字见工单 #13 与 `research/perf-baseline` 分支的报告。
