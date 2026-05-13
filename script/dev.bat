@echo off
setlocal
set "ROOT=%~dp0.."
for %%I in ("%ROOT%") do set "ROOT=%%~fI"

if not exist "%ROOT%\app\node_modules\" (
  echo app\node_modules not found. Run script\setup.bat first.
  exit /b 1
)

call "%~dp0codegen-methods.bat"
if errorlevel 1 exit /b %ERRORLEVEL%

node "%~dp0dev.mjs" %*
exit /b %ERRORLEVEL%
