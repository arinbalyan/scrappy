$ErrorActionPreference = "Stop"
$base = "https://github.com/arinbalyan/scrappy/releases/latest/download"
$archive = "scrappy_windows_amd64.zip"
$tmp = Join-Path $env:TEMP "scrappy-install-$([guid]::NewGuid())"
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
  Invoke-WebRequest -Uri "$base/$archive" -OutFile (Join-Path $tmp $archive)
  Invoke-WebRequest -Uri "$base/SHA256SUMS" -OutFile (Join-Path $tmp "SHA256SUMS")
  $expected = (Select-String -Path (Join-Path $tmp "SHA256SUMS") -Pattern " $archive$").Line.Split(' ')[0]
  $actual = (Get-FileHash -Algorithm SHA256 -Path (Join-Path $tmp $archive)).Hash.ToLowerInvariant()
  if ($actual -ne $expected) { throw "checksum mismatch for $archive" }
  Expand-Archive -Path (Join-Path $tmp $archive) -DestinationPath $tmp
  $target = Join-Path $env:LOCALAPPDATA "scrappy"
  New-Item -ItemType Directory -Force -Path $target | Out-Null
  Copy-Item -Path (Join-Path $tmp "scrappy.exe") -Destination (Join-Path $target "scrappy.exe") -Force
} finally {
  Remove-Item -Recurse -Force $tmp
}

