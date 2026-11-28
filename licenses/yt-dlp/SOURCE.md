# yt-dlp Windows 官方目录版许可证与对应源码说明

## 1. 随包构件锁定信息

- **上游项目**：yt-dlp (<https://github.com/yt-dlp/yt-dlp>)
- **固定版本**：`2026.11.23`
- **原始发布附件**：`yt-dlp_win.zip`（官方 PyInstaller 目录版，含 `yt-dlp.exe` 与 `_internal/`）
- **原始附件 URL**：<https://github.com/yt-dlp/yt-dlp/releases/download/2026.11.23/yt-dlp_win.zip>
- **原始附件 SHA-256**：`824860984b96ca01de09f98f7bfd74ed7bbdce72a3340b51af8b9dc5ee037a5f`
- **官方签名校验**：发布清单 `SHA2-256SUMS` 与签名 `SHA2-256SUMS.sig` 经 yt-dlp 官方 GPG 公钥（指纹 `AC0CBBE6848D6A873464AF4E57CF65933B5A7581`，`Simon Sawicki (yt-dlp signing key)`）核验。

## 2. 实际许可证构成（重要修正说明）

- `yt-dlp` 自身项目源码采用 **The Unlicense**（见 `LICENSE-UNLICENSE.txt`）。
- **但官方 PyInstaller 打包出的 `yt-dlp_win.zip` 可执行分发包包含第三方 GPL 许可组件**（含 PyInstaller 打包运行件与 `mutagen` 等 GPL-2.0-or-later / GPL-3.0-or-later 兼容许可链，详见随包 `tools/yt-dlp/_internal/THIRD_PARTY_LICENSES.txt`），因此**整套 `yt-dlp_win.zip` 对应可执行构建必须按实际 GPLv3+ / GPLv2+ 许可义务随附许可原文与对应源码获取途径**。
- 同目录同时提供：
  - `LICENSE-UNLICENSE.txt`（yt-dlp 项目源码）
  - `LICENSE-GPL-3.0.txt`（GNU GPL v3.0 原文）
  - `LICENSE-GPL-2.0.txt`（GNU GPL v2.0 原文，覆盖 `mutagen` 等 GPL-2.0-or-later 组件）
  - 随包运行目录中的完整聚合许可声明：`tools/yt-dlp/_internal/THIRD_PARTY_LICENSES.txt`

### Windows 目录版关键内置依赖许可核查摘要
依据 `tools/yt-dlp/_internal/THIRD_PARTY_LICENSES.txt`（共 4725 行）：
- **Python**：PSF-2.0 (<https://www.python.org/>)
- **yt-dlp-ejs**（YouTube EJS 求解脚本依赖）：yt-dlp-ejs (Unlicense) + **Meriyah** (ISC) + **Astring** (MIT)
- **mutagen**：GPL-2.0-or-later (<https://mutagen.readthedocs.io/>)
- **OpenSSL 3.0+ / requests / Tornado / BoringSSL**：Apache-2.0
- **certifi**：MPL-2.0 (<https://github.com/certifi/python-certifi>)
- **curl_cffi / curl-impersonate / urllib3 / brotli / libffi / Expat / charset-normalizer / c-ares / nghttp2 / ngtcp2 / nghttp3**：MIT
- **websockets / httpx / zstd / idna / pycparser / PyCryptodome / mpdecimal**：BSD-2-Clause / BSD-3-Clause / Public Domain
- **zlib / bzip2 / liblzma / SQLite**：zlib / bzip2-1.0.6 / 0BSD / Public Domain

## 3. 实际可获取的固定版本对应源码与构建材料

1. **yt-dlp 2026.11.23 官方对应源码归档（由官方 GPG 清单签名验证）**：
   - 下载地址：<https://github.com/yt-dlp/yt-dlp/releases/download/2026.11.23/yt-dlp.tar.gz>
   - 官方签名 SHA-256：`8f757547e94c723415fd82bb5da857d8e1b24be2e34218890dfaac134907723e`
   - 对应 Git Tag：<https://github.com/yt-dlp/yt-dlp/tree/2026.11.23>
2. **yt-dlp-ejs 与打包构建脚本**：
   - `yt-dlp` 官方 PyInstaller Windows 构建配方：`pyinst.py` 及 `.github/workflows/build.yml`（均包含在上方 `yt-dlp.tar.gz` 与 `2026.11.23` Tag 中）。
   - `yt-dlp-ejs` 上游源码：<https://github.com/yt-dlp/ejs>
   - `mutagen` 上游源码：<https://github.com/quodlibet/mutagen>
   - `CPython` 上游源码：<https://github.com/python/cpython>
3. **聚合调用边界**：
   - Media Fetcher 仅以独立外部进程方式（`os/exec`）调用 `tools\yt-dlp\yt-dlp.exe` 提取公开视频 JSON 元数据（`--skip-download --dump-single-json`），不嵌入 Python 解释器或导入 `yt-dlp` 代码。
   - 用户可通过设置环境变量 `MEDIA_FETCHER_YTDLP_PATH` 自由替换为自行构建的 `yt-dlp` 版本。
