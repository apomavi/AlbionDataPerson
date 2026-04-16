# Upstream Strategy

This repository now has a thinner integration boundary between the upstream
client flow and your local custom features.

## Current state

- Local custom branch: `main`
- Personal remote: `origin`
- Original project remote: `upstream`
- Common ancestor with upstream: `0.1.49` (`d4fa79a`)
- Latest fetched upstream: `0.1.52` on `upstream/master` (`64dc935`)

## What changed

Custom/background services are no longer started directly from the main entry
path, and public upload side effects are no longer called directly from the
dispatcher.

Instead, the core now uses a small hook layer:

- `client/hooks.go`
- `albiondata-client.go`
- `client/dispatcher.go`

This means future upstream merges should mostly collide in a few integration
points instead of across every custom feature file.

## Recommended structure going forward

Keep these categories separate:

1. Upstream core:
   Files that should stay as close as possible to `upstream/master`.
2. Thin integration layer:
   Small hook files that connect upstream runtime events to your custom code.
3. Local features:
   Files like your flipper, web UI, DB sync logic, location helpers, and any
   future custom tools.

## Safe update flow

Before updating, commit or stash your current work.

```powershell
git fetch upstream --tags
git checkout main
git merge upstream/master
```

If there are conflicts, resolve them in this order:

1. Keep upstream behavior first in packet/protocol/core files.
2. Re-attach your custom behavior only through the hook layer when possible.
3. Re-test with vendor mode:

```powershell
$env:GOCACHE="$PWD\\.gocache"
$env:GOMODCACHE="$PWD\\.gomodcache"
go build -mod=vendor ./...
```

## Next improvement

The next logical step is to move the local feature implementation files out of
the `client` package into a dedicated package such as `custom` or `addons`.
The hook layer added in this session is meant to make that second refactor much
smaller and safer.
