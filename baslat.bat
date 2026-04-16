@echo off
setlocal
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
echo.
choice /c 12 /n /m "Seciminiz [1/2]: "

set "RUN_ARGS=-d"
set "RUN_MODE_DESC=Sadece benim DB"

if errorlevel 2 (
    set "RUN_ARGS=-i https+pow://albion-online-data.com"
    set "RUN_MODE_DESC=Benim DB + AODP"
)

echo.
echo [3/3] Program baslatiliyor: %RUN_MODE_DESC%
go run -mod=vendor . %RUN_ARGS%

echo.
pause
