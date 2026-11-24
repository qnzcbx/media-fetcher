# Media Fetcher

Media Fetcher 是基于 Go Core、MyGo 和 Vue 3 的桌面媒体解析与下载应用。当前主要在 Windows 上开发和验证。桌面端与 CLI 共用站点解析、凭证和下载核心。

## Features

- 从纯链接或分享文案中识别 Bilibili 视频与 LOFTER 公开帖子。
- 预览视频封面和图片缩略图，选择实际可用的视频画质、音轨或多张图片。
- 通过 Bilibili 二维码在本机登录，使用当前账号权限获取可用媒体。
- 查看下载进度或取消下载，使用 FFmpeg Stream Copy 合并音视频。

## Supported Sites

| 站点 | 支持范围 |
| --- | --- |
| Bilibili | BV 视频链接、分 P、匿名解析和本地扫码授权。画质及音轨以实际响应为准。 |
| LOFTER | 匿名解析公开图片与图文帖子，支持图片多选下载。 |

Bilibili 短链请先展开为完整视频链接。

## Getting Started

在 Windows 上准备 Go 1.27.4（见 `go.mod`）、能运行项目 Vite 版本的 Node.js，以及 pnpm。本次构建使用 Node.js 22.22.2 和 pnpm 11.1.2。音视频合并需要 FFmpeg，可将其加入 PATH。

从仓库根目录运行：

```bash
pnpm install
pnpm run mygo:dev
pnpm run mygo:build
```

开发模式启动 Vue 和 MyGo 窗口。生产版使用 `mygo:build` 构建并嵌入 Vue 资源，普通 `go build` 只编译 Go 程序。启动生产版：

```bash
./build/windows-amd64/media-fetcher.exe
```

## CLI

示例使用公开 BV 链接，`ACCOUNT_ID` 请替换为 `accounts` 列出的账号 ID。执行 `bili-qr` 时按终端提示扫码。

```bash
go run ./cmd/media-fetcher-cli version
go run ./cmd/media-fetcher-cli accounts
go run ./cmd/media-fetcher-cli bili-qr
go run ./cmd/media-fetcher-cli extract 'https://www.bilibili.com/video/BV1xx411c7mD'
go run ./cmd/media-fetcher-cli extract -account ACCOUNT_ID 'https://www.bilibili.com/video/BV1xx411c7mD'
go run ./cmd/media-fetcher-cli download -output '.\downloads' 'https://www.bilibili.com/video/BV1xx411c7mD'
go run ./cmd/media-fetcher-cli download -audio-only -output '.\downloads' 'https://www.bilibili.com/video/BV1xx411c7mD'
```

## Architecture

Vue 通过 MyGo IPC 调用 `DesktopService`。项目无需独立 HTTP API Server。`frontend/src/mygo.ts` 由 MyGo 自动生成，请通过生成命令更新绑定。

```text
cmd/
  media-fetcher-cli/  命令行入口
  media-fetcher-app/ MyGo 桌面入口
core/
  auth/              登录与账号会话
  extractor/         站点解析
  downloader/        下载与 FFmpeg 封装
internal/desktop/    DesktopService 与桌面流程
pkg/                 凭证保险库、CookieJar 和文件工具
frontend/src/
  api/               IPC 调用封装
  components/        页面与弹窗组件
  composables/       凭证、解析、预览及下载状态
  utils/             链接与媒体工具
  styles/            全局样式
  mygo.ts            生成的 IPC 绑定
```

## Security

Windows 凭证通过当前用户作用域的 DPAPI 加密后保存在本机。账号会话按站点和账号隔离，匿名解析使用独立会话。扫码授权仅使用账号已有的站点权限。

## Development Status

当前处于 Windows 桌面预览阶段。可用媒体受站点接口和账号权限影响。持久化下载图库、下载历史及多任务并发尚未交付。

运行 `pnpm run test:credentials` 检查前端凭证契约，运行 `go test -count=1 ./...` 检查 Go 核心与桌面服务。
