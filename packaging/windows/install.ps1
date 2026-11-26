<#
.SYNOPSIS
Installs or updates this fork's Tailscale build on Windows.

.DESCRIPTION
Run from an unpacked release archive, it installs the files next to it.
Otherwise it downloads the latest release, or the one named by -Version,
and checks it against the release's SHA256SUMS file.

The files go to "Program Files\Tailscale" and tailscaled runs as the
"Tailscale" service, like the official client, so the official client has
to be uninstalled first. The login and settings in ProgramData\Tailscale
are kept.

It asks for administrator rights when it needs them.

.EXAMPLE
irm https://raw.githubusercontent.com/NightFeather0615/tailscale/fork/packaging/windows/install.ps1 | iex

.EXAMPLE
.\install.ps1 -Version 1.106.1-nf.1
#>
[CmdletBinding()]
param(
  # The release to install, such as 1.106.1-nf.1. Defaults to the latest.
  [string]$Version,

  # A directory with the unpacked release files, instead of a download.
  [string]$Source,

  # Download and check the release, print where it is, and stop.
  [switch]$DownloadOnly,

  # Set by the script when it restarts itself as administrator.
  [switch]$Elevated
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$repo = 'NightFeather0615/tailscale'
$dest = Join-Path $env:ProgramFiles 'Tailscale'
$serviceName = 'Tailscale'
$files = @(
  'tailscaled.exe', 'tailscale.exe', 'tailscale-systray.exe', 'wintun.dll',
  'LICENSE.txt', 'wintun-LICENSE.txt'
)

function Test-Administrator {
  $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
  $principal = [Security.Principal.WindowsPrincipal]::new($identity)
  $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# Downloads a release, checks its hash, unpacks it, and returns the directory.
function Get-Release([string]$Version) {
  if ($env:PROCESSOR_ARCHITECTURE -ne 'AMD64') {
    throw "No build for $env:PROCESSOR_ARCHITECTURE; releases cover 64-bit x86 Windows."
  }
  if (-not $Version) {
    $latest = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest"
    $Version = $latest.tag_name -replace '^v', ''
  }
  if ($Version -notmatch '^[0-9]+\.[0-9]+\.[0-9]+-nf\.[0-9]+$') {
    throw "'$Version' is not a release name such as 1.106.1-nf.1."
  }

  $name = "tailscale-nf_${Version}_windows_amd64"
  $url = "https://github.com/$repo/releases/download/v$Version"
  $work = Join-Path ([IO.Path]::GetTempPath()) "tailscale-nf-$Version"
  if (Test-Path $work) { Remove-Item $work -Recurse -Force }
  New-Item $work -ItemType Directory | Out-Null

  Write-Host "Downloading $name.zip"
  $zip = Join-Path $work "$name.zip"
  Invoke-WebRequest "$url/$name.zip" -OutFile $zip -UseBasicParsing
  $sums = (Invoke-WebRequest "$url/SHA256SUMS" -UseBasicParsing).Content
  if ($sums -is [byte[]]) { $sums = [Text.Encoding]::UTF8.GetString($sums) }
  $line = $sums -split "`n" | Where-Object { $_.TrimEnd() -match " \*?$([regex]::Escape("$name.zip"))$" }
  if (-not $line) { throw "SHA256SUMS has no entry for $name.zip." }
  $want = ($line -split '\s+')[0]
  $got = (Get-FileHash $zip -Algorithm SHA256).Hash
  if ($got -ne $want) { throw "$name.zip does not match SHA256SUMS." }

  Expand-Archive $zip -DestinationPath $work
  Join-Path $work $name
}

# Copies the files into place and makes sure the service runs. Needs
# administrator rights.
function Install-Release([string]$Source) {
  foreach ($file in $files) {
    if (-not (Test-Path (Join-Path $Source $file))) { throw "$Source has no $file." }
  }
  if (Test-Path (Join-Path $dest 'tailscale-ipn.exe')) {
    throw ('The official Tailscale client is installed. Uninstall it in ' +
      'Settings > Apps first; your login is kept.')
  }

  Get-Process 'tailscale-systray' -ErrorAction SilentlyContinue | Stop-Process -Force
  $service = Get-Service $serviceName -ErrorAction SilentlyContinue
  if ($service -and $service.Status -ne 'Stopped') {
    Write-Host "Stopping the $serviceName service"
    Stop-Service $serviceName -Force
    $service.WaitForStatus('Stopped', [TimeSpan]::FromSeconds(30))
  }
  # The service process can outlive the stop request for a moment.
  Get-Process 'tailscaled' -ErrorAction SilentlyContinue | Wait-Process -Timeout 15 -ErrorAction SilentlyContinue

  New-Item $dest -ItemType Directory -Force | Out-Null
  foreach ($file in $files) {
    Copy-Item (Join-Path $Source $file) (Join-Path $dest $file) -Force
  }

  if (-not $service) {
    Write-Host "Creating the $serviceName service"
    & (Join-Path $dest 'tailscaled.exe') install-system-daemon
    if ($LASTEXITCODE -ne 0) { throw "tailscaled install-system-daemon failed ($LASTEXITCODE)." }
  }
  Start-Service $serviceName

  $path = [Environment]::GetEnvironmentVariable('Path', 'Machine')
  if (($path -split ';') -notcontains $dest) {
    [Environment]::SetEnvironmentVariable('Path', "$($path.TrimEnd(';'));$dest", 'Machine')
  }

  $programs = [Environment]::GetFolderPath('CommonPrograms')
  $shortcut = (New-Object -ComObject WScript.Shell).CreateShortcut((Join-Path $programs 'Tailscale.lnk'))
  $shortcut.TargetPath = Join-Path $dest 'tailscale-systray.exe'
  $shortcut.WorkingDirectory = $dest
  $shortcut.Description = 'Tailscale (fork build)'
  $shortcut.Save()
}

function Start-Systray {
  $systray = Join-Path $dest 'tailscale-systray.exe'
  if (Test-Administrator) {
    # Started from here it would run as administrator. Explorer starts it
    # as the signed-in user.
    Start-Process explorer.exe $systray
  } else {
    Start-Process $systray
  }
}

if (-not $Source -and $PSScriptRoot -and (Test-Path (Join-Path $PSScriptRoot 'tailscaled.exe'))) {
  $Source = $PSScriptRoot
}
if (-not $Source) {
  $Source = Get-Release $Version
}
$Source = (Resolve-Path $Source).Path
if ($DownloadOnly) {
  Write-Host "The release is in $Source"
  return
}

if ($Elevated) {
  # This window closes when the script ends, so hold an error on screen.
  try {
    Install-Release $Source
  } catch {
    Write-Host "Error: $_" -ForegroundColor Red
    Read-Host 'Press Enter to close'
    exit 1
  }
  return
}

if (Test-Administrator) {
  Install-Release $Source
} else {
  # Run the copy of this script that came with the release as administrator.
  $shell = (Get-Process -Id $PID).Path
  $arguments = @(
    '-NoProfile', '-ExecutionPolicy', 'Bypass',
    '-File', "`"$(Join-Path $Source 'install.ps1')`"",
    '-Source', "`"$Source`"", '-Elevated'
  )
  $process = Start-Process $shell -Verb RunAs -ArgumentList $arguments -Wait -PassThru
  if ($process.ExitCode -ne 0) { throw "The installation failed ($($process.ExitCode))." }
}

Start-Systray
$installed = (& (Join-Path $dest 'tailscale.exe') version | Select-Object -First 1)
Write-Host "Installed Tailscale $installed (fork build) in $dest."
Write-Host 'To join a tailnet: tailscale up --login-server https://<your control server>'
