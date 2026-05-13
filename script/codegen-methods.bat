@echo off
setlocal
node "%~dp0codegen-methods.mjs" %*
exit /b %ERRORLEVEL%
