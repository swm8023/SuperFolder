@echo off
setlocal
set "ROOT=%~dp0.."
for %%I in ("%ROOT%") do set "ROOT=%%~fI"
set "APP_DIR=%ROOT%\app"
set "SERVICE_DIR=%ROOT%\service"
set "EXE_PATH=%ROOT%\bin\superfolder.exe"

if not exist "%APP_DIR%\node_modules\" (
  echo app\node_modules not found. Run script\setup.bat first.
  exit /b 1
)

call "%~dp0codegen-methods.bat"
if errorlevel 1 exit /b %ERRORLEVEL%

pushd "%SERVICE_DIR%" || exit /b 1
go test ./...
if errorlevel 1 exit /b %ERRORLEVEL%
popd

pushd "%APP_DIR%" || exit /b 1
call npm run typecheck
if errorlevel 1 exit /b %ERRORLEVEL%
call npm test
if errorlevel 1 exit /b %ERRORLEVEL%
popd

call "%~dp0build.bat"
if errorlevel 1 exit /b %ERRORLEVEL%

node "%~dp0smoke-headless.mjs" "%EXE_PATH%" 18081
exit /b %ERRORLEVEL%
