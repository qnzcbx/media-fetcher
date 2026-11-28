# 第三方组件与素材声明

## 一、Windows 完整发布包随附运行组件

Media Fetcher Windows (amd64) 完整发布包在 `tools/` 目录随附以下独立外部命令行程序（Go 主程序仅通过 `os/exec` 进程级调用，不链接第三方编解码或解析库）。完整许可全文、依赖清单与可实际获取的固定版本对应源码说明均随包附带于 `licenses/` 目录。

| 组件 | 固定版本 | 上游来源与原始归档 | 实际分发许可 | 对应源码与许可材料 |
| --- | --- | --- | --- | --- |
| **FFmpeg / ffprobe** | `n9.0.2-39-g7adfbe40dc` (`autobuild-2026-10-31-12-59`) | [BtbN/FFmpeg-Builds](https://github.com/BtbN/FFmpeg-Builds) (`ffmpeg-n9.0.2-39-g7adfbe40dc-win64-lgpl-shared-9.0.zip`) | **LGPL-3.0-or-later**（共享库形式，支持用户替换同 ABI 动态库） | `licenses/ffmpeg/LICENSE-LGPL-3.0.txt`、`licenses/ffmpeg/SOURCE.md` |
| **yt-dlp** | `2026.11.23` | [yt-dlp/yt-dlp](https://github.com/yt-dlp/yt-dlp) (`yt-dlp_win.zip`，经官方 GPG 公钥 `AC0CBBE6848D6A873464AF4E57CF65933B5A7581` 验签) | **项目源码：The Unlicense；官方 PyInstaller 构建整体包含 GPLv3+ / GPL-2.0-or-later 等组件（如 PyInstaller / mutagen），以及 Python (PSF-2.0)、OpenSSL 3 (Apache-2.0)、certifi (MPL-2.0)、yt-dlp-ejs (Unlicense / Meriyah ISC / Astring MIT) 等，须按 GPLv3+ 实际许可义务处理** | `licenses/yt-dlp/LICENSE-UNLICENSE.txt`、`LICENSE-GPL-3.0.txt`、`LICENSE-GPL-2.0.txt`、`SOURCE.md` 及 `tools/yt-dlp/_internal/THIRD_PARTY_LICENSES.txt` |
| **Deno** | `v2.9.8` | [denoland/deno](https://github.com/denoland/deno) (`deno-x86_64-pc-windows-msvc.zip`) | **MIT License** | `licenses/deno/LICENSE-MIT.txt`、`licenses/deno/SOURCE.md` |

## 二、Arcticons 品牌图标

| 图标 | 署名 | 原始素材 | 本项目调整 |
| --- | --- | --- | --- |
| arcticons:bilibili | Donnnno 与 Arcticons 贡献者 | [Iconify 条目](https://icon-sets.iconify.design/arcticons/bilibili/) | 转为结构化 SVG 数据，默认描边调整为 2.5，增加可选蓝色 |
| arcticons:lofter | Donnnno 与 Arcticons 贡献者 | [Iconify 条目](https://icon-sets.iconify.design/arcticons/lofter/) | 转为结构化 SVG 数据，默认描边调整为 3.0，增加可选绿色 |
| arcticons:youtube | Donnnno 与 Arcticons 贡献者 | [Iconify 条目](https://icon-sets.iconify.design/arcticons/youtube/) | 转为结构化 SVG 数据，默认描边调整为 2.5，增加可选红色 |

素材来自 [Arcticons](https://github.com/Arcticons-Team/Arcticons)，按 [Iconify 集合数据](https://github.com/iconify/icon-sets/blob/master/json/arcticons.json) 于 2026-11-25 收录，使用集合中公布的署名。原始路径和 48 × 48 坐标系保持不变。

原图标与本项目调整版均按 [Creative Commons Attribution-ShareAlike 4.0 International](https://creativecommons.org/licenses/by-sa/4.0/) 提供。[许可全文](https://creativecommons.org/licenses/by-sa/4.0/legalcode)。此声明适用于所列图标素材。平台标识用于标明支持的服务，未表示官方合作关系。
