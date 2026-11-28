# Prepare and verify pinned Windows amd64 bundled runtime components for Media Fetcher.
# Sources and exact SHA-256 hashes are locked in scripts/bundled-tools-windows.json.
# Never skips integrity verification on cache hits; verifies yt-dlp GPG signature;
# safely extracts in a fresh temporary directory; atomically replaces resources/windows-amd64.
param(
  [string]$CacheDir,
  [string]$OutputDir,
  [switch]$ForceDownload
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Console]::OutputEncoding = [Text.Encoding]::UTF8
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Add-Type -AssemblyName System.IO.Compression.FileSystem

$Root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$LockPath = Join-Path $PSScriptRoot 'bundled-tools-windows.json'
if (-not (Test-Path -LiteralPath $LockPath)) {
  throw "Lockfile not found: $LockPath"
}
$Lock = Get-Content -Raw -LiteralPath $LockPath | ConvertFrom-Json

if (-not $CacheDir) {
  $CacheDir = if ($env:MEDIA_FETCHER_TOOL_CACHE) { $env:MEDIA_FETCHER_TOOL_CACHE } else { Join-Path $Root '.cache\bundled-tools-windows' }
}
if (-not $OutputDir) {
  $OutputDir = Join-Path $Root 'resources\windows-amd64'
}

New-Item -ItemType Directory -Force -Path $CacheDir | Out-Null

function Get-Sha256([string]$Path) {
  (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant()
}

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

function Find-Gpg {
  $cmd = Get-Command gpg -ErrorAction SilentlyContinue
  if ($cmd) { return $cmd.Source }
  foreach ($p in @(
    "$env:ProgramFiles\Git\usr\bin\gpg.exe",
    "${env:ProgramFiles(x86)}\Git\usr\bin\gpg.exe",
    "$env:ProgramFiles\GnuPG\bin\gpg.exe"
  )) {
    if ($p -and (Test-Path -LiteralPath $p)) { return $p }
  }
  return $null
}

function Download-File([string]$Url, [string]$DestPath) {
  $part = "$DestPath.part"
  if (Test-Path -LiteralPath $part) { Remove-Item -Force -LiteralPath $part }
  $curl = Get-Command curl.exe -ErrorAction SilentlyContinue
  if ($curl) {
    $old = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
      & $curl.Source -fSL --retry 3 --connect-timeout 20 --max-time 600 -A 'media-fetcher-bundler/1.0' -o $part $Url
      $rc = $LASTEXITCODE
    } finally {
      $ErrorActionPreference = $old
    }
    if ($rc -ne 0) {
      Remove-Item -Force -LiteralPath $part -ErrorAction SilentlyContinue
      throw "curl download failed (exit=$rc): $Url"
    }
  } else {
    Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile $part -TimeoutSec 600
  }
  Move-Item -Force -LiteralPath $part -Destination $DestPath
}

function Ensure-VerifiedArchive([string]$Url, [string]$CacheName, [string]$WantSha256) {
  $path = Join-Path $CacheDir $CacheName
  $want = $WantSha256.ToLowerInvariant()
  if ((-not $ForceDownload) -and (Test-Path -LiteralPath $path)) {
    $got = Get-Sha256 $path
    if ($got -eq $want) {
      Write-Host "  [cache-verified] $CacheName (sha256=$got)"
      return $path
    }
    Write-Host "  [cache-corrupt] $CacheName sha256 mismatch (got=$got, want=$want); removing and re-downloading..."
    Remove-Item -Force -LiteralPath $path
  }
  Write-Host "  [download] $CacheName ..."
  Download-File $Url $path
  $got = Get-Sha256 $path
  if ($got -ne $want) {
    Remove-Item -Force -LiteralPath $path -ErrorAction SilentlyContinue
    throw "SHA-256 verification failed for $CacheName : want=$want got=$got"
  }
  Write-Host "  [verified] $CacheName (sha256=$got)"
  return $path
}

function Ensure-CachedTextFile([string]$Url, [string]$CacheName) {
  $path = Join-Path $CacheDir $CacheName
  if ($ForceDownload -or -not (Test-Path -LiteralPath $path) -or (Get-Item -LiteralPath $path).Length -eq 0) {
    Download-File $Url $path
  }
  return $path
}

# Safe ZIP extraction that strictly blocks Zip Slip / path traversal / absolute paths.
function Expand-ZipSafe([string]$ZipPath, [string]$DestDir) {
  New-Item -ItemType Directory -Force -Path $DestDir | Out-Null
  $fullDest = [IO.Path]::GetFullPath($DestDir).TrimEnd('\', '/') + [IO.Path]::DirectorySeparatorChar
  $zip = [IO.Compression.ZipFile]::OpenRead($ZipPath)
  try {
    if ($zip.Entries.Count -eq 0) {
      throw "Archive is empty: $ZipPath"
    }
    foreach ($entry in $zip.Entries) {
      $raw = $entry.FullName
      if ([string]::IsNullOrEmpty($raw)) { continue }
      $norm = $raw.Replace('/', '\')
      if ([IO.Path]::IsPathRooted($norm) -or $norm.StartsWith('\') -or $norm.Contains(':')) {
        throw "Unsafe absolute entry in archive $ZipPath : $raw"
      }
      $segments = $norm.Split('\')
      if ($segments -contains '..') {
        throw "Unsafe path traversal entry in archive $ZipPath : $raw"
      }
      $target = [IO.Path]::GetFullPath((Join-Path $fullDest $norm))
      if (-not $target.StartsWith($fullDest, [StringComparison]::OrdinalIgnoreCase)) {
        throw "Resolved path escapes destination in $ZipPath : $raw -> $target"
      }
      if ($raw.EndsWith('/') -or $raw.EndsWith('\')) {
        New-Item -ItemType Directory -Force -Path $target | Out-Null
      } else {
        $parent = Split-Path -Parent $target
        if ($parent) { New-Item -ItemType Directory -Force -Path $parent | Out-Null }
        [IO.Compression.ZipFileExtensions]::ExtractToFile($entry, $target, $true)
      }
    }
  } finally {
    $zip.Dispose()
  }
}

$Work = Join-Path $CacheDir ('stage-' + [Guid]::NewGuid().ToString('N').Substring(0, 12))
$StageOut = Join-Path $Work 'windows-amd64'
$TmpExtract = Join-Path $Work 'extract'
New-Item -ItemType Directory -Force -Path $StageOut, $TmpExtract | Out-Null

try {
  Write-Host '=== 1. Verifying and staging yt-dlp (official onedir + GPG signature) ==='
  $yt = $Lock.components.ytDlp
  $pubKeyPath = Join-Path $Root $yt.publicKeyFile
  if (-not (Test-Path -LiteralPath $pubKeyPath)) {
    throw "Missing yt-dlp official GPG public key: $pubKeyPath"
  }
  $gotKeyHash = Get-Sha256 $pubKeyPath
  if ($gotKeyHash -ne $yt.publicKeySha256.ToLowerInvariant()) {
    throw "yt-dlp public key SHA-256 mismatch: want=$($yt.publicKeySha256) got=$gotKeyHash"
  }

  $gpg = Find-Gpg
  if (-not $gpg) {
    throw 'gpg.exe not found (install Git for Windows or GnuPG to verify yt-dlp SHA2-256SUMS.sig)'
  }

  $sumsCache = Ensure-CachedTextFile $yt.checksumsUrl "yt-dlp-$($yt.version)-SHA2-256SUMS"
  $sigCache = Ensure-CachedTextFile $yt.signatureUrl "yt-dlp-$($yt.version)-SHA2-256SUMS.sig"

  $gpgWork = Join-Path $Work 'gpg-verify'
  New-Item -ItemType Directory -Force -Path (Join-Path $gpgWork 'gnupg') | Out-Null
  Copy-Item -Force -LiteralPath $pubKeyPath -Destination (Join-Path $gpgWork 'yt-dlp-public.key')
  Copy-Item -Force -LiteralPath $sumsCache -Destination (Join-Path $gpgWork 'SHA2-256SUMS')
  Copy-Item -Force -LiteralPath $sigCache -Destination (Join-Path $gpgWork 'SHA2-256SUMS.sig')

  Push-Location -LiteralPath $gpgWork
  try {
    # 1. Verify official public key fingerprint without spawning gpg-agent
    $fpRes = Invoke-Capture $gpg @('--homedir', 'gnupg', '--batch', '--show-keys', '--with-colons', '--with-fingerprint', 'yt-dlp-public.key')
    if ($fpRes.ExitCode -ne 0) { throw "GPG fingerprint query failed: $($fpRes.Lines -join ' ')" }
    $fpMatch = ($fpRes.Lines | Where-Object { $_ -like 'fpr:*' } | ForEach-Object { ($_ -split ':')[9] } | Select-Object -First 1)
    if ($fpMatch -ne $yt.publicKeyFingerprint.ToUpperInvariant()) {
      throw "yt-dlp GPG fingerprint mismatch: want=$($yt.publicKeyFingerprint) got=$fpMatch"
    }

    # 2. Dearmor key to standalone keyring and verify detached signature with gpgv (or gpg --no-default-keyring), avoiding MSYS2 gpg-agent socket issues
    $dearmor = Invoke-Capture $gpg @('--batch', '--yes', '--dearmor', '-o', 'trusted.gpg', 'yt-dlp-public.key')
    if ($dearmor.ExitCode -ne 0 -or -not (Test-Path -LiteralPath 'trusted.gpg')) {
      throw "GPG key dearmor failed: $($dearmor.Lines -join ' ')"
    }
    $gpgv = Join-Path (Split-Path -Parent $gpg) 'gpgv.exe'
    if (Test-Path -LiteralPath $gpgv) {
      $ver = Invoke-Capture $gpgv @('--homedir', 'gnupg', '--keyring', './trusted.gpg', 'SHA2-256SUMS.sig', 'SHA2-256SUMS')
    } else {
      $ver = Invoke-Capture $gpg @('--homedir', 'gnupg', '--batch', '--no-default-keyring', '--keyring', './trusted.gpg', '--verify', 'SHA2-256SUMS.sig', 'SHA2-256SUMS')
    }
    if ($ver.ExitCode -ne 0) {
      throw "yt-dlp SHA2-256SUMS.sig GPG signature verification FAILED: $($ver.Lines -join ' ')"
    }
    Write-Host "  [gpg-verified] SHA2-256SUMS.sig valid for fingerprint $fpMatch"
  } finally {
    Pop-Location
  }

  $sumsLines = Get-Content -LiteralPath $sumsCache
  $signedHash = ($sumsLines | Where-Object { $_ -match "^([0-9a-fA-F]{64})\s+\*?$([regex]::Escape($yt.asset))$" } | ForEach-Object { $Matches[1].ToLowerInvariant() } | Select-Object -First 1)
  if (-not $signedHash) { throw "Signed SHA2-256SUMS does not contain $($yt.asset)" }
  if ($signedHash -ne $yt.sha256.ToLowerInvariant()) {
    throw "Signed SHA2-256SUMS hash ($signedHash) disagrees with lockfile ($($yt.sha256))"
  }
  $signedSrcHash = ($sumsLines | Where-Object { $_ -match '^([0-9a-fA-F]{64})\s+\*?yt-dlp\.tar\.gz$' } | ForEach-Object { $Matches[1].ToLowerInvariant() } | Select-Object -First 1)
  if ($signedSrcHash -and ($signedSrcHash -ne $yt.sourceSha256.ToLowerInvariant())) {
    throw "Signed yt-dlp.tar.gz source hash ($signedSrcHash) disagrees with lockfile ($($yt.sourceSha256))"
  }

  $ytZip = Ensure-VerifiedArchive $yt.url "yt-dlp-$($yt.version)-$($yt.asset)" $yt.sha256
  $ytDst = Join-Path $StageOut ($yt.targetRelDir.Replace('/', '\'))
  Expand-ZipSafe $ytZip $ytDst

  $ytExe = Join-Path $StageOut ($yt.executableRelPath.Replace('/', '\'))
  if (-not (Test-Path -LiteralPath $ytExe)) { throw "Missing extracted yt-dlp.exe at $ytExe" }
  $gotYtExe = Get-Sha256 $ytExe
  if ($gotYtExe -ne $yt.executableSha256.ToLowerInvariant()) {
    throw "Extracted yt-dlp.exe SHA-256 mismatch: want=$($yt.executableSha256) got=$gotYtExe"
  }
  foreach ($reqRel in $yt.requiredRelFiles) {
    $reqFull = Join-Path $StageOut ($reqRel.Replace('/', '\'))
    if (-not (Test-Path -LiteralPath $reqFull)) { throw "Missing required yt-dlp file: $reqRel" }
  }
  Write-Host "  [staged] yt-dlp $($yt.version) -> $($yt.targetRelDir)"

  Write-Host '=== 2. Verifying and staging Deno ==='
  $deno = $Lock.components.deno
  $denoSumFile = Ensure-CachedTextFile $deno.checksumsUrl "deno-$($deno.version)-sha256sum"
  $denoOfficialSha = ([regex]::Match((Get-Content -Raw -LiteralPath $denoSumFile), '[0-9a-fA-F]{64}')).Value.ToLowerInvariant()
  if ($denoOfficialSha -ne $deno.sha256.ToLowerInvariant()) {
    throw "Deno official .sha256sum ($denoOfficialSha) disagrees with lockfile ($($deno.sha256))"
  }
  $denoZip = Ensure-VerifiedArchive $deno.url "deno-$($deno.version)-$($deno.asset)" $deno.sha256
  $denoTmp = Join-Path $TmpExtract 'deno'
  Expand-ZipSafe $denoZip $denoTmp
  $denoSrcExe = Join-Path $denoTmp 'deno.exe'
  if (-not (Test-Path -LiteralPath $denoSrcExe)) { throw 'deno.exe not found in archive' }
  $gotDenoExe = Get-Sha256 $denoSrcExe
  if ($gotDenoExe -ne $deno.executableSha256.ToLowerInvariant()) {
    throw "Extracted deno.exe SHA-256 mismatch: want=$($deno.executableSha256) got=$gotDenoExe"
  }
  $denoDstDir = Join-Path $StageOut ($deno.targetRelDir.Replace('/', '\'))
  New-Item -ItemType Directory -Force -Path $denoDstDir | Out-Null
  Copy-Item -Force -LiteralPath $denoSrcExe -Destination (Join-Path $StageOut ($deno.executableRelPath.Replace('/', '\')))
  Write-Host "  [staged] Deno v$($deno.version) -> $($deno.executableRelPath)"

  Write-Host '=== 3. Verifying and staging BtbN LGPL Shared FFmpeg / ffprobe ==='
  $ff = $Lock.components.ffmpeg
  $ffSumFile = Ensure-CachedTextFile $ff.checksumsUrl "btbn-$($ff.releaseTag)-checksums.sha256"
  $ffSumsRaw = Get-Content -Raw -LiteralPath $ffSumFile
  $ffMatch = [regex]::Match($ffSumsRaw, "([0-9a-fA-F]{64})\s+\*?$([regex]::Escape($ff.asset))")
  if (-not $ffMatch.Success) { throw "BtbN checksums.sha256 does not contain $($ff.asset)" }
  $ffOfficialSha = $ffMatch.Groups[1].Value.ToLowerInvariant()
  if ($ffOfficialSha -ne $ff.sha256.ToLowerInvariant()) {
    throw "BtbN checksums.sha256 ($ffOfficialSha) disagrees with lockfile ($($ff.sha256))"
  }
  $ffZip = Ensure-VerifiedArchive $ff.url $ff.asset $ff.sha256
  $ffTmp = Join-Path $TmpExtract 'ffmpeg'
  Expand-ZipSafe $ffZip $ffTmp
  $ffInner = Get-ChildItem -LiteralPath $ffTmp -Directory | Select-Object -First 1
  if (-not $ffInner) { throw 'Unexpected BtbN archive structure (missing top directory)' }
  $ffBinSrc = Join-Path $ffInner.FullName 'bin'
  if (-not (Test-Path -LiteralPath $ffBinSrc)) { throw "Missing bin/ in BtbN archive: $ffBinSrc" }

  $ffDstDir = Join-Path $StageOut ($ff.targetRelDir.Replace('/', '\'))
  New-Item -ItemType Directory -Force -Path $ffDstDir | Out-Null

  # Strict allowlist copy: ONLY declared ffmpeg.exe, ffprobe.exe, and runtime DLLs; exclude ffplay.exe & dev files
  $allowedProps = $ff.allowedBinFiles.PSObject.Properties
  foreach ($prop in $allowedProps) {
    $fname = $prop.Name
    $wantFileSha = ([string]$prop.Value).ToLowerInvariant()
    $srcFile = Join-Path $ffBinSrc $fname
    if (-not (Test-Path -LiteralPath $srcFile)) {
      throw "Missing expected FFmpeg runtime file in archive: $fname"
    }
    $gotFileSha = Get-Sha256 $srcFile
    if ($gotFileSha -ne $wantFileSha) {
      throw "FFmpeg file SHA-256 mismatch for $fname : want=$wantFileSha got=$gotFileSha"
    }
    Copy-Item -Force -LiteralPath $srcFile -Destination (Join-Path $ffDstDir $fname)
  }
  # Verify no excluded file snuck in
  foreach ($ex in $ff.excludedBinFiles) {
    if (Test-Path -LiteralPath (Join-Path $ffDstDir $ex)) {
      throw "Excluded binary $ex must not be staged in $ffDstDir"
    }
  }
  Write-Host "  [staged] FFmpeg $($ff.version) ($(@($allowedProps).Count) verified files) -> $($ff.targetRelDir)"

  Write-Host '=== 4. Staging license & corresponding source materials ==='
  $licSrcRoot = Join-Path $Root 'licenses'
  if (-not (Test-Path -LiteralPath $licSrcRoot)) { throw "Missing repository licenses/ directory: $licSrcRoot" }
  Copy-Item -Recurse -Force -LiteralPath $licSrcRoot -Destination (Join-Path $StageOut 'licenses')
  foreach ($reqLic in $Lock.requiredLicenses) {
    $p = Join-Path $StageOut ($reqLic.Replace('/', '\'))
    if (-not (Test-Path -LiteralPath $p) -or (Get-Item -LiteralPath $p).Length -eq 0) {
      throw "Missing or empty required license file: $reqLic"
    }
  }

  Write-Host '=== 5. Stripped-PATH smoke verification on staged binaries ==='
  $savedPath = $env:PATH
  $env:PATH = "$env:SystemRoot\System32;$env:SystemRoot;$env:SystemRoot\System32\Wbem"
  try {
    foreach ($chk in @(
      @{ name = 'yt-dlp';  exe = (Join-Path $StageOut 'tools\yt-dlp\yt-dlp.exe');  arg = '--version'; want = $yt.version },
      @{ name = 'deno';    exe = (Join-Path $StageOut 'tools\deno.exe');           arg = '--version'; want = "deno $($deno.version)" },
      @{ name = 'ffmpeg';  exe = (Join-Path $StageOut 'tools\ffmpeg\ffmpeg.exe');  arg = '-version';  want = $ff.version },
      @{ name = 'ffprobe'; exe = (Join-Path $StageOut 'tools\ffmpeg\ffprobe.exe'); arg = '-version';  want = $ff.version }
    )) {
      $r = Invoke-Capture $chk.exe @($chk.arg)
      $first = ($r.Lines | Where-Object { $_.Trim() -ne '' } | Select-Object -First 1)
      if ($r.ExitCode -ne 0 -or -not $first -or ($first -notlike "*$($chk.want)*")) {
        throw "Smoke check failed for $($chk.name): exit=$($r.ExitCode) out='$first' want='*$($chk.want)*'"
      }
      Write-Host "  [smoke-ok] $($chk.name): $first"
    }
    $encRes = Invoke-Capture (Join-Path $StageOut 'tools\ffmpeg\ffmpeg.exe') @('-hide_banner', '-encoders')
    $encText = $encRes.Lines -join "`n"
    if ($encRes.ExitCode -ne 0 -or ($encText -notmatch '\blibmp3lame\b') -or ($encText -notmatch '\baac\b')) {
      throw 'Staged ffmpeg is missing required audio encoders (aac / libmp3lame)'
    }
  } finally {
    $env:PATH = $savedPath
  }

  Write-Host '=== 6. Generating complete file manifest (tools/manifest.json) ==='
  $allFiles = Get-ChildItem -LiteralPath $StageOut -Recurse -File | Sort-Object FullName
  $fileEntries = @()
  $totalBytes = [int64]0
  foreach ($f in $allFiles) {
    $rel = $f.FullName.Substring($StageOut.Length).TrimStart('\').Replace('\', '/')
    $sha = Get-Sha256 $f.FullName
    $totalBytes += $f.Length
    $fileEntries += [ordered]@{
      path   = $rel
      size   = $f.Length
      sha256 = $sha
    }
  }
  $manifest = [ordered]@{
    schemaVersion = 1
    platform      = $Lock.platform
    generatedAt   = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
    components    = [ordered]@{
      ytDlp  = [ordered]@{ version = $yt.version; archiveSha256 = $yt.sha256; gpgFingerprint = $yt.publicKeyFingerprint }
      deno   = [ordered]@{ version = $deno.version; archiveSha256 = $deno.sha256 }
      ffmpeg = [ordered]@{ version = $ff.version; releaseTag = $ff.releaseTag; archiveSha256 = $ff.sha256 }
    }
    fileCount     = $fileEntries.Count
    totalBytes    = $totalBytes
    files         = $fileEntries
  }
  $manifestPath = Join-Path $StageOut 'tools\manifest.json'
  [IO.File]::WriteAllText($manifestPath, ($manifest | ConvertTo-Json -Depth 6) + "`n", [Text.UTF8Encoding]::new($false))

  Write-Host "=== 7. Atomically replacing output directory: $OutputDir ==="
  $outParent = Split-Path -Parent $OutputDir
  New-Item -ItemType Directory -Force -Path $outParent | Out-Null
  $backupDir = "$OutputDir.old-" + [Guid]::NewGuid().ToString('N').Substring(0, 8)
  if (Test-Path -LiteralPath $OutputDir) {
    Move-Item -Force -LiteralPath $OutputDir -Destination $backupDir
  }
  try {
    Move-Item -Force -LiteralPath $StageOut -Destination $OutputDir
  } catch {
    if (Test-Path -LiteralPath $backupDir) {
      Move-Item -Force -LiteralPath $backupDir -Destination $OutputDir -ErrorAction SilentlyContinue
    }
    throw
  }
  if (Test-Path -LiteralPath $backupDir) {
    Remove-Item -Recurse -Force -LiteralPath $backupDir -ErrorAction SilentlyContinue
  }

  Write-Host ("DONE: Bundled runtime staged at {0} ({1} files, {2:N2} MiB)" -f $OutputDir, $fileEntries.Count, ($totalBytes / 1MB))
} finally {
  if (Test-Path -LiteralPath $Work) {
    Remove-Item -Recurse -Force -LiteralPath $Work -ErrorAction SilentlyContinue
  }
}
