# AI Session Log

## 2026-05-31 00:00 - Codex

### Amac

GitHub'a yukleme oncesi 10k dosya kalabaligini engellemek icin `.gitignore`
kontrolu ve duzenlemesi yapmak.

### Yapilanlar

- `.gitignore` bolumlere ayrildi ve genisletildi.
- Recursive `**/node_modules/`, `**/.next/`, `**/dist/`, `**/out/`,
  `**/coverage/`, `.turbo`, `.vercel`, local DB ve local secret patternleri
  eklendi.
- `git status --short -uall` ve `git ls-files --others --exclude-standard`
  ile dosya sayilari kontrol edildi.
- `git check-ignore -v` ile `web/node_modules`, `web/.next` ve `.gomodcache`
  ignore kurallari dogrulandi.

### Degisen Dosyalar

- `.gitignore`
- `docs/ai/AI_HANDOFF.md`
- `docs/ai/AI_SESSION_LOG.md`
- `docs/ai/AI_CONTEXT_INDEX.md`

### Alinan Kararlar

- Dependency, build, cache, local DB ve local lisans/env dosyalari repository'ye
  alinmayacak.

### Karsilasilan Sorunlar

- CLI tarafinda 10k dosya gorunmedi; `git status --short -uall` 43 satir
  gosterdi. GitHub Desktop eski cache veya onceki gorunumden kaynakli fazla
  dosya gosterebilir.

### Siradaki Adimlar

- GitHub'a yuklemeden once GitHub Desktop/VS Code yenilenmeli ve dosya sayisi
  tekrar kontrol edilmeli.

## 2026-05-29 23:07 - Codex

### Amac

Rollback sonrasi calismayan web arayuzunu onarmak ve git status icindeki 10k
dosya kalabaligini azaltmak.

### Yapilanlar

- `web/app/market/page.tsx` tamamlandi.
- `web/app/flipper/page.tsx` tamamlandi.
- `web/app/dashboard/page.tsx` tamamlandi.
- `.gitignore` icine frontend dependency/build klasorleri eklendi.
- `npm run build` ile web build dogrulandi.

### Degisen Dosyalar

- `.gitignore`
- `web/app/market/page.tsx`
- `web/app/flipper/page.tsx`
- `web/app/dashboard/page.tsx`
- `AGENTS.md`
- `docs/ai/AI_HANDOFF.md`
- `docs/ai/AI_SESSION_LOG.md`
- `docs/ai/AI_CONTEXT_INDEX.md`

### Alinan Kararlar

- `web/node_modules/`, `web/.next/`, `node_modules/`, `.next/`, `dist/`,
  `out/` ve `coverage/` git disinda tutulacak.
- Frontend sayfalari mevcut backend endpointlerine gore onarildi.

### Karsilasilan Sorunlar

- Market, Flipper ve Dashboard sayfalari dosya ortasinda kesildigi icin Next
  parse hatasi veriyordu.

### Siradaki Adimlar

- Backend ayaktayken web ekranlarini tarayicida test et.
