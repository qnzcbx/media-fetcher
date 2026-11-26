# Media Fetcher

Media Fetcher 是基于 Go Core、MyGo 和 Vue 3 的桌面媒体解析与下载应用。当前主要在 Windows 上开发和验证。桌面端与 CLI 共用站点解析、凭证和下载核心。

## Features

- 从纯链接或分享文案中识别 Bilibili 视频与 LOFTER 公开帖子。
- 预览视频封面和图片缩略图，选择实际可用的视频画质、音轨或多张图片。
- 通过 Bilibili 二维码在本机登录，使用当前账号权限获取可用媒体。
- 「我的下载」以封面与缩略图展示逐资源进度，支持取消、重试和浏览本地历史。
- 下载完成后可打开原页面、本地文件或所在目录。移除记录会保留下载文件。视频通过 FFmpeg Stream Copy 合并音轨。

- Bilibili 支持完整 MP4、无声 MP4、M4A 和 MP3，桌面端遇到同名文件自动加序号。
- 下载详情显示实际文件大小和可读取的媒体时长，图库支持 WebP 缩略图。

## Supported Sites

| 站点 | 支持范围 |
| --- | --- |
| Bilibili | BV 视频链接、分 P、匿名解析和本地扫码授权。画质及音轨以实际响应为准。 |
| LOFTER | 匿名解析公开图片与图文帖子，支持图片多选下载。 |

Bilibili 短链请先展开为完整视频链接。

## Getting Started

在 Windows 上准备 Go 1.27.4（见 `go.mod`）、能运行项目 Vite 版本的 Node.js，以及 pnpm。本次构建使用 Node.js 22.22.2 和 pnpm 11.1.2。完整视频合并、需要重新封装的轨道和 MP3 转码需要 FFmpeg。MP3 使用 libmp3lame。建议同时将 FFprobe 加入 PATH，用于确认单轨容器及读取实际媒体时长。

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

当前处于 Windows 桌面预览阶段。可用媒体受站点接口和账号权限影响。下载历史和应用缩略图保存在 Windows 用户数据目录的 `media-fetcher/downloads` 下。当前每次运行一个下载任务。

运行 `pnpm run test:credentials` 检查前端凭证契约，运行 `go test -count=1 ./...` 检查 Go 核心与桌面服务。

## 第三方素材

Bilibili 与 LOFTER 线性图标来自 Arcticons，按 CC BY-SA 4.0 授权。详见 [第三方素材声明](THIRD_PARTY_NOTICES.md)。

下载位置可在设置中更改或恢复系统默认目录，合法选择会自动保存。正在下载的任务和重试使用原任务目录，历史文件保持原位置。自定义目录失效时需重新选择。

图库中的「移除记录」保留媒体文件。「删除文件」经确认后将可核验的本地文件移至 Windows 回收站，并清理对应记录。旧记录或身份不一致的文件会保守拒绝删除。

图库统一使用「删除」确认入口。默认勾选的本地文件操作会移至回收站，取消勾选仅清理记录。开发期旧记录使用相同的确认界面，后端仍校验当前文件快照。
