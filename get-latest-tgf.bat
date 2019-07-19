@echo off
setlocal enableDelayedExpansion

if "%TGF_PATH%" == "" (
    where tgf > tmp.txt
    SET /p TGF_PATH=<tmp.txt

    if "!TGF_PATH!"=="" (
        echo path: !TGF_PATH!
        echo tgf path not found
        SET TGF_PATH="%cd%"
    ) else (
        SET TGF_PATH=!TGF_PATH:~0,-8!
        echo TGF_PATH: !TGF_PATH!
    )
) else (
    echo TGF_PATH is set to: !TGF_PATH!
)

curl --silent https://coveo-bootstrap-us-east-1.s3.amazonaws.com/tgf_version.txt > tmp.txt
SET /p  TGF_LATEST_VERSION=<tmp.txt

tgf --current-version > tmp.txt
SET /p TGF_LOCAL_VERSION=<tmp.txt
if "%TGF_LOCAL_VERSION%" NEQ "" (
    SET TGF_LOCAL_VERSION=%TGF_LOCAL_VERSION:~5%
)


echo "- tgf version (local): %TGF_LOCAL_VERSION%"
echo "- tgf version (latest): %TGF_LATEST_VERSION%"

if "%TGF_LOCAL_VERSION%"=="%TGF_LATEST_VERSION%" (
    echo Local version is up to date.
) else (
    echo Installing latest tgf version for Windows in %TGF_PATH% ...
    powershell Invoke-WebRequest https://github.com/coveo/tgf/releases/download/v1.21.0/tgf_1.21.0_windows_64-bits.zip -OutFile tgf.zip
    powershell Expand-Archive tgf.zip -Force -DestinationPath %TGF_PATH%
    del tgf.zip

    echo Done.
    %TGF_PATH%\tgf --current-version
)

REM Cleanup
del tmp.txt
SET "TGF_PATH="
SET "TGF_LATEST_VERSION="
SET "TGF_LOCAL_VERSION="