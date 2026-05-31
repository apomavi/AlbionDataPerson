@echo off
setlocal EnableDelayedExpansion
cd /d "%~dp0"

title Albion Ticaret Istihbarat Motoru
color 0A

echo ===================================================
echo      ALBION TICARET MOTORU - OTOMATIK BASLATICI
echo ===================================================
echo.
echo [1/3] Yerel derleme ortami hazirlaniyor...
set "GOCACHE=%cd%\.gocache"
set "GOMODCACHE=%cd%\.gomodcache"

echo.
echo [2/3] Veri gonderim modu seciliyor...
echo.
echo 1 - Sadece benim DB'ye yaz
echo 2 - Benim DB'ye yaz + AODP havuzuna da gonder
echo 3 - Ayri backend'e gonder (collector-only)
echo 4 - Ayri backend'e gonder + AODP havuzuna da gonder
echo.
choice /c 1234 /n /m "Seciminiz [1/2/3/4]: "

set "MODE=1"
if errorlevel 4 set "MODE=4"
if "%MODE%"=="1" if errorlevel 3 set "MODE=3"
if "%MODE%"=="1" if errorlevel 2 set "MODE=2"

set "RUN_ARGS=-d"
set "RUN_MODE_DESC=Sadece benim DB"

if "%MODE%"=="3" (
    set "RUN_ARGS=-d -embedded-custom=false -collector-url http://localhost:8082/api/collector/events"
    set "RUN_MODE_DESC=Collector-only + Standalone Backend"
)
if "%MODE%"=="4" (
    set "RUN_ARGS=-i https+pow://albion-online-data.com -embedded-custom=false -collector-url http://localhost:8082/api/collector/events"
    set "RUN_MODE_DESC=Standalone Backend + AODP"
)
if "%MODE%"=="2" (
    set "RUN_ARGS=-i https+pow://albion-online-data.com"
    set "RUN_MODE_DESC=Benim DB + AODP"
)

echo.
echo [3/3] Program baslatiliyor: %RUN_MODE_DESC%
if "%MODE%"=="3" (
    echo Standalone backend'in ayri bir pencerede calisiyor olmasi gerekir.
    echo Gerekirse once baslat-backend.bat calistir.
)
if "%MODE%"=="4" (
    echo Standalone backend'in ayri bir pencerede calisiyor olmasi gerekir.
    echo Gerekirse once baslat-backend.bat calistir.
)

set "COLLECTOR_USER_TOKEN="
if "%MODE%"=="3" (
    set /p COLLECTOR_USER_TOKEN=Collector kullanici tokeni - bos birak = anonim: 
)
if "%MODE%"=="4" (
    set /p COLLECTOR_USER_TOKEN=Collector kullanici tokeni - bos birak = anonim: 
)
set "COLLECTOR_USER_TOKEN=!COLLECTOR_USER_TOKEN: =!"
if defined COLLECTOR_USER_TOKEN (
    set "RUN_ARGS=!RUN_ARGS! -collector-token=!COLLECTOR_USER_TOKEN!"
)

go run -mod=vendor . !RUN_ARGS!

echo.
pause
