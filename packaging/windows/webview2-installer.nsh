; Media Fetcher Windows Installer Hook — Microsoft Edge WebView2 Runtime Detection & Bootstrap
; Included by MyGo v0.10.0 via mygo.json windows.installerInclude.
; Official reference: https://learn.microsoft.com/microsoft-edge/webview2/concepts/distribution
;
; Behavior:
; 1. Detects Evergreen WebView2 Runtime pv under EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}
;    across HKLM (32-bit/WOW6432Node & 64-bit views) and HKCU (32-bit & 64-bit views).
; 2. If already installed (pv non-empty and != "0.0.0.0"), does nothing.
; 3. Only when missing: asks user (interactive mode) or downloads Microsoft's official Evergreen Bootstrapper,
;    verifies download origin + Authenticode signature ("Valid" & "Microsoft Corporation"), runs silent install,
;    checks exit code (0 / 3010), and re-verifies registry pv post-install.
; 4. On offline / download failure / signature failure / install failure / user cancel: aborts cleanly with
;    a non-zero error level and NEVER falsely reports installation succeeded.
; 5. Does NOT define customUnInstall — never uninstalls user's WebView2 Runtime or alters system settings.

!define WEBVIEW2_CLIENT_KEY "SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
!define WEBVIEW2_BOOTSTRAPPER_URL "https://go.microsoft.com/fwlink/p/?LinkId=2124703"

Function CheckWebView2Installed
  Push $1
  StrCpy $0 "0"

  ; 1. HKLM 32-bit view (WOW6432Node on 64-bit Windows — Microsoft primary machine-wide Evergreen location)
  SetRegView 32
  ReadRegStr $1 HKLM "${WEBVIEW2_CLIENT_KEY}" "pv"
  StrCmp $1 "" check_hklm64
  StrCmp $1 "0.0.0.0" check_hklm64
  StrCpy $0 "1"
  Goto done_check

check_hklm64:
  ; 2. HKLM 64-bit view
  SetRegView 64
  ReadRegStr $1 HKLM "${WEBVIEW2_CLIENT_KEY}" "pv"
  StrCmp $1 "" check_hkcu32
  StrCmp $1 "0.0.0.0" check_hkcu32
  StrCpy $0 "1"
  Goto done_check

check_hkcu32:
  ; 3. HKCU 32-bit view (per-user Evergreen installation)
  SetRegView 32
  ReadRegStr $1 HKCU "${WEBVIEW2_CLIENT_KEY}" "pv"
  StrCmp $1 "" check_hkcu64
  StrCmp $1 "0.0.0.0" check_hkcu64
  StrCpy $0 "1"
  Goto done_check

check_hkcu64:
  ; 4. HKCU 64-bit view
  SetRegView 64
  ReadRegStr $1 HKCU "${WEBVIEW2_CLIENT_KEY}" "pv"
  StrCmp $1 "" done_check
  StrCmp $1 "0.0.0.0" done_check
  StrCpy $0 "1"

done_check:
  SetRegView 32
  Pop $1
FunctionEnd

!macro customInstall
  Push $0
  Push $1
  Push $2

  Call CheckWebView2Installed
  StrCmp $0 "1" wv2_done

  ; WebView2 Runtime is missing. Prompt in interactive mode; proceed directly in silent (/S) mode.
  IfSilent wv2_install
  MessageBox MB_YESNO|MB_ICONQUESTION "未检测到 Microsoft Edge WebView2 运行时，Media Fetcher 需要该系统组件才能正常显示界面。$\r$\n$\r$\n是否现在从 Microsoft 官方服务器下载并安装 Evergreen WebView2 运行时？" IDYES wv2_install
  MessageBox MB_OK|MB_ICONEXCLAMATION "已取消 WebView2 运行时安装。请先前往 Microsoft 官网安装 WebView2 运行时后再重新运行安装程序。" /SD IDOK
  SetErrorLevel 1602
  Abort "用户取消了 WebView2 运行时安装"

wv2_install:
  DetailPrint "正在下载并校验 Microsoft Edge WebView2 Evergreen 运行时..."
  StrCpy $1 "$WINDIR\Sysnative\WindowsPowerShell\v1.0\powershell.exe"
  IfFileExists $1 +2
  StrCpy $1 "$SYSDIR\WindowsPowerShell\v1.0\powershell.exe"
  IfFileExists $1 +5
  IfSilent +2
  MessageBox MB_OK|MB_ICONSTOP "系统缺少 PowerShell，无法自动下载 Microsoft Edge WebView2 运行时。请手动安装 WebView2 后重试。" /SD IDOK
  SetErrorLevel 10
  Abort "缺少 PowerShell，无法引导安装 WebView2"

  InitPluginsDir
  FileOpen $2 "$PLUGINSDIR\install-webview2.ps1" w
  FileWrite $2 "$$ErrorActionPreference = 'Stop'$\r$\n"
  FileWrite $2 "$$ProgressPreference = 'SilentlyContinue'$\r$\n"
  FileWrite $2 "[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12$\r$\n"
  FileWrite $2 "$$url = '${WEBVIEW2_BOOTSTRAPPER_URL}'$\r$\n"
  FileWrite $2 "$$tmp = Join-Path $$env:TEMP ('mf-wv2-' + [Guid]::NewGuid().ToString('N').Substring(0,8))$\r$\n"
  FileWrite $2 "New-Item -ItemType Directory -Force -Path $$tmp | Out-Null$\r$\n"
  FileWrite $2 "$$setup = Join-Path $$tmp 'MicrosoftEdgeWebview2Setup.exe'$\r$\n"
  FileWrite $2 "function Test-WebView2Installed {$\r$\n"
  FileWrite $2 "  $$sub = 'SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}'$\r$\n"
  FileWrite $2 "  foreach ($$hive in @([Microsoft.Win32.RegistryHive]::LocalMachine, [Microsoft.Win32.RegistryHive]::CurrentUser)) {$\r$\n"
  FileWrite $2 "    foreach ($$view in @([Microsoft.Win32.RegistryView]::Registry32, [Microsoft.Win32.RegistryView]::Registry64)) {$\r$\n"
  FileWrite $2 "      try {$\r$\n"
  FileWrite $2 "        $$base = [Microsoft.Win32.RegistryKey]::OpenBaseKey($$hive, $$view)$\r$\n"
  FileWrite $2 "        $$k = $$base.OpenSubKey($$sub)$\r$\n"
  FileWrite $2 "        if ($$k) {$\r$\n"
  FileWrite $2 "          $$pv = [string]$$k.GetValue('pv', '')$\r$\n"
  FileWrite $2 "          $$k.Close()$\r$\n"
  FileWrite $2 "          if (-not [string]::IsNullOrWhiteSpace($$pv) -and $$pv -ne '0.0.0.0') { return $$true }$\r$\n"
  FileWrite $2 "        }$\r$\n"
  FileWrite $2 "      } catch {}$\r$\n"
  FileWrite $2 "    }$\r$\n"
  FileWrite $2 "  }$\r$\n"
  FileWrite $2 "  return $$false$\r$\n"
  FileWrite $2 "}$\r$\n"
  FileWrite $2 "try {$\r$\n"
  FileWrite $2 "  try {$\r$\n"
  FileWrite $2 "    $$req = [Net.HttpWebRequest]::Create($$url)$\r$\n"
  FileWrite $2 "    $$req.AllowAutoRedirect = $$true$\r$\n"
  FileWrite $2 "    $$req.Timeout = 30000$\r$\n"
  FileWrite $2 "    $$resp = $$req.GetResponse()$\r$\n"
  FileWrite $2 "    $$finalUri = $$resp.ResponseUri$\r$\n"
  FileWrite $2 "    if ($$finalUri.Scheme -ne 'https' -or ($$finalUri.Host -notmatch '(^|\.)(microsoft\.com|msedge\.net)$$')) { $$resp.Close(); exit 15 }$\r$\n"
  FileWrite $2 "    $$inStream = $$resp.GetResponseStream()$\r$\n"
  FileWrite $2 "    $$outStream = [IO.File]::Create($$setup)$\r$\n"
  FileWrite $2 "    try { $$inStream.CopyTo($$outStream) } finally { $$outStream.Close(); $$inStream.Close(); $$resp.Close() }$\r$\n"
  FileWrite $2 "  } catch { exit 10 }$\r$\n"
  FileWrite $2 "  if (-not (Test-Path -LiteralPath $$setup) -or (Get-Item -LiteralPath $$setup).Length -lt 100000) { exit 10 }$\r$\n"
  FileWrite $2 "  $$sig = Get-AuthenticodeSignature -LiteralPath $$setup$\r$\n"
  FileWrite $2 "  if ($$sig.Status -ne 'Valid' -or $$sig.SignerCertificate.Subject -notlike '*Microsoft Corporation*') { exit 20 }$\r$\n"
  FileWrite $2 "  $$proc = Start-Process -FilePath $$setup -ArgumentList '/silent','/install' -Wait -PassThru$\r$\n"
  FileWrite $2 "  if ($$proc.ExitCode -ne 0 -and $$proc.ExitCode -ne 3010) { exit 30 }$\r$\n"
  FileWrite $2 "  if (-not (Test-WebView2Installed)) { exit 40 }$\r$\n"
  FileWrite $2 "  exit 0$\r$\n"
  FileWrite $2 "} finally {$\r$\n"
  FileWrite $2 "  Remove-Item -Recurse -Force -LiteralPath $$tmp -ErrorAction SilentlyContinue$\r$\n"
  FileWrite $2 "}$\r$\n"
  FileClose $2

  System::Call 'Kernel32::SetEnvironmentVariable(t "MF_WV2_SCRIPT", t "$PLUGINSDIR\install-webview2.ps1")'
  nsExec::Exec '"$1" -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "& ([scriptblock]::Create([IO.File]::ReadAllText($$env:MF_WV2_SCRIPT)))"'
  Pop $2
  StrCmp $2 "0" wv2_recheck

  StrCmp $2 "10" wv2_err_dl
  StrCmp $2 "15" wv2_err_origin
  StrCmp $2 "20" wv2_err_sig
  StrCmp $2 "30" wv2_err_exec
  Goto wv2_err_verify

wv2_err_dl:
  IfSilent +2
  MessageBox MB_OK|MB_ICONSTOP "无法下载 Microsoft Edge WebView2 运行时（当前可能处于离线或网络受限状态）。请联网或手动安装官方 WebView2 运行时后重试。" /SD IDOK
  SetErrorLevel 10
  Abort "WebView2 下载失败"

wv2_err_origin:
  IfSilent +2
  MessageBox MB_OK|MB_ICONSTOP "WebView2 下载重定向来源不是受信任的 Microsoft 域名，已中止安装。" /SD IDOK
  SetErrorLevel 15
  Abort "WebView2 下载来源不可信"

wv2_err_sig:
  IfSilent +2
  MessageBox MB_OK|MB_ICONSTOP "下载的 WebView2 引导程序数字签名校验未通过（非有效的 Microsoft Corporation 签名），已中止安装。" /SD IDOK
  SetErrorLevel 20
  Abort "WebView2 签名校验失败"

wv2_err_exec:
  IfSilent +2
  MessageBox MB_OK|MB_ICONSTOP "Microsoft Edge WebView2 运行时安装程序返回错误退出码，安装未成功完成。" /SD IDOK
  SetErrorLevel 30
  Abort "WebView2 安装程序执行失败"

wv2_err_verify:
  IfSilent +2
  MessageBox MB_OK|MB_ICONSTOP "WebView2 安装结束后仍未检测到有效运行时注册表项，安装未成功完成。" /SD IDOK
  SetErrorLevel 40
  Abort "WebView2 安装后复检失败"

wv2_recheck:
  Call CheckWebView2Installed
  StrCmp $0 "1" wv2_done
  IfSilent +2
  MessageBox MB_OK|MB_ICONSTOP "WebView2 安装后检测仍未通过，请手动安装官方 WebView2 运行时后重试。" /SD IDOK
  SetErrorLevel 40
  Abort "WebView2 复检未通过"

wv2_done:
  Pop $2
  Pop $1
  Pop $0
!macroend
