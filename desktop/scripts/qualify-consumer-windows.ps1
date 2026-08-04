$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

if ($env:GITHUB_ACTIONS -ne "true" -or $args.Count -ne 1) {
  throw "Run only in GitHub Actions with one Squirrel Setup executable."
}

$artifact = (Resolve-Path $args[0]).Path
$applicationRoot = Join-Path $env:LocalAppData "leapview"
$update = Join-Path $applicationRoot "Update.exe"
$protocolKey = "Registry::HKEY_CURRENT_USER\Software\Classes\leapview-desktop"

$install = Start-Process $artifact -ArgumentList "--silent" -Wait -PassThru
if ($install.ExitCode -ne 0) {
  throw "Squirrel install failed with exit code $($install.ExitCode)."
}
$applicationDirectories = @(
  Get-ChildItem -Path $applicationRoot -Directory -Filter "app-*"
)
if ($applicationDirectories.Count -ne 1) {
  throw "Squirrel did not install exactly one application version."
}
$application = Join-Path $applicationDirectories[0].FullName "LeapView.exe"
if (-not (Test-Path $application) -or -not (Test-Path $update)) {
  Get-ChildItem -Path $applicationRoot -Recurse -ErrorAction SilentlyContinue |
    ForEach-Object { Write-Warning $_.FullName }
  throw "Squirrel did not install LeapView in the current user profile."
}
if ($application.StartsWith($env:ProgramFiles, [System.StringComparison]::OrdinalIgnoreCase)) {
  throw "Consumer installation unexpectedly used Program Files."
}

$helpers = @(
  Get-ChildItem `
    -Path (Join-Path $applicationRoot "app-*\resources\leapview-windows-policy.exe") `
    -File
)
if ($helpers.Count -ne 1) {
  throw "Consumer install did not contain exactly one policy probe."
}
$probe = (& $helpers[0].FullName) | ConvertFrom-Json
if ($probe.schemaVersion -ne 1 -or $probe.security -ne "missing") {
  throw "An absent enterprise policy did not resolve to consumer open mode."
}

$deadline = (Get-Date).AddSeconds(15)
while (-not (Test-Path $protocolKey) -and (Get-Date) -lt $deadline) {
  Start-Sleep -Milliseconds 250
}
if (-not (Test-Path $protocolKey)) {
  throw "Squirrel lifecycle did not register the per-user protocol."
}
$protocolCommand = (Get-Item $protocolKey).OpenSubKey("shell\open\command").GetValue("")
if (
  -not $protocolCommand.Contains("LeapView.exe") -or
  -not $protocolCommand.Contains('"%1"')
) {
  throw "Per-user protocol command has unsafe or stale quoting."
}

$uninstall = Start-Process $update `
  -ArgumentList "--uninstall", "-s" `
  -Wait `
  -PassThru
if ($uninstall.ExitCode -ne 0) {
  throw "Squirrel uninstall failed with exit code $($uninstall.ExitCode)."
}
$deadline = (Get-Date).AddSeconds(30)
while (
  ((Test-Path $protocolKey) -or (Test-Path $application)) -and
  (Get-Date) -lt $deadline
) {
  Start-Sleep -Milliseconds 250
}
if ((Test-Path $protocolKey) -or (Test-Path $application)) {
  throw "Squirrel uninstall left a stale per-user protocol handler."
}
