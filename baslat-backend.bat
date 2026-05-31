@echo off
setlocal
cd /d "%~dp0"

echo ===========================================
echo    ALBION PERSONAL BACKEND - STANDALONE
echo ===========================================
echo.
echo Baslatiliyor: http://localhost:8082
echo.

set GOCACHE=%CD%\.gocache
set GOMODCACHE=%CD%\.gomodcache

go run ./cmd/albion-personal-backend --addr :8082
