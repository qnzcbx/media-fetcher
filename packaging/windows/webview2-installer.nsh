; Media Fetcher Windows Installer Hook
; Included by MyGo v0.10.0 via mygo.json windows.installerInclude.
;
; Responsibilities:
; 1. customInit (.onInit): Single-instance mutex guard to prevent two concurrent installers
;    from racing on $INSTDIR\tools\* (which causes "Error opening file for writing").
; 2. customInstall (after File /r):
;    a) Post-install runtime integrity verification against $INSTDIR\tools\manifest.json
;       (ensures missing, locked-and-skipped, unwritable, or AV-quarantined files NEVER report success).
;    b) Microsoft Edge WebView2 Evergreen Runtime detection & verified bootstrap
;       (https://learn.microsoft.com/microsoft-edge/webview2/concepts/distribution).
; 3. Never defines customUnInstall — never uninstalls user WebView2 or deletes user data/downloads.

!define WEBVIEW2_CLIENT_KEY "SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
!define WEBVIEW2_BOOTSTRAPPER_URL "https://go.microsoft.com/fwlink/p/?LinkId=2124703"
!define SETUP_MUTEX_NAME "Local\MFetchSetupSingleInstance"

!macro customInit
  Push $0
  Push $1
  System::Call 'kernel32::CreateMutex(i 0, i 0, t "${SETUP_MUTEX_NAME}") i .r0 ?e'
  Pop $1
  StrCmp $1 183 0 setup_mutex_ok
  IfSilent +2
  MessageBox MB_OK|MB_ICONEXCLAMATION "MFetch 安装程序已在运行中，请先完成或关闭已有安装窗口后再重试。" /SD IDOK
  SetErrorLevel 1618
  Abort "另一个 MFetch 安装程序正在运行"
setup_mutex_ok:
  Pop $1
  Pop $0
!macroend

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
  Push $3

  ; --- Step A: Post-install runtime & executable integrity verification (tools\manifest.json) ---
  DetailPrint "正在校验安装文件与随包运行组件完整性..."
  IfFileExists "$INSTDIR\MFetch.exe" mf_exe_ok
  IfSilent +2
  MessageBox MB_OK|MB_ICONSTOP "安装未完成：无法写入主程序文件 ($INSTDIR\MFetch.exe)。$\r$\n$\r$\n请检查安装目录写入权限、关闭占用进程或尝试更换安装目录后重试。" /SD IDOK
  SetErrorLevel 50
  Abort "主程序文件写入失败"

mf_exe_ok:
  IfFileExists "$INSTDIR\tools\manifest.json" mf_verify_manifest
  ; Dev builds without bundled tools do not include tools\manifest.json; only enforce when manifest is present.
  Goto mf_verify_done

mf_verify_manifest:
  StrCpy $1 "$WINDIR\Sysnative\WindowsPowerShell\v1.0\powershell.exe"
  IfFileExists $1 +2
  StrCpy $1 "$SYSDIR\WindowsPowerShell\v1.0\powershell.exe"
  IfFileExists $1 +5
  IfSilent +2
  MessageBox MB_OK|MB_ICONSTOP "系统缺少 PowerShell，无法校验随包运行组件完整性。请手动核验后重试。" /SD IDOK
  SetErrorLevel 51
  Abort "缺少 PowerShell，无法校验安装完整性"

  InitPluginsDir
  FileOpen $2 "$PLUGINSDIR\verify-manifest.ps1" w
  FileWrite $2 "$$ErrorActionPreference = 'Stop'$\r$\n"
  FileWrite $2 "$$dir = $$env:MF_INSTALL_DIR$\r$\n"
  FileWrite $2 "$$errFile = $$env:MF_VERIFY_ERR$\r$\n"
  FileWrite $2 "$$mfPath = Join-Path $$dir 'tools\manifest.json'$\r$\n"
  FileWrite $2 "try {$\r$\n"
  FileWrite $2 "  $$mf = [IO.File]::ReadAllText($$mfPath, [Text.UTF8Encoding]::new($$false)) | ConvertFrom-Json$\r$\n"
  FileWrite $2 "  $$sha = [Security.Cryptography.SHA256]::Create()$\r$\n"
  FileWrite $2 "  $$bad = [Collections.Generic.List[string]]::new()$\r$\n"
  FileWrite $2 "  foreach ($$entry in $$mf.files) {$\r$\n"
  FileWrite $2 "    $$rel = [string]$$entry.path$\r$\n"
  FileWrite $2 "    $$full = Join-Path $$dir ($$rel -replace '/', '\')$\r$\n"
  FileWrite $2 "    if (-not (Test-Path -LiteralPath $$full)) { $$bad.Add('missing: ' + $$rel); continue }$\r$\n"
  FileWrite $2 "    $$fi = Get-Item -LiteralPath $$full$\r$\n"
  FileWrite $2 "    if ($$fi.Length -ne [int64]$$entry.size) { $$bad.Add('size-mismatch: ' + $$rel); continue }$\r$\n"
  FileWrite $2 "    $$fs = [IO.File]::Open($$full, [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::ReadWrite)$\r$\n"
  FileWrite $2 "    try { $$hash = ([BitConverter]::ToString($$sha.ComputeHash($$fs)) -replace '-', '').ToLowerInvariant() } finally { $$fs.Close() }$\r$\n"
  FileWrite $2 "    if ($$hash -ne ([string]$$entry.sha256).ToLowerInvariant()) { $$bad.Add('hash-mismatch: ' + $$rel) }$\r$\n"
  FileWrite $2 "  }$\r$\n"
  FileWrite $2 "  $$sha.Dispose()$\r$\n"
  FileWrite $2 "  if ($$bad.Count -gt 0) {$\r$\n"
  FileWrite $2 "    $$summary = ($$bad | Select-Object -First 3) -join '; '$\r$\n"
  FileWrite $2 "    if ($$bad.Count -gt 3) { $$summary += ' (+' + ($$bad.Count - 3) + ' more)' }$\r$\n"
  FileWrite $2 "    [IO.File]::WriteAllText($$errFile, $$summary, [Text.Encoding]::ASCII)$\r$\n"
  FileWrite $2 "    exit 52$\r$\n"
  FileWrite $2 "  }$\r$\n"
  FileWrite $2 "  exit 0$\r$\n"
  FileWrite $2 "} catch {$\r$\n"
  FileWrite $2 "  [IO.File]::WriteAllText($$errFile, $$_.Exception.Message, [Text.Encoding]::ASCII)$\r$\n"
  FileWrite $2 "  exit 53$\r$\n"
  FileWrite $2 "}$\r$\n"
  FileClose $2

  System::Call 'Kernel32::SetEnvironmentVariable(t "MF_VERIFY_SCRIPT", t "$PLUGINSDIR\verify-manifest.ps1")'
  System::Call 'Kernel32::SetEnvironmentVariable(t "MF_INSTALL_DIR", t "$INSTDIR")'
  System::Call 'Kernel32::SetEnvironmentVariable(t "MF_VERIFY_ERR", t "$PLUGINSDIR\verify-err.txt")'
  nsExec::Exec '"$1" -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "& ([scriptblock]::Create([IO.File]::ReadAllText($$env:MF_VERIFY_SCRIPT)))"'
  Pop $2
  StrCmp $2 "0" mf_verify_done

  StrCpy $3 "部分随包运行组件未正确写入或已被修改"
  IfFileExists "$PLUGINSDIR\verify-err.txt" 0 mf_verify_fail_msg
  FileOpen $0 "$PLUGINSDIR\verify-err.txt" r
  FileRead $0 $3
  FileClose $0

mf_verify_fail_msg:
  IfSilent +2
  MessageBox MB_OK|MB_ICONSTOP "安装未通过完整性校验：$3$\r$\n$\r$\n常见原因：目标文件正被其他程序占用、安全软件实时扫描或隔离了组件，或安装目录权限受限。请关闭相关程序、检查安装目录权限后重新安装，或改用便携版 ZIP。" /SD IDOK
  SetErrorLevel 52
  Abort "随包运行组件完整性校验失败"

mf_verify_done:

  ; --- Step B: WebView2 Evergreen Runtime Detection & Bootstrap ---
  Call CheckWebView2Installed
  StrCmp $0 "1" wv2_done

  ; WebView2 Runtime is missing. Prompt in interactive mode; proceed directly in silent (/S) mode.
  IfSilent wv2_install
  MessageBox MB_YESNO|MB_ICONQUESTION "未检测到 Microsoft Edge WebView2 运行时，MFetch 需要该系统组件才能正常显示界面。$\r$\n$\r$\n是否现在从 Microsoft 官方服务器下载并安装 Evergreen WebView2 运行时？" IDYES wv2_install
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
  Pop $3
  Pop $2
  Pop $1
  Pop $0
!macroend
