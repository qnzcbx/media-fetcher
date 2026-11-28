# Verify a bundled Windows amd64 Media Fetcher directory, NSIS installer, or Portable ZIP.
# Checks allowlisted binaries & SHA-256 against lockfile + tools/manifest.json,
# ensures no forbidden/stray files (ffplay, user data, credentials, pdb), verifies licenses,
# and executes stripped-PATH smoke checks on all bundled tools.
param(
  [string]$BundleDir,
  [string]$Installer,
  [string]$PortableZip,
  [switch]$TestSilentInstall
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Console]::OutputEncoding = [Text.Encoding]::UTF8
Add-Type -AssemblyName System.IO.Compression.FileSystem

$Root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$Lock = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'bundled-tools-windows.json') | ConvertFrom-Json
$MygoCfg = Get-Content -Raw -LiteralPath (Join-Path $Root 'mygo.json') | ConvertFrom-Json
$AppName = [string]$MygoCfg.name
$AppVersion = [string]$MygoCfg.version
$AppId = [string]$MygoCfg.identifier

$Script:Fails = 0
function Pass([string]$m) { Write-Host "  [PASS] $m" }
function Fail([string]$m) { Write-Host "  [FAIL] $m"; $Script:Fails++ }
function Section([string]$m) { Write-Host ''; Write-Host "=== $m ===" }
function Get-Sha256([string]$Path) { (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant() }
function Invoke-Capture([string]$Exe, [string[]]$Arguments) {
  $old = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  try {
    $out = & $Exe @Arguments 2>&1
    $code = $LASTEXITCODE
  } finally {
    $ErrorActionPreference = $old
  }
  return @{ ExitCode = $code; Lines = @($out | ForEach-Object { "$_" }) }
}

function Verify-AppDirectory([string]$Dir, [string]$Label) {
  Section "Checking $Label ($Dir)"
  $exe = Join-Path $Dir "$AppName.exe"
  if (Test-Path -LiteralPath $exe) { Pass "$AppName.exe present ($([math]::Round((Get-Item $exe).Length / 1MB, 2)) MiB)" }
  else { Fail "$AppName.exe missing in $Dir"; return }

  # 1. Verify manifest.json and every file listed in it
  $manifestPath = Join-Path $Dir 'tools\manifest.json'
  if (-not (Test-Path -LiteralPath $manifestPath)) {
    Fail 'tools/manifest.json missing'
    return
  }
  $manifest = Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json
  $manifestMap = @{}
  foreach ($entry in $manifest.files) {
    $rel = [string]$entry.path
    $manifestMap[$rel.ToLowerInvariant()] = $entry
    $full = Join-Path $Dir ($rel.Replace('/', '\'))
    if (-not (Test-Path -LiteralPath $full)) {
      Fail "Manifest file missing: $rel"
      continue
    }
    $item = Get-Item -LiteralPath $full
    if ($item.Length -ne [int64]$entry.size) {
      Fail "Size mismatch for $rel (got=$($item.Length) want=$($entry.size))"
      continue
    }
    $sha = Get-Sha256 $full
    if ($sha -ne ([string]$entry.sha256).ToLowerInvariant()) {
      Fail "SHA-256 mismatch for $rel"
    }
  }
  Pass "Verified $($manifest.files.Count) files declared in tools/manifest.json"

  # 2. Check no undeclared files exist inside tools/ or licenses/
  foreach ($sub in @('tools', 'licenses')) {
    $subPath = Join-Path $Dir $sub
    if (-not (Test-Path -LiteralPath $subPath)) { Fail "Missing directory: $sub/"; continue }
    $actual = Get-ChildItem -LiteralPath $subPath -Recurse -File
    foreach ($f in $actual) {
      $rel = $f.FullName.Substring($Dir.Length).TrimStart('\').Replace('\', '/')
      if ($rel -ieq 'tools/manifest.json') { continue }
      if (-not $manifestMap.ContainsKey($rel.ToLowerInvariant())) {
        Fail "Undeclared stray file in $sub/: $rel"
      }
    }
  }
  Pass 'No undeclared stray files found under tools/ or licenses/'

  # 3. Verify lockfile component hashes explicitly
  $yt = $Lock.components.ytDlp
  $deno = $Lock.components.deno
  $ff = $Lock.components.ffmpeg

  $ytExe = Join-Path $Dir ($yt.executableRelPath.Replace('/', '\'))
  if ((Test-Path -LiteralPath $ytExe) -and ((Get-Sha256 $ytExe) -eq $yt.executableSha256.ToLowerInvariant())) {
    Pass "yt-dlp.exe SHA-256 matches lockfile ($($yt.version))"
  } else { Fail 'yt-dlp.exe missing or SHA-256 mismatch' }

  $denoExe = Join-Path $Dir ($deno.executableRelPath.Replace('/', '\'))
  if ((Test-Path -LiteralPath $denoExe) -and ((Get-Sha256 $denoExe) -eq $deno.executableSha256.ToLowerInvariant())) {
    Pass "deno.exe SHA-256 matches lockfile ($($deno.version))"
  } else { Fail 'deno.exe missing or SHA-256 mismatch' }

  foreach ($prop in $ff.allowedBinFiles.PSObject.Properties) {
    $p = Join-Path $Dir ("tools\ffmpeg\" + $prop.Name)
    if (-not (Test-Path -LiteralPath $p) -or ((Get-Sha256 $p) -ne ([string]$prop.Value).ToLowerInvariant())) {
      Fail "FFmpeg runtime file missing or SHA-256 mismatch: tools/ffmpeg/$($prop.Name)"
    }
  }
  foreach ($ex in $ff.excludedBinFiles) {
    if (Test-Path -LiteralPath (Join-Path $Dir ("tools\ffmpeg\" + $ex))) {
      Fail "Excluded file present: tools/ffmpeg/$ex"
    }
  }
  Pass "FFmpeg runtime allowlist ($(@($ff.allowedBinFiles.PSObject.Properties).Count) files) verified and ffplay excluded"

  # 4. Verify third-party notices & licenses
  $notices = Join-Path $Dir 'THIRD_PARTY_NOTICES.md'
  if ((Test-Path -LiteralPath $notices) -and ((Get-Item -LiteralPath $notices).Length -gt 0)) {
    Pass 'THIRD_PARTY_NOTICES.md present'
  } else { Fail 'THIRD_PARTY_NOTICES.md missing or empty' }

  foreach ($reqLic in $Lock.requiredLicenses) {
    $lp = Join-Path $Dir ($reqLic.Replace('/', '\'))
    if (-not (Test-Path -LiteralPath $lp) -or ((Get-Item -LiteralPath $lp).Length -eq 0)) {
      Fail "Required license missing or empty: $reqLic"
    }
  }
  foreach ($reqYt in $yt.requiredRelFiles) {
    $yp = Join-Path $Dir ($reqYt.Replace('/', '\'))
    if (-not (Test-Path -LiteralPath $yp) -or ((Get-Item -LiteralPath $yp).Length -eq 0)) {
      Fail "Required yt-dlp license/file missing: $reqYt"
    }
  }
  Pass 'All required third-party license texts and source notices present'

  # 5. Ensure no user data, credentials, debug artifacts leaked into bundle
  $forbiddenFiles = Get-ChildItem -LiteralPath $Dir -Recurse -File | Where-Object {
    $_.Extension -in @('.cred', '.pdb', '.map', '.log') -or
    $_.Name -in @('history.json', 'accounts_meta.json', '.DS_Store', 'Thumbs.db', 'ffplay.exe')
  }
  if ($forbiddenFiles) {
    Fail "Forbidden files leaked into bundle: $($forbiddenFiles.Name -join ', ')"
  } else {
    Pass 'No credentials, history, debug symbols, or forbidden binaries in bundle'
  }

  # 6. Stripped-PATH runtime execution smoke check
  $savedPath = $env:PATH
  $savedAppData = $env:APPDATA
  $tmpAppData = Join-Path $env:TEMP ('mf-check-appdata-' + [Guid]::NewGuid().ToString('N').Substring(0, 8))
  New-Item -ItemType Directory -Force -Path $tmpAppData | Out-Null
  $env:PATH = "$env:SystemRoot\System32;$env:SystemRoot;$env:SystemRoot\System32\Wbem"
  $env:APPDATA = $tmpAppData
  try {
    foreach ($chk in @(
      @{ name = 'yt-dlp';  exe = $ytExe;                                      arg = '--version'; want = $yt.version },
      @{ name = 'deno';    exe = $denoExe;                                    arg = '--version'; want = "deno $($deno.version)" },
      @{ name = 'ffmpeg';  exe = (Join-Path $Dir 'tools\ffmpeg\ffmpeg.exe');  arg = '-version';  want = $ff.version },
      @{ name = 'ffprobe'; exe = (Join-Path $Dir 'tools\ffmpeg\ffprobe.exe'); arg = '-version';  want = $ff.version }
    )) {
      $r = Invoke-Capture $chk.exe @($chk.arg)
      $first = ($r.Lines | Where-Object { $_.Trim() -ne '' } | Select-Object -First 1)
      if ($r.ExitCode -ne 0 -or -not $first -or ($first -notlike "*$($chk.want)*")) {
        Fail "Stripped-PATH check failed for $($chk.name): exit=$($r.ExitCode) out='$first'"
      } else {
        Pass "Stripped-PATH $($chk.name): $first"
      }
    }
    $denoEval = Invoke-Capture $denoExe @('eval', 'console.log(6*7)')
    if ($denoEval.ExitCode -eq 0 -and (($denoEval.Lines | Select-Object -Last 1).Trim() -eq '42')) {
      Pass 'Stripped-PATH deno JS execution OK'
    } else { Fail "deno eval failed: $($denoEval.Lines -join ' ')" }

    $encRes = Invoke-Capture (Join-Path $Dir 'tools\ffmpeg\ffmpeg.exe') @('-hide_banner', '-encoders')
    $muxRes = Invoke-Capture (Join-Path $Dir 'tools\ffmpeg\ffmpeg.exe') @('-hide_banner', '-muxers')
    $encTxt = $encRes.Lines -join "`n"
    $muxTxt = $muxRes.Lines -join "`n"
    if (($encTxt -match '\blibmp3lame\b') -and ($encTxt -match '\baac\b') -and ($muxTxt -match '\bmp4\b') -and ($muxTxt -match '\bwebm\b') -and ($muxTxt -match '\bmatroska\b')) {
      Pass 'Stripped-PATH ffmpeg supports mp4/webm/matroska muxing and aac/libmp3lame audio encoding'
    } else {
      Fail 'Stripped-PATH ffmpeg missing required muxers or audio encoders'
    }
  } finally {
    $env:PATH = $savedPath
    $env:APPDATA = $savedAppData
    Remove-Item -Recurse -Force -LiteralPath $tmpAppData -ErrorAction SilentlyContinue
  }
}

$Work = Join-Path $env:TEMP ('mf-check-bundle-' + [Guid]::NewGuid().ToString('N').Substring(0, 8))
New-Item -ItemType Directory -Force -Path $Work | Out-Null

try {
  if (-not $BundleDir -and -not $Installer -and -not $PortableZip) {
    $BundleDir = Join-Path $Root 'build\windows-amd64'
  }

  if ($BundleDir) {
    Verify-AppDirectory (Resolve-Path -LiteralPath $BundleDir).Path 'Build Output Directory'
  }

  if ($PortableZip) {
    Section "Checking Portable ZIP ($PortableZip)"
    if (-not (Test-Path -LiteralPath $PortableZip)) { Fail "Portable ZIP not found: $PortableZip" }
    else {
      $zipSize = (Get-Item -LiteralPath $PortableZip).Length
      Pass ("Portable ZIP size: {0:N2} MiB ({1} bytes)" -f ($zipSize / 1MB), $zipSize)
      $pOut = Join-Path $Work 'portable-中文 路径'
      [IO.Compression.ZipFile]::ExtractToDirectory($PortableZip, $pOut)
      $exeHit = Get-ChildItem -LiteralPath $pOut -Recurse -Filter "$AppName.exe" | Select-Object -First 1
      if (-not $exeHit) { Fail "Portable ZIP missing $AppName.exe" }
      else { Verify-AppDirectory $exeHit.DirectoryName 'Portable ZIP Extracted' }
    }
  }

  if ($Installer) {
    Section "Checking NSIS Installer ($Installer)"
    if (-not (Test-Path -LiteralPath $Installer)) { Fail "Installer not found: $Installer" }
    else {
      $instSize = (Get-Item -LiteralPath $Installer).Length
      Pass ("NSIS installer size: {0:N2} MiB ({1} bytes)" -f ($instSize / 1MB), $instSize)
      if ($TestSilentInstall) {
        $uninstKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\$AppId"
        if (Test-Path $uninstKey) {
          Fail "Refusing -TestSilentInstall because $uninstKey already exists on this host"
        } else {
          $instDir = Join-Path $Work '安装验收 测试目录\Media Fetcher'
          $p = Start-Process -FilePath $Installer -ArgumentList "/S /D=$instDir" -Wait -PassThru
          if ($p.ExitCode -ne 0) {
            Fail "Silent install failed (exit=$($p.ExitCode))"
          } else {
            Pass "Silent install succeeded into $instDir"
            try {
              Verify-AppDirectory $instDir 'NSIS Installed Directory'
            } finally {
              $un = Join-Path $instDir 'Uninstall.exe'
              if (Test-Path -LiteralPath $un) {
                Start-Process -FilePath $un -ArgumentList '/S' -Wait | Out-Null
                for ($i = 0; $i -lt 30 -and (Test-Path -LiteralPath $instDir); $i++) { Start-Sleep -Milliseconds 500 }
              }
              if ((Test-Path -LiteralPath $instDir) -or (Test-Path $uninstKey)) {
                Fail "Silent uninstall left remnants (dirExists=$(Test-Path -LiteralPath $instDir), regExists=$(Test-Path $uninstKey))"
              } else {
                Pass 'Silent uninstall removed installation directory and HKCU Uninstall key cleanly'
              }
            }
          }
        }
      }
    }
  }
} finally {
  Remove-Item -Recurse -Force -LiteralPath $Work -ErrorAction SilentlyContinue
}

Write-Host ''
if ($Script:Fails -eq 0) {
  Write-Host 'ALL BUNDLE CHECKS PASSED.'
  exit 0
}
Write-Host "BUNDLE CHECK FAILED: $($Script:Fails) failure(s)."
exit 1
