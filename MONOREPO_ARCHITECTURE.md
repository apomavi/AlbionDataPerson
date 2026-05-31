# Monorepo Architecture

Current target structure inside this single repository:

- `client/`
  Packet capture and game-facing collector behavior.
- `backend/`
  Standalone ingest API, event store, processor, and product-facing backend logic.
- `web/`
  Future user-facing website application.
- `contracts/`
  Shared event schema used by both client and backend.
- `custom/`
  Transitional compatibility layer while moving from embedded runtime to separated runtimes.

Current rule set:

1. The system must keep working during migration.
2. `baslat.bat` remains the easiest way to run the client.
3. `baslat-backend.bat` runs the standalone backend.
4. The embedded `custom` runtime is temporary compatibility code, not the long-term architecture.
5. New client/backend communication contracts should go into `contracts/`, not be duplicated.

Planned end state:

- `client` runs as a thin collector.
- `backend` owns ingest, auth, processing, projections, and business logic.
- `web` becomes a dedicated frontend app that talks to the backend API.

Current migration milestone:

- standalone `backend` now serves the first website-compatible API surface:
  - `/api/items`
  - `/api/pricecheck/:item_id`
  - `/api/flipper`
  - `/api/private/dev/bootstrap`
  - `/api/private/me`
  - `/api/private/me/profile`
  - `/api/private/dashboard`
- `web/` has started consuming backend APIs directly:
  - `/market` uses `items` + `pricecheck`
  - `/flipper` uses `flipper`
  - `/dashboard` uses the first private user/token/dashboard API set
- collector events can now be owned by a specific user token
- mode `3` and `4` in `baslat.bat` can optionally attach a collector user token
- old `custom/web.go` still exists only as a compatibility bridge while the new web spine grows

Legacy embedded screens inventory:

- `public/index.html` -> new home: `web/app/market/page.tsx`
- `public/flipper.html` -> new home: `web/app/flipper/page.tsx`
