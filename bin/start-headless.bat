@echo off
setlocal
set "PORT=%~1"
if "%PORT%"=="" set "PORT=18080"
set "EXE=%~dp0app-host-demo.exe"

if not exist "%EXE%" (
  echo app-host-demo.exe not found. Run ..\script\build.bat first.
  pause
  exit /b 1
)

echo Starting APP Host Demo headless on http://127.0.0.1:%PORT%/
"%EXE%" --headless --port "%PORT%"
if errorlevel 1 pause
exit /b %ERRORLEVEL%
