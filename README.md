# Media Fetcher

**A lightweight, extensible media extractor and downloader built with Go.**

Media Fetcher 是一个基于 Go 开发的跨站点媒体解析与下载工具，旨在通过统一的解析接口，从不同媒体平台发现可访问的图片、视频和音频资源，并提供安全的凭证管理与下载能力。

项目采用 **Go Core + Site Extractors + Downloader** 的模块化架构，优先支持 Windows，计划使用 [MyGo](https://github.com/egoist/mygo) 构建桌面图形界面。

> **Development Status: Work in Progress**
>
> 当前已实现核心模块与实验性 CLI。图形界面、真实站点登录流程及部分端到端功能仍在开发和验证中。

## Features

### Media Extraction

- **Bilibili** — BV 视频链接解析、分 P 识别、WBI 签名及 DASH 视频/音频流提取。
- **LOFTER** — 公开帖子解析，支持图片和图文帖子中的图片资源。
- **Extensible Extractors** — 站点适配器与通用下载逻辑分离，便于扩展更多平台。
- **Anonymous Mode** — 无须登录即可尝试解析公开资源。

实际可用画质与资源取决于站点响应和当前账号权限。

### Credential Management

- Windows DPAPI 加密凭证存储，无需 CGO。
- 支持持久化 CookieJar、Cookie 过期处理与会话恢复。
- 按站点与账号隔离 HTTP 会话。
- 支持独立匿名 Client。
- 实现 Bilibili 扫码登录协调器及 WebView 登录接口契约。
- 凭证失效时不静默降低用户已选择的画质。

### Download Core

- 流式文件下载与临时文件管理。
- HTTP Range 请求与响应范围校验。
- 下载取消、异常清理与路径安全检查。
- 使用 FFmpeg Stream Copy 合并 DASH 视频和音频，无需重新编码。

## Architecture

```text
media-fetcher/
├── cmd/
│   └── media-fetcher-cli/    # CLI 入口
├── core/
│   ├── auth/                 # 凭证与账号管理
│   ├── extractor/            # 站点解析器
│   │   ├── bilibili/
│   │   └── lofter/
│   └── downloader/           # 下载与 FFmpeg 封装
├── pkg/
│   ├── cookiejar/            # 持久化 CookieJar
│   └── vault/                # 加密凭证存储
├── test/                     # 验收测试
```

核心模块独立于图形界面，可以通过 Go CLI 与自动化测试验证。

## Getting Started

### Requirements

- Go 1.27+
- Windows 10/11（当前主要开发与验证平台）
- Node.js (带有 pnpm 或 bun 等包管理器，用于前端开发与构建)
- FFmpeg（音视频混流功能需要）

### Desktop App (MyGo GUI)

本工具包含基于 MyGo 的图形桌面客户端：

1. **开发调试 (热重载前端与启动桌面)**
   ```bash
   pnpm install
   pnpm run mygo:dev
   ```

2. **生产构建打包**
   ```bash
   pnpm run mygo:build
   ```
   构建后执行：
   ```bash
   ./build/media-fetcher.exe
   ```

### CLI 命令行工具

### Build

克隆仓库后，在项目根目录执行：

```bash
go mod download
go build -o ./bin/media-fetcher-cli.exe ./cmd/media-fetcher-cli
```

### Run

```bash
go run ./cmd/media-fetcher-cli version
```

当前 CLI 包含 `extract`、`accounts`、`bili-qr` 等实验性命令。具体参数与行为以 CLI 实现为准。

### Tests

```bash
go test -count=1 ./...
go vet ./...
```

Windows 环境下可额外验证无 CGO 构建：

```bash
CGO_ENABLED=0 go build ./...
```

## Security

Media Fetcher 将用户凭证视为敏感信息。

- 凭证通过本地加密存储，不上传至项目服务器。
- 不同账号使用独立 CookieJar。
- 匿名请求不携带已保存的账号凭证。
- 不通过读取浏览器私有数据库获取 Cookie。
- 不应将 Cookie、Token 或临时媒体签名 URL 提交到 Git 仓库。

Windows V1 使用当前用户作用域的 DPAPI 保护持久化凭证。加密存储并不替代操作系统账号安全及其他必要的访问控制。
