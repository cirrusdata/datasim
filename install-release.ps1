#requires -Version 5.1

[CmdletBinding()]
param(
    [string]$Repository = "cirrusdata/datasim",
    [string]$Version = "latest",
    [string]$InstallDir = [System.IO.Path]::Combine($env:LOCALAPPDATA, "Programs", "datasim")
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

function Write-Step {
    param([string]$Message)

    Write-Host ""
    Write-Host $Message
}

function Get-PlatformArch {
    $architecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()

    switch ($architecture) {
        "x64" { return "amd64" }
        "arm64" { return "arm64" }
        default { throw "unsupported Windows architecture: $architecture" }
    }
}

function Get-ReleaseMetadata {
    param(
        [string]$Repo,
        [string]$Tag
    )

    $headers = @{
        "Accept" = "application/vnd.github+json"
        "User-Agent" = "datasim-installer"
    }

    if ($Tag -eq "latest") {
        $uri = "https://api.github.com/repos/$Repo/releases/latest"
    } else {
        $escapedTag = [Uri]::EscapeDataString($Tag)
        $uri = "https://api.github.com/repos/$Repo/releases/tags/$escapedTag"
    }

    return Invoke-RestMethod -Headers $headers -Uri $uri
}

function Get-ArchiveAsset {
    param(
        [object]$Release,
        [string]$Arch
    )

    $pattern = "^datasim_.*_windows_${Arch}\.zip$"
    $asset = $Release.assets | Where-Object { $_.name -match $pattern } | Select-Object -First 1
    if ($null -eq $asset) {
        throw "no release archive found for windows/$Arch"
    }

    return $asset
}

function Get-ReleaseAssetByName {
    param(
        [object]$Release,
        [string]$Name
    )

    $asset = $Release.assets | Where-Object { $_.name -eq $Name } | Select-Object -First 1
    if ($null -eq $asset) {
        throw "release asset not found: $Name"
    }

    return $asset
}

function Download-File {
    param(
        [string]$Uri,
        [string]$Destination
    )

    Invoke-WebRequest -UseBasicParsing -Uri $Uri -OutFile $Destination
}

function Get-ExpectedChecksum {
    param(
        [string]$ChecksumFile,
        [string]$AssetName
    )

    foreach ($line in Get-Content -LiteralPath $ChecksumFile) {
        if ($line -match "^([A-Fa-f0-9]{64})\s+\*?(.+)$" -and $Matches[2] -eq $AssetName) {
            return $Matches[1].ToLowerInvariant()
        }
    }

    throw "checksum for $AssetName not found in checksums.txt"
}

function Add-InstallDirToPath {
    param([string]$PathToAdd)

    $normalized = $PathToAdd.TrimEnd("\")
    $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    $userEntries = @()

    if (-not [string]::IsNullOrWhiteSpace($userPath)) {
        $userEntries = $userPath -split ";" | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
    }

    foreach ($entry in $userEntries) {
        if ($entry.TrimEnd("\") -ieq $normalized) {
            return $false
        }
    }

    $newUserPath = if ([string]::IsNullOrWhiteSpace($userPath)) {
        $PathToAdd
    } else {
        "{0};{1}" -f $userPath.TrimEnd(";"), $PathToAdd
    }

    [Environment]::SetEnvironmentVariable("PATH", $newUserPath, "User")

    $processEntries = @()
    if (-not [string]::IsNullOrWhiteSpace($env:PATH)) {
        $processEntries = $env:PATH -split ";" | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
    }

    $alreadyInProcessPath = $false
    foreach ($entry in $processEntries) {
        if ($entry.TrimEnd("\") -ieq $normalized) {
            $alreadyInProcessPath = $true
            break
        }
    }

    if (-not $alreadyInProcessPath) {
        $env:PATH = if ([string]::IsNullOrWhiteSpace($env:PATH)) {
            $PathToAdd
        } else {
            "{0};{1}" -f $PathToAdd, $env:PATH
        }
    }

    return $true
}

$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("datasim-install-" + [Guid]::NewGuid().ToString("N"))

try {
    Write-Host "datasim Release Installer for Windows"
    Write-Host "====================================="

    $arch = Get-PlatformArch
    $release = Get-ReleaseMetadata -Repo $Repository -Tag $Version
    $tag = $release.tag_name
    if ([string]::IsNullOrWhiteSpace($tag)) {
        throw "release metadata did not include a tag name"
    }

    $archiveAsset = Get-ArchiveAsset -Release $release -Arch $arch
    $checksumAsset = Get-ReleaseAssetByName -Release $release -Name "checksums.txt"

    Write-Host ""
    Write-Host ("Detected platform: Windows ({0})" -f $arch)
    Write-Host ("Installing release: {0}" -f $tag)

    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

    $archivePath = Join-Path $tempDir $archiveAsset.name
    $checksumPath = Join-Path $tempDir $checksumAsset.name
    $extractDir = Join-Path $tempDir "extract"

    Write-Step ("Downloading {0}..." -f $archiveAsset.name)
    Download-File -Uri $archiveAsset.browser_download_url -Destination $archivePath

    Write-Step "Downloading checksums.txt..."
    Download-File -Uri $checksumAsset.browser_download_url -Destination $checksumPath

    Write-Step "Verifying checksum..."
    $expectedChecksum = Get-ExpectedChecksum -ChecksumFile $checksumPath -AssetName $archiveAsset.name
    $actualChecksum = (Get-FileHash -Algorithm SHA256 -LiteralPath $archivePath).Hash.ToLowerInvariant()
    if ($actualChecksum -ne $expectedChecksum) {
        throw ("checksum mismatch for {0}: expected {1}, got {2}" -f $archiveAsset.name, $expectedChecksum, $actualChecksum)
    }

    Write-Step ("Installing to: {0}" -f $InstallDir)
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    New-Item -ItemType Directory -Path $extractDir -Force | Out-Null
    Expand-Archive -LiteralPath $archivePath -DestinationPath $extractDir -Force

    $binary = Get-ChildItem -LiteralPath $extractDir -Filter "datasim.exe" -File -Recurse | Select-Object -First 1
    if ($null -eq $binary) {
        throw "datasim.exe was not found in the release archive"
    }

    $destination = Join-Path $InstallDir "datasim.exe"
    Copy-Item -LiteralPath $binary.FullName -Destination $destination -Force

    if (-not (Test-Path -LiteralPath $destination -PathType Leaf)) {
        throw "installation failed: datasim.exe was not written to the install directory"
    }

    $pathAdded = Add-InstallDirToPath -PathToAdd $InstallDir

    Write-Host ""
    Write-Host "Installation complete."
    Write-Host ("Installed to: {0}" -f $destination)
    if ($pathAdded) {
        Write-Host ("Added to user PATH: {0}" -f $InstallDir)
        Write-Host "Open a new terminal to guarantee the PATH change is visible everywhere."
    } else {
        Write-Host ("Install directory is already on PATH: {0}" -f $InstallDir)
    }

    Write-Host ""
    Write-Host "Try it now:"
    Write-Host "  datasim version"
    Write-Host "  datasim --help"
} catch {
    Write-Error $_
    exit 1
} finally {
    if (Test-Path -LiteralPath $tempDir) {
        Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
