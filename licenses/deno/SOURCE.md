# Deno Windows x64 构建来源与许可说明（MIT）

- **上游项目**：Deno (<https://github.com/denoland/deno>)
- **固定版本**：`v2.9.8`
- **原始发布附件**：`deno-x86_64-pc-windows-msvc.zip`
- **原始附件 URL**：<https://github.com/denoland/deno/releases/download/v2.9.8/deno-x86_64-pc-windows-msvc.zip>
- **原始附件 SHA-256**：`cba84b26e0df5ae7f74fc66b86a95b866872a86292ebd84fa1e959927edd93bf`
- **解压产物 `deno.exe` SHA-256**：`924397800533d3eb0ed0c889b20ad70e18b970d93acd8135bc043f425e528266`
- **对应源码归档**：<https://github.com/denoland/deno/archive/refs/tags/v2.9.8.tar.gz>
- **许可证**：MIT License（全文见同目录 `LICENSE-MIT.txt`）
- **调用边界**：Media Fetcher 仅将其路径通过 `--no-js-runtimes --js-runtimes deno:<path>` 传给 `yt-dlp` 用于 YouTube EJS 挑战求解，不执行任意第三方脚本。
