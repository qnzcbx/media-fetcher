# 随包第三方运行时许可证与对应源码说明

Media Fetcher Windows (amd64) 完整发布包在安装目录 `tools/` 下附带了三个独立外部命令行程序（通过 `os/exec` 进程级参数数组调用，Go 主程序不静态或动态链接其库文件）。

为完整履行各组件的开源许可义务，本目录随包提供许可原文、固定版本对应源码获取地址及构建重现说明：

| 目录 | 组件 | 固定版本 | 实际分发许可 | 关键合规说明 |
| --- | --- | --- | --- | --- |
| `ffmpeg/` | FFmpeg / ffprobe 及共享库 | `n9.0.2-39-g7adfbe40dc` (`BtbN` `autobuild-2026-10-31-12-59`) | **LGPL-3.0-or-later** | 随附 `LICENSE-LGPL-3.0.txt` 与 `SOURCE.md`；以共享 DLL 形式存放于 `tools/ffmpeg/`，允许用户按 LGPL §4(d) 替换为同 ABI 的自编译动态库 |
| `yt-dlp/` | yt-dlp Windows 目录版 (`yt-dlp_win.zip`) | `2026.11.23` | **项目源码：The Unlicense；官方 PyInstaller 整体构建包含 GPLv3+ / GPL-2.0-or-later、PSF-2.0、Apache-2.0、MPL-2.0、MIT、BSD、ISC 等组件，对应可执行包须按 GPLv3+ 实际许可义务处理** | 随附 `LICENSE-UNLICENSE.txt`、`LICENSE-GPL-3.0.txt`、`LICENSE-GPL-2.0.txt`、`SOURCE.md` 及运行时 `tools/yt-dlp/_internal/THIRD_PARTY_LICENSES.txt`；提供官方签名校验的对应源码归档 `yt-dlp.tar.gz` 与依赖清单 |
| `deno/` | Deno Windows x64 构建 | `v2.9.8` | **MIT License** | 随附 `LICENSE-MIT.txt` 与 `SOURCE.md` |

此外，仓库根目录与安装目录下的 `THIRD_PARTY_NOTICES.md` 同时包含 Arcticons 品牌图标的 CC BY-SA 4.0 署名声明。
