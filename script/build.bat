@echo off
setlocal EnableExtensions
set "ROOT=%~dp0.."
for %%I in ("%ROOT%") do set "ROOT=%%~fI"
set "APP_DIR=%ROOT%\app"
set "SERVICE_DIR=%ROOT%\service"
set "BUILD_DIR=%ROOT%\.build"
set "EMBED_DIR=%BUILD_DIR%\embedweb"
set "EMBED_APP_DIR=%EMBED_DIR%\app"
set "SERVICE_BUILD_DIR=%BUILD_DIR%\service"
set "BIN_DIR=%ROOT%\bin"
set "EXE_PATH=%BIN_DIR%\superfolder.exe"

if not exist "%APP_DIR%\node_modules\" (
  echo app\node_modules not found. Run script\setup.bat first.
  exit /b 1
)

call "%~dp0codegen-methods.bat"
if errorlevel 1 exit /b %ERRORLEVEL%

if exist "%EMBED_DIR%" rmdir /s /q "%EMBED_DIR%"
if errorlevel 1 exit /b %ERRORLEVEL%
if exist "%SERVICE_BUILD_DIR%" rmdir /s /q "%SERVICE_BUILD_DIR%"
if errorlevel 1 exit /b %ERRORLEVEL%
if exist "%EXE_PATH%" del /q "%EXE_PATH%"
if errorlevel 1 exit /b %ERRORLEVEL%

if not exist "%BUILD_DIR%" mkdir "%BUILD_DIR%"
if not exist "%BIN_DIR%" mkdir "%BIN_DIR%"

pushd "%APP_DIR%" || exit /b 1
call npm run build
if errorlevel 1 exit /b %ERRORLEVEL%
popd

if not exist "%EMBED_APP_DIR%\" (
  echo frontend build output not found: %EMBED_APP_DIR%
  exit /b 1
)

> "%EMBED_DIR%\go.mod" (
  echo module apphostdemo/embedweb
  echo.
  echo go 1.26
)

> "%EMBED_DIR%\embedweb.go" (
  echo package embedweb
  echo.
  echo import "embed"
  echo.
  echo //go:embed all:app
  echo var FS embed.FS
)

xcopy "%SERVICE_DIR%" "%SERVICE_BUILD_DIR%\" /E /I /Y /Q >nul
if errorlevel 2 exit /b %ERRORLEVEL%

> "%SERVICE_BUILD_DIR%\backend\embedweb_release.go" (
  echo package backend
  echo.
  echo import ^(
  echo 	"io/fs"
  echo.
  echo 	embedweb "apphostdemo/embedweb"
  echo ^)
  echo.
  echo func init^(^) {
  echo 	sub, err := fs.Sub^(embedweb.FS, "app"^)
  echo 	if err != nil {
  echo 		panic^(err^)
  echo 	}
  echo 	EmbeddedWebFS = sub
  echo }
)

pushd "%SERVICE_BUILD_DIR%" || exit /b 1
go mod edit "-require=apphostdemo/embedweb@v0.0.0"
if errorlevel 1 exit /b %ERRORLEVEL%
go mod edit "-replace=apphostdemo/embedweb=../embedweb"
if errorlevel 1 exit /b %ERRORLEVEL%
go mod tidy
if errorlevel 1 exit /b %ERRORLEVEL%
go build -ldflags="-H windowsgui" -o "..\..\bin\superfolder.exe" .
if errorlevel 1 exit /b %ERRORLEVEL%
popd

if not exist "%EXE_PATH%" (
  echo build did not produce %EXE_PATH%
  exit /b 1
)

echo Built %EXE_PATH%
exit /b 0
