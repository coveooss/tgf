$ErrorActionPreference = "Stop" #Make all errors terminating

try {
    $bVersion = (Invoke-WebRequest -Uri "https://coveo-bootstrap-us-east-1.s3.amazonaws.com/tgf_version.txt").Content
    $LATEST_VERSION = [System.Text.Encoding]::ASCII.GetString($bVersion)
    Write-Host "- tgf version (latest):" $LATEST_VERSION
} catch {
    Write-Host Error fetching latest version
    Exit 1
}

$ZipFile = "tgf.zip"
$TempTgfFolder = "tgf_unzipped"
$TempTgfPath = Join-Path -Path $TempTgfFolder -ChildPath "tgf.exe"

Write-Host "Installing tgf v$($LATEST_VERSION) in the current directory ($(Get-Location)) ..."
Invoke-WebRequest "https://github.com/coveo/tgf/releases/download/v$($LATEST_VERSION)/tgf_$($LATEST_VERSION)_windows_64-bits.zip" -OutFile $ZipFile
Expand-Archive -Path $ZipFile -DestinationPath $TempTgfFolder
Copy-Item $TempTgfPath -Destination $TGF_PATH -Force

Remove-Item $ZipFile
Remove-Item $TempTgfFolder -Recurse
Write-Host "Installation is completed!"
Write-Host "Make sure to add tgf to your path."