# FFmpeg / ffprobe 对应源码与构建说明（LGPL-3.0-or-later）

## 1. 随包构件锁定信息

- **上游项目**：FFmpeg (<https://ffmpeg.org/>)
- **构建来源**：`BtbN/FFmpeg-Builds` (<https://github.com/BtbN/FFmpeg-Builds>)
- **固定发布 Tag**：`autobuild-2026-10-31-12-59`（BtbN 月末永久保留构建）
- **原始发布附件**：`ffmpeg-n9.0.2-39-g7adfbe40dc-win64-lgpl-shared-9.0.zip`
- **原始附件 URL**：<https://github.com/BtbN/FFmpeg-Builds/releases/download/autobuild-2026-10-31-12-59/ffmpeg-n9.0.2-39-g7adfbe40dc-win64-lgpl-shared-9.0.zip>
- **原始附件 SHA-256**：`e907b01fa4783913151237e37df2a10fa97d99a1598a021bf59d4ae62ffed4c0`
- **许可证**：GNU Lesser General Public License v3.0 or later（全文见同目录 `LICENSE-LGPL-3.0.txt`）

## 2. 实际可获取的固定版本对应源码与构建材料

1. **FFmpeg 对应源码（固定提交 `7adfbe40dc`）**：
   - Git 仓库：<https://git.ffmpeg.org/ffmpeg.git>（`release/9.0` 分支，提交 `7adfbe40dc`）
   - 固定提交源码快照：<https://github.com/FFmpeg/FFmpeg/archive/7adfbe40dc.tar.gz>
2. **BtbN 构建脚本与全部第三方编解码库补丁**：
   - 构建脚本仓库：<https://github.com/BtbN/FFmpeg-Builds>（该构建对应 2026-10-31 构建脚本与 `variants/win64-lgpl-shared.sh`、`scripts.d/` 配置）
   - 完整编译参数可在运行随包 `tools\ffmpeg\ffmpeg.exe -buildconf` 时实时查看（包含 `--enable-version3 --enable-shared --disable-static --disable-libx264 --disable-libx265` 等纯 LGPLv3 选项）。
3. **本仓库获取与验证脚本**：
   - 参见本仓库 `scripts/bundled-tools-windows.json` 与 `scripts/fetch-bundled-tools-windows.ps1`。

## 3. LGPL 共享库替换与聚合调用说明

- Media Fetcher 仅通过 `os/exec` 独立外部进程调用 `tools\ffmpeg\ffmpeg.exe` 与 `tools\ffmpeg\ffprobe.exe`，Go 主程序未链接任何 `libav*` 符号。
- 随包产物仅保留运行所需的 `ffmpeg.exe`、`ffprobe.exe` 及 7 个 LGPL 共享动态库（`avcodec-63.dll`、`avdevice-63.dll`、`avfilter-12.dll`、`avformat-63.dll`、`avutil-61.dll`、`swresample-7.dll`、`swscale-10.dll`），已排除 `ffplay.exe` 与开发头文件。
- 终端用户可按 LGPL-3.0 条款自行编译相同主版本号（ABI 兼容）的 LGPL FFmpeg 动态库，直接替换安装目录 `tools\ffmpeg\` 下的对应 DLL，或通过环境变量 `MEDIA_FETCHER_FFMPEG_PATH` / `MEDIA_FETCHER_FFPROBE_PATH` 指向自编译版本。
