$ErrorActionPreference = "Stop" #Make all errors terminating

# This script uses Powershell 7 features. Make sure we are running in Powershell 7.
if ($PSVersionTable.PSVersion.Major -lt 7) {
    Write-Host "The tgf install script requires PowerShell 7 or higher. Please install PowerShell 7 and try again."
    Exit 1
}

try {
    $latestReleaseRequest = @{
        Method = "HEAD"
        Uri = "https://github.com/coveooss/tgf/releases/latest"
        # Prevent redirect. We want the Location header.
        MaximumRedirection = 0
        # It considers the redirect http codes errors. Ignore that.
        SkipHttpErrorCheck = $true
        # The missed redirect generates an actual error which stops the program. We ignore it.
        ErrorAction = "SilentlyContinue"
    }

    $latestReleaseUrl = (Invoke-WebRequest @latestReleaseRequest).Headers["Location"]
    $LATEST_VERSION = $latestReleaseUrl.Split("/")[-1].TrimStart("v")
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
