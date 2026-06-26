# scrappy installer for Windows — auto-detects arch, downloads latest release
$ErrorActionPreference = "Stop"

$Repo = "arinbalyan/scrappy"
$Latest = (Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest").tag_name

if (-not $Latest) {
    Write-Error "Could not determine latest release"
    exit 1
}

$Arch = if ([System.Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
$File = "scrappy_windows_$Arch.zip"
$Url = "https://github.com/$Repo/releases/download/$Latest/$File"
$Tmp = New-Item -ItemType Directory -Path "$env:TEMP\scrappy-install" -Force

Write-Host "Downloading scrappy $Latest for Windows/$Arch..."
Invoke-WebRequest -Uri $Url -OutFile "$Tmp\$File"
Expand-Archive -Path "$Tmp\$File" -DestinationPath "$Tmp" -Force

$InstallDir = "$env:LOCALAPPDATA\Programs\scrappy"
New-Item -ItemType Directory -Path $InstallDir -Force
Move-Item -Path "$Tmp\scrappy_windows_$Arch.exe" -Destination "$InstallDir\scrappy.exe" -Force
Remove-Item -Recurse -Force $Tmp

# Add to PATH if not already there
$Path = [System.Environment]::GetEnvironmentVariable("Path", "User")
if ($Path -notlike "*$InstallDir*") {
    [System.Environment]::SetEnvironmentVariable("Path", "$Path;$InstallDir", "User")
    Write-Host "Added $InstallDir to PATH (restart terminal to take effect)"
}

Write-Host "scrappy $Latest installed to $InstallDir\scrappy.exe"
Write-Host "  Run 'scrappy --help' to get started."