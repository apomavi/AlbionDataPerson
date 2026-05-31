"use client";

import { useEffect, useMemo, useState } from "react";
import { backendBase, backendUrl } from "../../lib/backend";

type FlipResult = {
  item_id: string;
  item_name: string;
  tier: number;
  enchant: number;
  quality: number;
  buy_from_location: string;
  sell_to_location: string;
  buy_price: number;
  sell_price: number;
  buy_amount: number;
  sell_amount: number;
  profit_premium: number;
  profit_normal: number;
  profit_percentage: number;
  buy_age_mins: number;
  sell_age_mins: number;
  buy_source: string;
  sell_source: string;
};

type SortKey =
  | "profit_premium"
  | "profit_percentage"
  | "buy_price"
  | "sell_price"
  | "item_name";

const cityOptions = [
  "Hepsi",
  "Lymhurst",
  "Fort Sterling",
  "Martlock",
  "Bridgewatch",
  "Thetford",
  "Caerleon",
];

const qualityMap: Record<number, string> = {
  1: "Normal",
  2: "Good",
  3: "Outstanding",
  4: "Excellent",
  5: "Masterpiece",
};

const consumedStorageKey = "albion-personal-consumed-flips";

function itemImageUrl(itemId: string, quality = 1) {
  return `https://render.albiononline.com/v1/item/${encodeURIComponent(itemId)}.png?quality=${quality}`;
}

function formatSilver(value: number) {
  return value.toLocaleString("tr-TR");
}

function formatAge(minutes: number) {
  if (minutes < 1) {
    return "simdi";
  }
  if (minutes < 60) {
    return `${minutes} dk`;
  }
  if (minutes < 1440) {
    return `${Math.floor(minutes / 60)} sa`;
  }
  return `${Math.floor(minutes / 1440)} gun`;
}

export default function FlipperPage() {
  const [allItems, setAllItems] = useState<FlipResult[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [city, setCity] = useState("Hepsi");
  const [useAODP, setUseAODP] = useState(false);
  const [showNonPremium, setShowNonPremium] = useState(false);
  const [hideConsumed, setHideConsumed] = useState(true);
  const [minProfit, setMinProfit] = useState(5000);
  const [minProfitPct, setMinProfitPct] = useState(10);
  const [maxBuyAge, setMaxBuyAge] = useState(60);
  const [maxSellAge, setMaxSellAge] = useState(120);
  const [minAmount, setMinAmount] = useState(1);
  const [searchText, setSearchText] = useState("");
  const [selectedTiers, setSelectedTiers] = useState<number[]>([4, 5, 6, 7, 8]);
  const [consumedItems, setConsumedItems] = useState<string[]>([]);
  const [sortKey, setSortKey] = useState<SortKey>("profit_premium");
  const [sortDesc, setSortDesc] = useState(true);

  useEffect(() => {
    const saved = window.localStorage.getItem(consumedStorageKey);
    if (!saved) {
      return;
    }
    try {
      const parsed = JSON.parse(saved) as string[];
      setConsumedItems(Array.isArray(parsed) ? parsed : []);
    } catch {
      setConsumedItems([]);
    }
  }, []);

  useEffect(() => {
    window.localStorage.setItem(
      consumedStorageKey,
      JSON.stringify(consumedItems),
    );
  }, [consumedItems]);

  async function load() {
    setLoading(true);
    setError("");
    try {
      const params = new URLSearchParams({
        city_filter: city,
        use_aodp: String(useAODP),
      });
      const response = await fetch(`${backendUrl("/api/flipper")}?${params}`, {
        cache: "no-store",
      });
      if (!response.ok) {
        throw new Error(`Backend ${response.status} dondu`);
      }
      const payload = (await response.json()) as FlipResult[];
      setAllItems(Array.isArray(payload) ? payload : []);
    } catch (err) {
      setAllItems([]);
      setError(err instanceof Error ? err.message : "Flipper verisi alinamadi");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, [city, useAODP]);

  const filteredItems = useMemo(() => {
    const loweredSearch = searchText.trim().toLocaleLowerCase("tr-TR");
    const consumedSet = new Set(consumedItems);
    const items = allItems.filter((item) => {
      if (hideConsumed && consumedSet.has(item.item_id)) {
        return false;
      }
      if (!selectedTiers.includes(item.tier)) {
        return false;
      }
      if (item.buy_age_mins > maxBuyAge || item.sell_age_mins > maxSellAge) {
        return false;
      }
      if (item.buy_amount < minAmount) {
        return false;
      }
      if (item.profit_percentage < minProfitPct) {
        return false;
      }
      if (showNonPremium) {
        if (item.profit_normal < minProfit) {
          return false;
        }
      } else if (item.profit_premium < minProfit) {
        return false;
      }
      if (loweredSearch) {
        const haystack = `${item.item_name} ${item.item_id}`.toLocaleLowerCase(
          "tr-TR",
        );
        if (!haystack.includes(loweredSearch)) {
          return false;
        }
      }
      return true;
    });

    return [...items].sort((left, right) => {
      const leftValue =
        sortKey === "profit_premium" && showNonPremium
          ? left.profit_normal
          : left[sortKey];
      const rightValue =
        sortKey === "profit_premium" && showNonPremium
          ? right.profit_normal
          : right[sortKey];

      if (typeof leftValue === "string" && typeof rightValue === "string") {
        return sortDesc
          ? rightValue.localeCompare(leftValue)
          : leftValue.localeCompare(rightValue);
      }

      const leftNumber = Number(leftValue);
      const rightNumber = Number(rightValue);
      return sortDesc ? rightNumber - leftNumber : leftNumber - rightNumber;
    });
  }, [
    allItems,
    consumedItems,
    hideConsumed,
    maxBuyAge,
    maxSellAge,
    minAmount,
    minProfit,
    minProfitPct,
    searchText,
    selectedTiers,
    showNonPremium,
    sortDesc,
    sortKey,
  ]);

  function toggleTier(tier: number) {
    setSelectedTiers((current) =>
      current.includes(tier)
        ? current.filter((value) => value !== tier)
        : [...current, tier],
    );
  }

  function markConsumed(itemID: string) {
    setConsumedItems((current) =>
      current.includes(itemID) ? current : [...current, itemID],
    );
  }

  function clearConsumed() {
    setConsumedItems([]);
  }

  function sortBy(nextKey: SortKey) {
    if (sortKey === nextKey) {
      setSortDesc((current) => !current);
      return;
    }
    setSortKey(nextKey);
    setSortDesc(nextKey !== "item_name");
  }

  const profitLabel = showNonPremium ? "Normal kar" : "Premium kar";

  return (
    <main className="page-shell">
      <section className="hero hero-dark">
        <p className="eyebrow">Flipper</p>
        <h1>Black Market firsatlarini sirala.</h1>
        <p className="lede">
          Local taranan veriyi ve istersen AODP fiyatlarini birlestirir. Royal
          sehirlerden alip Black Market'e satilabilecek karlari filtreler.
        </p>
      </section>

      <section className="control-surface">
        <div className="control-grid flipper-control-grid">
          <label className="field">
            <span>Alis sehri</span>
            <select value={city} onChange={(event) => setCity(event.target.value)}>
              {cityOptions.map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </select>
          </label>

          <label className="field">
            <span>Arama</span>
            <input
              value={searchText}
              onChange={(event) => setSearchText(event.target.value)}
              placeholder="Esya adi veya item id"
            />
          </label>

          <label className="field">
            <span>Min kar</span>
            <input
              min="0"
              type="number"
              value={minProfit}
              onChange={(event) => setMinProfit(Number(event.target.value) || 0)}
            />
          </label>

          <label className="field">
            <span>Min kar yuzdesi</span>
            <input
              min="0"
              type="number"
              value={minProfitPct}
              onChange={(event) =>
                setMinProfitPct(Number(event.target.value) || 0)
              }
            />
          </label>

          <label className="field">
            <span>Max alis yasi (dk)</span>
            <input
              min="1"
              type="number"
              value={maxBuyAge}
              onChange={(event) => setMaxBuyAge(Number(event.target.value) || 1)}
            />
          </label>

          <label className="field">
            <span>Max BM yasi (dk)</span>
            <input
              min="1"
              type="number"
              value={maxSellAge}
              onChange={(event) => setMaxSellAge(Number(event.target.value) || 1)}
            />
          </label>

          <label className="field">
            <span>Min adet</span>
            <input
              min="1"
              type="number"
              value={minAmount}
              onChange={(event) => setMinAmount(Number(event.target.value) || 1)}
            />
          </label>
        </div>

        <div className="tier-bar">
          {[4, 5, 6, 7, 8].map((tier) => (
            <button
              className={`tier-pill ${selectedTiers.includes(tier) ? "active" : ""}`}
              key={tier}
              type="button"
              onClick={() => toggleTier(tier)}
            >
              T{tier}
            </button>
          ))}
        </div>

        <div className="toggle-row">
          <label className="checkbox-pill">
            <input
              checked={useAODP}
              type="checkbox"
              onChange={(event) => setUseAODP(event.target.checked)}
            />
            AODP verisini dahil et
          </label>
          <label className="checkbox-pill">
            <input
              checked={showNonPremium}
              type="checkbox"
              onChange={(event) => setShowNonPremium(event.target.checked)}
            />
            Premiumsiz kar hesapla
          </label>
          <label className="checkbox-pill">
            <input
              checked={hideConsumed}
              type="checkbox"
              onChange={(event) => setHideConsumed(event.target.checked)}
            />
            Isaretlenenleri gizle
          </label>
          <button className="secondary-button" type="button" onClick={load}>
            Yenile
          </button>
          <button className="secondary-button" type="button" onClick={clearConsumed}>
            Isaretleri temizle
          </button>
        </div>
      </section>

      <section className={`status-panel ${error ? "error-panel" : "dark-panel"}`}>
        <strong>{loading ? "Yukleniyor" : `${filteredItems.length} firsat`}</strong>
        <p>
          API: {backendBase} | Kaynak: local veri
          {useAODP ? " + AODP" : ""}
        </p>
        {error && <p>{error}</p>}
      </section>

      <section className="table-shell dark-table-shell">
        <table className="data-table">
          <thead>
            <tr>
              <th onClick={() => sortBy("item_name")}>Esya</th>
              <th onClick={() => sortBy("buy_price")}>Alis</th>
              <th onClick={() => sortBy("sell_price")}>Black Market</th>
              <th onClick={() => sortBy("profit_premium")}>{profitLabel}</th>
              <th onClick={() => sortBy("profit_percentage")}>Yuzde</th>
              <th>Durum</th>
            </tr>
          </thead>
          <tbody>
            {filteredItems.slice(0, 250).map((item) => {
              const displayProfit = showNonPremium
                ? item.profit_normal
                : item.profit_premium;
              return (
                <tr key={`${item.item_id}-${item.quality}-${item.buy_from_location}`}>
                  <td>
                    <div className="flip-item-cell">
                      <img
                        alt=""
                        className="flip-item-image"
                        src={itemImageUrl(item.item_id, item.quality)}
                      />
                      <div className="flip-item-meta">
                        <strong>{item.item_name || item.item_id}</strong>
                        <span>
                          {item.item_id} | T{item.tier}.{item.enchant} |{" "}
                          {qualityMap[item.quality] ?? item.quality}
                        </span>
                      </div>
                    </div>
                  </td>
                  <td>
                    <div className="flip-price-block">
                      <strong>{formatSilver(item.buy_price)}</strong>
                      <span>
                        {item.buy_from_location}
                        {item.buy_source && (
                          <span className="source-badge">{item.buy_source}</span>
                        )}
                      </span>
                      <span className="muted-line">
                        {item.buy_amount} adet | {formatAge(item.buy_age_mins)}
                      </span>
                    </div>
                  </td>
                  <td>
                    <div className="flip-price-block">
                      <strong>{formatSilver(item.sell_price)}</strong>
                      <span>
                        {item.sell_to_location}
                        {item.sell_source && (
                          <span className="source-badge">{item.sell_source}</span>
                        )}
                      </span>
                      <span className="muted-line">
                        {item.sell_amount} adet | {formatAge(item.sell_age_mins)}
                      </span>
                    </div>
                  </td>
                  <td>
                    <strong className="profit-cell">
                      {formatSilver(displayProfit)}
                    </strong>
                  </td>
                  <td>{item.profit_percentage.toFixed(1)}%</td>
                  <td>
                    <button
                      className="secondary-button"
                      type="button"
                      onClick={() => markConsumed(item.item_id)}
                    >
                      Isaretle
                    </button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </section>
    </main>
  );
}
