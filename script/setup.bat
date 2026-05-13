@echo off
setlocal
set "ROOT=%~dp0.."
for %%I in ("%ROOT%") do set "ROOT=%%~fI"

go version || exit /b %ERRORLEVEL%
node --version || exit /b %ERRORLEVEL%
call npm --version
if errorlevel 1 exit /b %ERRORLEVEL%

pushd "%ROOT%\app" || exit /b 1
call npm install
if errorlevel 1 exit /b %ERRORLEVEL%
popd

pushd "%ROOT%\service" || exit /b 1
go mod download
if errorlevel 1 exit /b %ERRORLEVEL%
popd

call "%~dp0codegen-methods.bat"
exit /b %ERRORLEVEL%
