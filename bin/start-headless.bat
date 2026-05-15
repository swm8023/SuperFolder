@echo off
setlocal
set "PORT=%~1"
if "%PORT%"=="" set "PORT=18080"
set "EXE=%~dp0superfolder.exe"

if not exist "%EXE%" (
  echo superfolder.exe not found. Run ..\script\build.bat first.
  pause
  exit /b 1
)

echo Starting SuperFolder headless on http://127.0.0.1:%PORT%/
"%EXE%" --headless --port "%PORT%"
if errorlevel 1 pause
exit /b %ERRORLEVEL%
