$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

if ($env:GITHUB_ACTIONS -ne "true" -or $args.Count -ne 1) {
  throw "Run only in GitHub Actions with one MSI package."
}

$artifact = (Resolve-Path $args[0]).Path
$applicationRoot = Join-Path $env:ProgramFiles "LeapView"
$application = Join-Path $applicationRoot "LeapView.exe"
$helper = Join-Path $applicationRoot "resources\leapview-windows-policy.exe"
$protocolKey = "Registry::HKEY_LOCAL_MACHINE\Software\Classes\leapview-desktop"

function Invoke-MSI([string[]] $Arguments) {
  $process = Start-Process msiexec.exe -ArgumentList $Arguments -Wait -PassThru
  if ($process.ExitCode -notin @(0, 1641, 3010)) {
    throw "msiexec failed with exit code $($process.ExitCode)"
  }
}

function Read-PolicyProbe {
  $output = & $helper
  if ($LASTEXITCODE -ne 0) {
    throw "Native policy helper failed with exit code $LASTEXITCODE."
  }
  return $output | ConvertFrom-Json
}

Invoke-MSI @("/i", "`"$artifact`"", "/qn", "/norestart")
if (-not (Test-Path $application)) {
  Get-ChildItem -Path $env:ProgramFiles, ${env:ProgramFiles(x86)}, $env:LocalAppData `
    -Filter LeapView.exe -Recurse -ErrorAction SilentlyContinue |
    ForEach-Object { Write-Warning $_.FullName }
  throw "MSI did not install LeapView.exe in Program Files."
}
if (-not (Test-Path $helper)) {
  Get-ChildItem -Path $applicationRoot -Recurse |
    ForEach-Object { Write-Warning $_.FullName }
  throw "MSI did not install the native policy helper."
}

$probe = Read-PolicyProbe
if ($probe.schemaVersion -ne 1 -or $probe.security -ne "missing") {
  throw "Native policy helper did not verify the installer-owned directory."
}
$policyPath = $probe.policyPath
Set-Content -LiteralPath $policyPath `
  -Value '{"qualification":"preserve-on-repair"}' `
  -Encoding utf8NoBOM
& icacls.exe $policyPath /setowner "*S-1-5-32-544" | Out-Null
if ($LASTEXITCODE -ne 0) {
  throw "Failed to set the managed policy owner."
}
& icacls.exe $policyPath /inheritance:r `
  /grant:r "*S-1-5-18:(F)" "*S-1-5-32-544:(F)" "*S-1-5-32-545:(R)" | Out-Null
if ($LASTEXITCODE -ne 0) {
  throw "Failed to set the managed policy ACL."
}
$probe = Read-PolicyProbe
if ($probe.security -ne "secure") {
  throw "Native policy helper did not accept the administrator-only policy."
}

$protocolCommand = (Get-Item $protocolKey).OpenSubKey(
  "shell\open\command"
).GetValue("")
$expectedCommand = "`"$application`" `"%1`""
if ($protocolCommand -ne $expectedCommand) {
  throw "MSI protocol command has unsafe or stale quoting."
}

Invoke-MSI @("/fa", "`"$artifact`"", "/qn", "/norestart")
if ((Get-Content -Raw -LiteralPath $policyPath).Trim() -ne `
    '{"qualification":"preserve-on-repair"}') {
  throw "MSI repair did not preserve managed policy."
}

Invoke-MSI @("/x", "`"$artifact`"", "/qn", "/norestart")
if (Test-Path $protocolKey) {
  throw "MSI uninstall left a stale protocol handler."
}
if (-not (Test-Path $policyPath)) {
  throw "MSI uninstall removed retained managed policy."
}
