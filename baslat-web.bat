@echo off
setlocal
cd /d "%~dp0"

echo ===========================================
echo      ALBION PERSONAL WEB - NEXT.JS
echo ===========================================
echo.
echo Web arayuzu baslatiliyor: http://localhost:3000
echo.

if not exist "web\package.json" (
    echo HATA: web\package.json bulunamadi.
    pause
    exit /b 1
)

if not exist "web\node_modules" (
    echo [1/2] Web bagimliliklari kuruluyor...
    pushd web
    call npm install
    if errorlevel 1 (
        echo.
        echo HATA: npm install basarisiz oldu.
        popd
        pause
        exit /b 1
    )
    popd
    echo.
)

echo [2/2] Web gelistirme sunucusu baslatiliyor...
echo Tarayicida ac: http://localhost:3000
echo.

pushd web
call npm run dev
set EXIT_CODE=%ERRORLEVEL%
popd

if not "%EXIT_CODE%"=="0" (
    echo.
    echo Web sunucusu hata ile kapandi. Kod: %EXIT_CODE%
    pause
    exit /b %EXIT_CODE%
)
