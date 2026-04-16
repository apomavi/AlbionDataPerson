@echo off
setlocal
cd /d "%~dp0"

title Albion Ticaret Istihbarat Motoru
color 0A

echo ===================================================
echo      ALBION TICARET MOTORU - UPSTREAM GUNCELLEME
echo ===================================================
echo.
echo Bu arac orijinal GitHub reposundan guncel kodlari ceker.
echo Kendi degisikliklerin varsa once commit etmen veya yedek alman onerilir.
echo.

git remote get-url upstream >nul 2>nul
if errorlevel 1 (
    echo [HATA] 'upstream' remote tanimli degil.
    echo Sunu bir kez calistir:
    echo git remote add upstream https://github.com/ao-data/albiondata-client.git
    goto :end
)

echo [1/4] Calisma klasoru kontrol ediliyor...
git status --short
echo.

echo [2/4] Upstream repo cekiliyor...
git fetch upstream --tags
if errorlevel 1 (
    echo [HATA] Upstream fetch basarisiz oldu.
    goto :end
)

echo.
echo [3/4] Upstream/master bu branch ile birlestiriliyor...
git merge upstream/master
if errorlevel 1 (
    echo.
    echo [DIKKAT] Merge sirasinda conflict ciktiysa once onlari coz.
    goto :end
)

echo.
echo [4/4] Derleme kontrolu yapiliyor...
set "GOCACHE=%cd%\.gocache"
set "GOMODCACHE=%cd%\.gomodcache"
go build -mod=vendor ./...
if errorlevel 1 (
    echo [HATA] Derleme kontrolu basarisiz oldu.
    goto :end
)

echo.
echo [BASARILI] Upstream guncellemesi ve derleme kontrolu tamamlandi.
echo ===================================================
echo.

:end
pause
