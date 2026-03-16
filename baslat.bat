@echo off
title Albion Ticaret Istihbarat Motoru
color 0A

echo ===================================================
echo      ALBION TICARET MOTORU - OTOMATIK BASLATICI
echo ===================================================
echo.
echo [1/3] Dis kaynaklar (GitHub vb.) kontrol ediliyor ve guncelleniyor...
cd C:\Users\MAVI\Desktop\AlbionDataPerson
go get -u all

echo.
echo [2/3] Gereksiz baglantilar temizleniyor (Mod Tidy)...
go mod tidy

echo.
echo [3/3] Kodlar yerel 'vendor' klasorune kopyalaniyor...
go mod vendor

echo.
echo [BASARILI] Tum sistem guncel ve yerellestirildi!
echo Motor baslatiliyor, lutfen bekleyin...
echo ===================================================
echo.

go run .

:: Eger program bir sekilde cokerse veya kapanirsa ekranin hemen kapanmamasi icin:
pause