# AI Handoff

## Proje Amaci

AlbionPersonal, Albion Online market verisini ve collector olaylarini local
backend uzerinden izlemek icin kullanilan kisisel arac setidir.

## Mevcut Durum

- Web arayuzu Next.js altinda `web/` klasorundedir.
- Backend endpointleri `backend/` ve mevcut custom katmanlari ile saglanir.
- Market, Flipper ve Dashboard sayfalari frontend build alacak hale getirildi.
- `web/node_modules/` ve `web/.next/` gibi uretilen klasorler git disina alindi.

## Kullanilan Teknolojiler

- Go backend
- Fiber HTTP router
- Next.js / React frontend
- Local collector ve market order isleme katmani

## Mimari Kararlar

- Frontend varsayilan backend adresi `web/lib/backend.ts` icinde
  `http://localhost:8082` olarak tutulur.
- Web sayfalari backend API'lerine dogrudan fetch ile baglanir.
- Uretilen paket/build klasorleri repository takibine alinmaz.

## Veritabani / Servis / API Notlari

- Market endpointleri: `/api/items`, `/api/pricecheck/:item_id`,
  `/api/flipper`.
- Private dashboard endpointleri: `/api/private/dev/bootstrap`,
  `/api/private/dashboard`, `/api/private/me/profile`.

## Onemli Dosyalar

- `web/app/market/page.tsx`
- `web/app/flipper/page.tsx`
- `web/app/dashboard/page.tsx`
- `web/lib/backend.ts`
- `.gitignore`

## Yapilan Son Isler

- `.gitignore` GitHub'a yukleme oncesi genisletildi.
- `**/node_modules/`, `**/.next/`, cache, local DB, `.env` ve local lisans
  dosyalari Git disina alindi.
- `git status --short -uall` 43 satir, ignored olmayan yeni dosya sayisi 34
  olarak dogrulandi.
- Kirik kalan web sayfalari tamamlandi.
- Next build dogrulandi.
- Git status kalabaligi icin uretilen frontend klasorleri ignore edildi.

## Devam Eden Isler

- Su anda aktif yarim kalan is yok.

## Siradaki Adimlar

- Backend calisirken `/market`, `/flipper` ve `/dashboard` ekranlari tarayicida
  kontrol edilebilir.
- Gerekiyorsa browser console hatalari yeniden incelenebilir.

## Acik Sorular

- Yok.

## Dikkat Edilmesi Gerekenler

- Kullaniciya ait token, lisans, sifre veya connection string kayit dosyalarina
  yazilmayacak.
- GitHub Desktop 10k dosya gostermeye devam ederse refresh/restart denenmeli;
  CLI tarafinda `web/node_modules` ve `web/.next` takip edilmiyor.
