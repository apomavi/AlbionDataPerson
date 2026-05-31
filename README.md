<!-- [![CircleCI](https://circleci.com/gh/broderickhyman/albiondata-client/tree/master.svg?style=svg)](https://circleci.com/gh/broderickhyman/albiondata-client/tree/master) [![Go Report Card](https://goreportcard.com/badge/github.com/broderickhyman/albiondata-client)](https://goreportcard.com/report/github.com/broderickhyman/albiondata-client)
-->

# Albion Data - Client
Distributed client for the [Albion Online Data](https://www.albion-online-data.com/)
project.

## Personal Fork Workflow
This repository is used as a personal fork that still tracks updates from the original `ao-data/albiondata-client` project.

Main goals:
- keep pulling upstream fixes from the original project
- keep custom logic mostly inside `custom/`
- keep core merge conflicts as small as possible

## Current Monorepo Direction
This repository is being shaped into three clear runtime spines while keeping the current system working:

- `client`: game-facing collector
- `backend`: standalone ingest, processor, and business backend
- `web`: future user-facing frontend

Shared client/backend event contracts now live under `contracts/`.

### Start
Run `baslat.bat`.

It will ask which mode you want:
- `1`: only your local/custom database
- `2`: your local/custom database plus AODP upload
- `3`: collector-only mode, send events to the standalone backend
- `4`: collector-only mode, send events to the standalone backend and AODP together

In both modes, your custom database flow stays active.

### Optional Collector Mode
This fork can also post normalized gameplay events to your own backend API.

Examples:
- `albiondata-client.exe -collector-url http://localhost:9000/api/collector/events`
- `albiondata-client.exe -collector-url https://your-domain/api/collector/events -collector-token YOUR_TOKEN`

For local development with the built-in web server, you can point the client back to:
- `http://localhost:8081/api/collector/events`

Useful local inspection endpoints:
- `GET /api/collector/health`
- `GET /api/collector/events/recent?limit=25`
- `GET /api/collector/processor/status`
- `GET /api/collector/projections/player-state`
- `GET /api/collector/projections/trades/recent?limit=25`

Current normalized event coverage:
- join/player state updates
- market order snapshots
- market history snapshots
- gold price snapshots
- completed trade reports

This is the first step toward a hosted architecture where the client stays thin and your website/backend owns accounts, permissions, and product logic.

### Standalone Backend
There is now also a separate backend executable in this repo.

Start it with:
- `baslat-backend.bat`

Or manually:
- `go run ./cmd/albion-personal-backend --addr :8082`

Then point the client collector to it:
- `albiondata-client.exe -collector-url http://localhost:8082/api/collector/events`

This means the project now supports two runtime roles:
- client collector
- standalone ingest/processor backend

The backend also now exposes the first website-facing compatibility APIs:
- `GET /api/items`
- `GET /api/pricecheck/:item_id`
- `GET /api/flipper`

This lets the new `web/` frontend start talking to `backend/` directly while the old embedded Go website keeps working during migration.

Current new frontend routes:
- `/market`
- `/flipper`
- `/dashboard`

Current private backend routes:
- `POST /api/private/dev/bootstrap`
- `GET /api/private/me`
- `POST /api/private/me/profile`
- `GET /api/private/dashboard`

Collector ownership flow:
- create a dev user from the new `/dashboard` page
- copy that user's API token
- start the client in mode `3` or `4`
- when asked for `Collector kullanici tokeni`, paste that token

When the token is provided, backend events are attached to that user instead of staying anonymous.

### Update From Original Repo
Run `guncelle.bat`.

That script:
1. fetches `upstream`
2. merges `upstream/master`
3. runs a build check

If it finishes successfully, start the app again with `baslat.bat`.

### Custom Layer
Heavy custom logic was moved under `custom/`.

Only a few thin bridge calls remain in upstream core files such as:
- `client/decode.go`
- `client/dispatcher.go`
- `client/operation_join.go`
- `client/operation_gold_market_get_average_info.go`

This is intentional and makes future upstream merges easier.

A quick note on the legality of this application and if it
violates the Terms and Conditions for Albion Online. Here is
the response from SBI when asked if we are allowed to do
monitor network packets relating to Albion Online:
> Our position is quite simple. As long as you just look and
analyze we are ok with it. The moment you modify or manipulate
something or somehow interfere with our services we will react
(e.g. perma-ban, take legal action, whatever).

~ MadDave - Technical Lead for Albion Online

Source: https://forum.albiononline.com/index.php/Thread/51604-Is-it-allowed-to-scan-your-internet-trafic-and-pick-up-logs/?postID=512670#post512670

This client monitors local network traffic, identifies UDP packets
that contain relevant data for Albion Online, and ships the information
off to a central NATS server that anyone can subscribe to.

<!--
[Client download stats](https://www.somsubhra.com/github-release-stats/?username=broderickhyman&repository=albiondata-client)
-->

<!-- 
### Contributing
This process is run on a [DigitalOcean Droplet](https://www.digitalocean.com) in order to ensure almost perfect uptime and high performance for the users. If you find this project beneficial to you then please consider a donation, thanks!!

-->

# Contributions
Many thanks to the original developers:
- [Regner](https://github.com/Regner)
- [pcdummy](https://github.com/pcdummy)
- [Ultraporing](https://github.com/Ultraporing)


Many thanks also to [broderickhyman](https://github.com/broderickhyman) for picking up development and funding for the the last few years of the project!

As of 2023-01-01, [Stanx](https://github.com/phendryx) is the primary maintainer and provides funding of the related projects.  

[Walkynn](https://github.com/walkeralencar) has been a long time maintainer of different aspets of the project as well.

# Downloads
Downloads can be found here: https://github.com/ao-data/albiondata-client/releases

Stats for the client releases can be viewed [here](https://tooomm.github.io/github-release-stats/?username=ao-data&repository=albiondata-client).
## Running on Mac

### Running from the Finder
1. Download the latest `albiondata-client-amd64-mac.zip` file from [the Releases page](https://github.com/ao-data/albiondata-client/releases)
2. Unzip that file from the Finder
3. Enter the `albiondata-client` folder.
4. Double click the `run.command` file. It will ask for your password for permissions reasons.

### Running from the Terminal
1. Download the latest `update-darwin-amd64.gz` file from [the Releases page](https://github.com/ao-data/albiondata-client/releases)
2. Unzip that file from the Finder or with `gunzip update-darwin-amd64.gz`
3. The unzipped `albiondata-client` file is a Golang binary file. You'll need to make this file executable so it can be run directly. You can do this from your Terminal with: `chmod +x albiondata-client`
4. Run the client from your Terminal with `./albiondata-client`

## Running on Debian or Debian based distros

### Install app binary
<sup>`~/.local/bin` requires systemd. If you don't roll with systemd use something else. </sup>

1. Create ~/.local/bin folder: `mkdir -p ~/.local/bin`
2. Download latest `update-linux-amd64.gz` version from [the Releases page](https://github.com/ao-data/albiondata-client/releases)  
`curl -L https://github.com/ao-data/albiondata-client/releases/latest/download/update-linux-amd64.gz -o - | gzip -d > ~/.local/bin/albiondata-client`
3. Give user execution permission: `chmod u+x ~/.local/bin/albiondata-client`

### Install dependency libpcap

```bash
sudo apt install libpcap-dev
```

### Give binary permission to capture network traffic

To allow binary to capture data without using sudo

```bash
sudo setcap cap_net_raw,cap_net_admin=eip ~/.local/bin/albiondata-client
```

# Related Projects
- [albiondata-deduper-dotNet](https://github.com/ao-data/albiondata-deduper-dotNet)
- [albiondata-sql-dotNet](https://github.com/ao-data/albiondata-sql-dotNet)
- [albiondata-api-dotNet](https://github.com/ao-data/albiondata-api-dotNet)
- [AlbionData.Models](https://github.com/ao-data/albiondata-models-dotNet) [![NuGet](https://img.shields.io/nuget/v/AlbionData.Models.svg)](https://www.nuget.org/packages/AlbionData.Models/)
- [albion-data-website](https://github.com/ao-data/albion-data-website)

# Contact Us
The best way to get in touch with us is on the Albion Online Fansites Discord server in either the #proj-albiondata or the #developers channel. A permanent invite link can be found here: [https://discord.gg/TjWdq24](https://discord.gg/TjWdq24)

# Developer Setup
### Mac/Linux Setup
- Install go
- Build the project (Go modules will download automatically)

### Windows Setup
[Windows Setup Guide](https://github.com/ao-data/albiondata-client/wiki/Building-in-Windows)

# License
This project, and all contributed code, are licensed under the MIT
License. A copy of the MIT License may be found in the repository.
