# AI Context Index

## Proje Klasor Yapisi

- `backend/`: Yeni backend API ve servis katmani.
- `custom/`: Mevcut client/custom collector ve market katmanlari.
- `cmd/`: Go binary giris noktalari.
- `contracts/`: Collector event sozlesmeleri.
- `web/`: Next.js frontend.
- `docs/ai/`: Kalici AI calisma kayitlari.

## Onemli Dosyalar

- `.gitignore`: Repository disinda tutulacak uretilen dosyalar.
- `web/lib/backend.ts`: Frontend backend URL ayari.
- `web/app/market/page.tsx`: Market fiyat ekrani.
- `web/app/flipper/page.tsx`: Flipper firsat ekrani.
- `web/app/dashboard/page.tsx`: Private dashboard ekrani.

## Calistirma Komutlari

- Frontend build: `cd web; npm run build`
- Frontend dev: `cd web; npm run dev`

## Test Komutlari

- Web icin mevcut dogrulama: `npm run build`

## Build / Deploy Notlari

- `web/.next/` build ciktisidir ve git'e girmez.
- `web/node_modules/` dependency klasorudur ve git'e girmez.
- `node_modules`, `.next`, `dist`, `out`, `coverage`, `.turbo`, `.vercel`,
  Go cache klasorleri ve local DB dosyalari `.gitignore` ile dislanir.

## Veritabani Notlari

- Market verisi backend API uzerinden okunur.
- Bu oturumda veritabani semasi degistirilmedi.

## Harici Servisler

- Flipper istege bagli olarak Albion Online Data Project fiyat API'sini
  backend uzerinden kullanabilir.

## Bilinmemesi Gereken / Yazilmamasi Gereken Gizli Bilgiler

- Token, lisans dosyasi icerigi, sifre, connection string ve kisisel erisim
  bilgileri bu dosyalara yazilmaz.
- `.env`, `.env.*`, `.secrets` ve `license.lic` Git disinda tutulur.
