"use client";

import { useEffect, useState } from "react";
import { backendUrl } from "../../lib/backend";

type DashboardUser = {
  id: string;
  handle: string;
  displayName: string;
  preferredCharacterName: string;
  apiToken?: string;
};

type DashboardPayload = {
  user: DashboardUser;
  filter: string;
  collectorLaunchArgs: string;
  playerState?: {
    characterName?: string;
    guildName?: string;
    locationId?: string;
    updatedAt?: string;
  };
  recentTrades: Array<{
    eventId: string;
    sessionId: string;
    location: string;
    completedAt: string;
    localPartyName: string;
    remotePartyName: string;
    netProfit: number;
  }>;
  recentEvents: Array<{
    eventId: string;
    eventType: string;
    occurredAt: string;
    locationId: string;
  }>;
};

const tokenStorageKey = "albion-personal-dev-token";

function formatDate(value?: string) {
  if (!value) {
    return "-";
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  return parsed.toLocaleString("tr-TR");
}

function formatSilver(value: number) {
  return value.toLocaleString("tr-TR");
}

export default function DashboardPage() {
  const [handle, setHandle] = useState("local-user");
  const [displayName, setDisplayName] = useState("Local User");
  const [preferredCharacterName, setPreferredCharacterName] = useState("");
  const [token, setToken] = useState("");
  const [profileCharacterName, setProfileCharacterName] = useState("");
  const [dashboard, setDashboard] = useState<DashboardPayload | null>(null);
  const [status, setStatus] = useState("Hazir");
  const [error, setError] = useState("");

  useEffect(() => {
    const savedToken = window.localStorage.getItem(tokenStorageKey) ?? "";
    if (savedToken) {
      setToken(savedToken);
    }
  }, []);

  useEffect(() => {
    if (token) {
      window.localStorage.setItem(tokenStorageKey, token);
    }
  }, [token]);

  async function bootstrapUser() {
    setStatus("Kullanici olusturuluyor...");
    setError("");
    try {
      const response = await fetch(backendUrl("/api/private/dev/bootstrap"), {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          handle,
          displayName,
          preferredCharacterName,
        }),
      });
      if (!response.ok) {
        throw new Error(`Bootstrap ${response.status} dondu`);
      }
      const payload = (await response.json()) as { user?: DashboardUser };
      const nextToken = payload.user?.apiToken ?? "";
      setToken(nextToken);
      setProfileCharacterName(payload.user?.preferredCharacterName ?? "");
      setStatus("Kullanici hazir");
      await loadDashboard(nextToken);
    } catch (err) {
      setStatus("Hata");
      setError(err instanceof Error ? err.message : "Bootstrap yapilamadi");
    }
  }

  async function loadDashboard(forcedToken?: string) {
    const currentToken = forcedToken ?? token;
    if (!currentToken) {
      setError("Once bir token olustur ya da gir");
      return;
    }

    setStatus("Dashboard yukleniyor...");
    setError("");
    try {
      const response = await fetch(backendUrl("/api/private/dashboard"), {
        headers: {
          Authorization: `Bearer ${currentToken}`,
        },
        cache: "no-store",
      });
      if (!response.ok) {
        throw new Error(`Dashboard ${response.status} dondu`);
      }
      const payload = (await response.json()) as DashboardPayload;
      setDashboard(payload);
      setDisplayName(payload.user?.displayName ?? displayName);
      setProfileCharacterName(payload.user?.preferredCharacterName ?? "");
      setStatus("Dashboard hazir");
    } catch (err) {
      setStatus("Hata");
      setDashboard(null);
      setError(err instanceof Error ? err.message : "Dashboard alinamadi");
    }
  }

  async function saveProfile() {
    if (!token) {
      setError("Profil kaydetmek icin token gerekli");
      return;
    }

    setStatus("Profil kaydediliyor...");
    setError("");
    try {
      const response = await fetch(backendUrl("/api/private/me/profile"), {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          displayName,
          preferredCharacterName: profileCharacterName,
        }),
      });
      if (!response.ok) {
        throw new Error(`Profil update ${response.status} dondu`);
      }
      setStatus("Profil kaydedildi");
      await loadDashboard(token);
    } catch (err) {
      setStatus("Hata");
      setError(err instanceof Error ? err.message : "Profil kaydedilemedi");
    }
  }

  function clearToken() {
    window.localStorage.removeItem(tokenStorageKey);
    setToken("");
    setDashboard(null);
    setStatus("Token temizlendi");
    setError("");
  }

  return (
    <main className="page-shell">
      <section className="hero">
        <p className="eyebrow">Private dashboard</p>
        <h1>Kisisel collector verisi.</h1>
        <p className="lede">
          Bu ekran local backend icindeki kullanici tokeniyle collector
          olaylarini, oyuncu durumunu ve son trade raporlarini ayirir.
        </p>
      </section>

      <section className="summary-row">
        <div className="card compact-card">
          <h2>Durum</h2>
          <p>{status}</p>
        </div>
        <div className="card compact-card">
          <h2>Aktif filtre</h2>
          <p>{dashboard?.filter || profileCharacterName || "-"}</p>
        </div>
        <div className="card compact-card">
          <h2>Trade sayisi</h2>
          <p>{dashboard?.recentTrades.length ?? 0}</p>
        </div>
      </section>

      {error && (
        <section className="status-panel error-panel">
          <strong>Baglanti sorunu</strong>
          <p>{error}</p>
        </section>
      )}

      <section className="grid two-column-grid">
        <div className="card dark-card">
          <h2>Kullanici hazirla</h2>
          <div className="control-grid">
            <label className="field dark-field">
              <span>Handle</span>
              <input
                value={handle}
                onChange={(event) => setHandle(event.target.value)}
              />
            </label>
            <label className="field dark-field">
              <span>Gorunen ad</span>
              <input
                value={displayName}
                onChange={(event) => setDisplayName(event.target.value)}
              />
            </label>
            <label className="field dark-field">
              <span>Karakter adi</span>
              <input
                value={preferredCharacterName}
                onChange={(event) =>
                  setPreferredCharacterName(event.target.value)
                }
              />
            </label>
          </div>
          <div className="button-row market-action-row">
            <button className="primary-button" type="button" onClick={bootstrapUser}>
              Kullanici olustur
            </button>
          </div>
        </div>

        <div className="card dark-card">
          <h2>Token ile baglan</h2>
          <div className="control-grid">
            <label className="field dark-field">
              <span>API token</span>
              <input
                value={token}
                onChange={(event) => setToken(event.target.value.trim())}
                placeholder="Bearer token"
              />
            </label>
            <label className="field dark-field">
              <span>Profil karakter adi</span>
              <input
                value={profileCharacterName}
                onChange={(event) => setProfileCharacterName(event.target.value)}
              />
            </label>
          </div>
          <div className="button-row market-action-row">
            <button
              className="secondary-button"
              type="button"
              onClick={() => loadDashboard()}
            >
              Dashboard yukle
            </button>
            <button className="secondary-button" type="button" onClick={saveProfile}>
              Profili kaydet
            </button>
            <button className="secondary-button danger-button" type="button" onClick={clearToken}>
              Token temizle
            </button>
          </div>
        </div>
      </section>

      {dashboard && (
        <>
          <section className="grid two-column-grid">
            <div className="card dark-card">
              <h2>Oyuncu durumu</h2>
              <p>
                Karakter: {dashboard.playerState?.characterName || "-"}
                <br />
                Guild: {dashboard.playerState?.guildName || "-"}
                <br />
                Lokasyon: {dashboard.playerState?.locationId || "-"}
                <br />
                Guncel: {formatDate(dashboard.playerState?.updatedAt)}
              </p>
            </div>

            <div className="card dark-card">
              <h2>Collector argumanlari</h2>
              <p>
                <code>{dashboard.collectorLaunchArgs}</code>
              </p>
            </div>
          </section>

          <section className="table-shell dark-table-shell">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Trade</th>
                  <th>Lokasyon</th>
                  <th>Taraflar</th>
                  <th>Net kar</th>
                  <th>Tarih</th>
                </tr>
              </thead>
              <tbody>
                {dashboard.recentTrades.map((trade) => (
                  <tr key={trade.eventId}>
                    <td className="muted-code">{trade.sessionId || trade.eventId}</td>
                    <td>{trade.location || "-"}</td>
                    <td>
                      {trade.localPartyName || "-"} /{" "}
                      {trade.remotePartyName || "-"}
                    </td>
                    <td>
                      <strong
                        className={
                          trade.netProfit >= 0 ? "profit-cell" : "sell-text"
                        }
                      >
                        {formatSilver(trade.netProfit)}
                      </strong>
                    </td>
                    <td>{formatDate(trade.completedAt)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>

          <section className="table-shell dark-table-shell">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Event</th>
                  <th>Tip</th>
                  <th>Lokasyon</th>
                  <th>Tarih</th>
                </tr>
              </thead>
              <tbody>
                {dashboard.recentEvents.map((event) => (
                  <tr key={event.eventId}>
                    <td className="muted-code">{event.eventId}</td>
                    <td>{event.eventType}</td>
                    <td>{event.locationId || "-"}</td>
                    <td>{formatDate(event.occurredAt)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>
        </>
      )}
    </main>
  );
}
