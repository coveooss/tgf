$ErrorActionPreference = "Stop" #Make all errors terminating
$env:PATH =$env:PATH+";."

try {
    $TGF_PATH = (Get-Command tgf.exe).Path
    Write-Host tgf path: $TGF_PATH
    $LOCAL_VERSION = (tgf.exe --current-version) | Out-String
    $LOCAL_VERSION = [regex]::Matches($LOCAL_VERSION,'[0-9]+.[0-9]+.[0-9]+').Value
} catch {
    $TGF_PATH = Join-Path -Path (Get-Location) -ChildPath "tgf.exe"
    Write-Host tgf not found. using $TGF_PATH
} finally {
    Write-Host "- tgf version (local):" $LOCAL_VERSION
}

try {
    $bVersion = (Invoke-WebRequest -Uri "https://coveo-bootstrap-us-east-1.s3.amazonaws.com/tgf_version.txt").Content
    $LATEST_VERSION = [System.Text.Encoding]::ASCII.GetString($bVersion)
    Write-Host "- tgf version (latest):" $LATEST_VERSION
} catch {
    Write-Host Error fetching latest version
    Exit 1
}

if ($LOCAL_VERSION -ne $LATEST_VERSION){
    $ZipFile = Join-Path -Path (Get-Location) -ChildPath  "tgf.zip"
    $TempTgfFolder = Join-Path -Path (Get-Location) -ChildPath  "tgf_unzipped"
    $TempTgfPath = Join-Path -Path $TempTgfFolder -ChildPath "tgf.exe"

    Write-Host Installing latest tgf version for Windows in $TGF_PATH ...
    Invoke-WebRequest "https://github.com/coveo/tgf/releases/download/v$($LATEST_VERSION)/tgf_$($LATEST_VERSION)_windows_64-bits.zip" -OutFile $ZipFile
    Expand-Archive -Path $ZipFile -DestinationPath $TempTgfFolder
    Copy-Item $TempTgfPath -Destination $TGF_PATH -Force

    Remove-Item $ZipFile
    Remove-Item $TempTgfFolder -Recurse
} else {
    Write-Host Local version is up to date. 
}

Invoke-Expression -Command "$($TGF_PATH) --current-version"