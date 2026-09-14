param(
    [string]$Version = "",
    [string]$InstallDir = ""
)

$ErrorActionPreference = "Stop"
$repo = if ($env:BREAD_RELEASE_REPOSITORY) { $env:BREAD_RELEASE_REPOSITORY } else { "HuakunShen/bread" }
if (-not $InstallDir) {
    $InstallDir = if ($env:BREAD_INSTALL_DIR) { $env:BREAD_INSTALL_DIR } else {
        Join-Path $env:LOCALAPPDATA "bread\bin"
    }
}

$architecture = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
$arch = switch ($architecture.ToUpperInvariant()) {
    "AMD64" { "amd64"; break }
    "ARM64" { "arm64"; break }
    default { throw "bread installer: unsupported architecture $architecture" }
}

if (-not $Version) {
    $release = Invoke-RestMethod `
        -Headers @{ Accept = "application/vnd.github+json" } `
        -Uri "https://api.github.com/repos/$repo/releases/latest"
    $Version = $release.tag_name
}
if (-not $Version) {
    throw "bread installer: could not determine the latest release"
}

$tag = $Version
$versionNumber = $tag.TrimStart("v")
$asset = "bread_${versionNumber}_windows_${arch}.zip"
$baseUrl = "https://github.com/$repo/releases/download/$tag"
$temporaryDirectory = Join-Path ([IO.Path]::GetTempPath()) ("bread-install-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $temporaryDirectory | Out-Null
try {
    $archivePath = Join-Path $temporaryDirectory $asset
    $checksumPath = Join-Path $temporaryDirectory "checksums.txt"
    Invoke-WebRequest -UseBasicParsing -OutFile $archivePath -Uri "$baseUrl/$asset"
    Invoke-WebRequest -UseBasicParsing -OutFile $checksumPath -Uri "$baseUrl/checksums.txt"

    $pattern = '^\s*([0-9a-fA-F]{64})\s+\*?' + [regex]::Escape($asset) + '\s*$'
    $checksumLine = Get-Content $checksumPath |
        Where-Object { $_ -match $pattern } |
        Select-Object -First 1
    if (-not $checksumLine) {
        throw "bread installer: checksum entry not found for $asset"
    }
    $expected = $Matches[1].ToLowerInvariant()
    $actual = (Get-FileHash -Algorithm SHA256 -Path $archivePath).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
        throw "bread installer: checksum verification failed"
    }

    $extractDirectory = Join-Path $temporaryDirectory "extracted"
    Expand-Archive -Path $archivePath -DestinationPath $extractDirectory -Force
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Copy-Item -Force (Join-Path $extractDirectory "bread.exe") (Join-Path $InstallDir "bread.exe")
    Write-Host "Installed bread $versionNumber to $(Join-Path $InstallDir 'bread.exe')"
    if (($env:Path -split ";") -notcontains $InstallDir) {
        Write-Host "Add $InstallDir to PATH to run bread."
    }
}
finally {
    Remove-Item -Recurse -Force $temporaryDirectory -ErrorAction SilentlyContinue
}
