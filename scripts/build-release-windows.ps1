# Full Windows amd64 bundled release build for Media Fetcher:
#   1. Fetch & verify pinned bundled tools (yt-dlp onedir + GPG, Deno, BtbN LGPL shared FFmpeg)
#   2. Optional Go & frontend tests (using staged LGPL FFmpeg, isolated AppData)
#   3. Build frontend + MyGo Windows amd64 executable & NSIS installer
#   4. Package full Portable ZIP (including tools/, licenses/, THIRD_PARTY_NOTICES.md, README.md)
#   5. Verify both build output, Portable ZIP, and optional silent NSIS install via check-bundle-windows.ps1
#   6. Generate SHA256SUMS.txt in dist/windows-amd64/
param(
  [string]$CacheDir,
  [string]$DistDir,
  [switch]$SkipTests,
  [switch]$TestSilentInstall
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Console]::OutputEncoding = [Text.Encoding]::UTF8
Add-Type -AssemblyName System.IO.Compression.FileSystem

$Root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$MygoCfg = Get-Content -Raw -LiteralPath (Join-Path $Root 'mygo.json') | ConvertFrom-Json
$AppName = [string]$MygoCfg.name
$AppSlug = if ($MygoCfg.slug) { [string]$MygoCfg.slug } else { ($AppName.ToLowerInvariant() -replace '[^a-z0-9]+', '-').Trim('-') }
$AppVersion = [string]$MygoCfg.version
if (-not $DistDir) { $DistDir = Join-Path $Root 'dist\windows-amd64' }

function Exec-Checked([string]$Label, [scriptblock]$Action) {
  Write-Host ''
  Write-Host ">>> $Label"
  & $Action
  if ($LASTEXITCODE -ne 0) {
    throw "$Label failed (exit=$LASTEXITCODE)"
  }
}

Push-Location -LiteralPath $Root
try {
  # 1. Fetch & verify bundled tools into resources/windows-amd64
  Write-Host '>>> Step 1/6: Preparing verified bundled runtime components'
  $fetchArgs = @{}
  if ($CacheDir) { $fetchArgs.CacheDir = $CacheDir }
  & (Join-Path $PSScriptRoot 'fetch-bundled-tools-windows.ps1') @fetchArgs

  $stagedFfmpeg = Join-Path $Root 'resources\windows-amd64\tools\ffmpeg\ffmpeg.exe'
  $stagedFfprobe = Join-Path $Root 'resources\windows-amd64\tools\ffmpeg\ffprobe.exe'

  # 2. Run Go & frontend tests against staged LGPL FFmpeg in isolated AppData
  if (-not $SkipTests) {
    $isoTmp = Join-Path $env:TEMP ('mf-release-test-' + [Guid]::NewGuid().ToString('N').Substring(0, 8))
    $isoAppData = Join-Path $isoTmp 'appdata'
    $isoLocal = Join-Path $isoTmp 'localappdata'
    New-Item -ItemType Directory -Force -Path $isoAppData, $isoLocal | Out-Null
    $savedEnv = @{
      APPDATA = $env:APPDATA
      LOCALAPPDATA = $env:LOCALAPPDATA
      MEDIA_FETCHER_FFMPEG_PATH = $env:MEDIA_FETCHER_FFMPEG_PATH
      MEDIA_FETCHER_FFPROBE_PATH = $env:MEDIA_FETCHER_FFPROBE_PATH
    }
    $env:APPDATA = $isoAppData
    $env:LOCALAPPDATA = $isoLocal
    $env:MEDIA_FETCHER_FFMPEG_PATH = $stagedFfmpeg
    $env:MEDIA_FETCHER_FFPROBE_PATH = $stagedFfprobe
    try {
      Exec-Checked 'Step 2/6: Go core & desktop tests (staged LGPL FFmpeg, isolated AppData)' {
        & go test -count=1 ./core/... ./internal/... ./pkg/... ./test/...
      }
      Exec-Checked 'Step 2/6: Frontend contract tests' {
        & pnpm run test:youtube
        if ($LASTEXITCODE -ne 0) { return }
        & pnpm run test:downloads
        if ($LASTEXITCODE -ne 0) { return }
        & pnpm run test:credentials
        if ($LASTEXITCODE -ne 0) { return }
        & pnpm run test:preferences
        if ($LASTEXITCODE -ne 0) { return }
        if (Test-Path -LiteralPath (Join-Path $Root 'frontend\test\runtime-status.test.mjs')) {
          & pnpm run test:runtime
          if ($LASTEXITCODE -ne 0) { return }
        }
        if (Test-Path -LiteralPath (Join-Path $Root 'frontend\test\clipboard.test.mjs')) {
          & pnpm run test:clipboard
        }
      }
    } finally {
      foreach ($k in $savedEnv.Keys) {
        if ($null -eq $savedEnv[$k]) { Remove-Item "Env:$k" -ErrorAction SilentlyContinue }
        else { Set-Item "Env:$k" $savedEnv[$k] }
      }
      Remove-Item -Recurse -Force -LiteralPath $isoTmp -ErrorAction SilentlyContinue
    }
  } else {
    Write-Host '>>> Step 2/6: Skipped tests (-SkipTests)'
  }

  # 3. Build Windows amd64 executable & NSIS installer via MyGo (runs frontend buildCommand)
  # Clean stale build/windows-amd64 output so renamed executables cannot leak into the Portable ZIP.
  $buildWin = Join-Path $Root 'build\windows-amd64'
  if (Test-Path -LiteralPath $buildWin) { Remove-Item -Recurse -Force -LiteralPath $buildWin }
  Exec-Checked 'Step 3/6: MyGo Windows amd64 build (bundled NSIS installer)' {
    & pnpm exec mygo build -platform windows/amd64
  }

  # 4. Assemble release distribution artifacts (NSIS installer + Portable ZIP)
  Write-Host ''
  Write-Host ">>> Step 4/6: Packaging Portable ZIP and assembling $DistDir"
  $setupName = "$AppSlug-setup-$AppVersion-amd64.exe"
  $setupPath = Join-Path $buildWin $setupName
  if (-not (Test-Path -LiteralPath $setupPath)) {
    throw "Expected NSIS installer not found: $setupPath"
  }

  if (Test-Path -LiteralPath $DistDir) { Remove-Item -Recurse -Force -LiteralPath $DistDir }
  New-Item -ItemType Directory -Force -Path $DistDir | Out-Null

  $distSetup = Join-Path $DistDir $setupName
  Copy-Item -Force -LiteralPath $setupPath -Destination $distSetup
  Copy-Item -Force -LiteralPath (Join-Path $Root 'THIRD_PARTY_NOTICES.md') -Destination (Join-Path $DistDir 'THIRD_PARTY_NOTICES.md')

  $portableStageRoot = Join-Path $env:TEMP ('mf-portable-' + [Guid]::NewGuid().ToString('N').Substring(0, 8))
  $portableInner = Join-Path $portableStageRoot "$AppSlug-$AppVersion-windows-amd64"
  New-Item -ItemType Directory -Force -Path $portableInner | Out-Null
  try {
    Get-ChildItem -LiteralPath $buildWin | Where-Object {
      $_.Name -notlike '*-setup-*.exe' -and $_.Name -ne '.mygo-files'
    } | ForEach-Object {
      Copy-Item -Recurse -Force -LiteralPath $_.FullName -Destination (Join-Path $portableInner $_.Name)
    }
    Copy-Item -Force -LiteralPath (Join-Path $Root 'README.md') -Destination (Join-Path $portableInner 'README.md')
    $portableName = "$AppSlug-$AppVersion-windows-amd64-portable.zip"
    $portablePath = Join-Path $DistDir $portableName
    [IO.Compression.ZipFile]::CreateFromDirectory($portableStageRoot, $portablePath, [IO.Compression.CompressionLevel]::Optimal, $false)
  } finally {
    Remove-Item -Recurse -Force -LiteralPath $portableStageRoot -ErrorAction SilentlyContinue
  }

  # 5. Verify bundle integrity, allowlist, licenses, and stripped-PATH tool execution
  Write-Host ''
  Write-Host '>>> Step 5/6: Verifying build output, Portable ZIP, and NSIS installer'
  $chkParams = @{
    BundleDir   = $buildWin
    PortableZip = $portablePath
    Installer   = $distSetup
  }
  if ($TestSilentInstall) { $chkParams.TestSilentInstall = $true }
  & (Join-Path $PSScriptRoot 'check-bundle-windows.ps1') @chkParams

  # 6. Generate SHA256SUMS.txt (lowercase hex + two spaces + filename, UTF-8 without BOM + LF)
  Write-Host ''
  Write-Host '>>> Step 6/6: Generating SHA256SUMS.txt'
  $targets = Get-ChildItem -LiteralPath $DistDir -File | Where-Object { $_.Name -ne 'SHA256SUMS.txt' } | Sort-Object Name
  $sumLines = foreach ($f in $targets) {
    $h = (Get-FileHash -Algorithm SHA256 -LiteralPath $f.FullName).Hash.ToLowerInvariant()
    "$h  $($f.Name)"
  }
  $sumsPath = Join-Path $DistDir 'SHA256SUMS.txt'
  [IO.File]::WriteAllText($sumsPath, (($sumLines -join "`n") + "`n"), [Text.UTF8Encoding]::new($false))

  Write-Host ''
  Write-Host '=== Windows Bundled Release Artifacts ==='
  Get-ChildItem -LiteralPath $DistDir -File | Format-Table Name, Length -AutoSize
  Write-Host '=== SHA256SUMS.txt ==='
  Get-Content -LiteralPath $sumsPath
} finally {
  Pop-Location
}
