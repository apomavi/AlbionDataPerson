"use client";

import { useEffect, useMemo, useState } from "react";
import { backendUrl } from "../../lib/backend";

type ItemInfo = {
  id: string;
  name: string;
  tier: number;
  enchant: number;
  category: string;
  subCategory: string;
  subCategory2?: string;
};

type PriceOrder = {
  price: number;
  amount: number;
  time: string;
};

type PriceCity = {
  id: number;
  name: string;
  sell: PriceOrder;
  buy: PriceOrder;
};

type PriceView = "both" | "sell" | "buy";

const qualityOptions = [
  { value: "1", label: "1 - Normal" },
  { value: "2", label: "2 - Good" },
  { value: "3", label: "3 - Outstanding" },
  { value: "4", label: "4 - Excellent" },
  { value: "5", label: "5 - Masterpiece" },
];

const cityColors: Record<string, string> = {
  "Black Market": "#424242",
  Brecilien: "#c2185b",
  Bridgewatch: "#fbc02d",
  Caerleon: "#d32f2f",
  "Fort Sterling": "#f5f5f5",
  Lymhurst: "#388e3c",
  Martlock: "#1976d2",
  Thetford: "#7b1fa2",
};

const tierOptions = [4, 5, 6, 7, 8];
const enchantOptions = [0, 1, 2, 3, 4];
const maxSelectedItems = 24;
const categoryOrder = [
  "Weapons",
  "Head Armor",
  "Chest Armor",
  "Foot Armor",
  "Armor",
  "Capes",
  "Bags",
  "Accessories",
  "Off-Hands",
  "Gathering Equipment",
  "Consumables",
  "Crafting",
  "Resources",
  "Artifacts",
  "Mount",
  "Mounts",
  "Farming",
  "Furniture",
  "Vanity",
  "Other",
];

function itemImageUrl(itemId: string, quality = "1") {
  return `https://render.albiononline.com/v1/item/${encodeURIComponent(itemId)}.png?quality=${quality}`;
}

function normalizeText(value: string) {
  return value.toLocaleLowerCase("tr-TR");
}

function formatSilver(value: number) {
  if (!value) {
    return "-";
  }
  return value.toLocaleString("tr-TR");
}

function timeAgo(value: string) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  const diffMinutes = Math.floor((Date.now() - date.getTime()) / 60000);
  if (!Number.isFinite(diffMinutes) || diffMinutes < 0) {
    return "-";
  }
  if (diffMinutes < 1) {
    return "simdi";
  }
  if (diffMinutes < 60) {
    return `${diffMinutes} dk once`;
  }
  if (diffMinutes < 1440) {
    return `${Math.floor(diffMinutes / 60)} sa once`;
  }
  return `${Math.floor(diffMinutes / 1440)} gun once`;
}

function cityHasData(city: PriceCity) {
  return city.sell.price > 0 || city.buy.price > 0;
}

function categoryWeight(name: string) {
  const index = categoryOrder.indexOf(name);
  return index === -1 ? categoryOrder.length : index;
}

export default function MarketPage() {
  const [allItems, setAllItems] = useState<ItemInfo[]>([]);
  const [selectedCategory, setSelectedCategory] = useState("");
  const [selectedSubCategory, setSelectedSubCategory] = useState("");
  const [selectedTiers, setSelectedTiers] = useState<number[]>([]);
  const [selectedEnchants, setSelectedEnchants] = useState<number[]>([]);
  const [selectedQuality, setSelectedQuality] = useState("1");
  const [itemSearch, setItemSearch] = useState("");
  const [selectedItemIds, setSelectedItemIds] = useState<string[]>([]);
  const [maxAgeMins, setMaxAgeMins] = useState("1440");
  const [priceView, setPriceView] = useState<PriceView>("both");
  const [hideEmptyCities, setHideEmptyCities] = useState(true);
  const [cityVisibility, setCityVisibility] = useState<Record<string, boolean>>(
    {},
  );
  const [pricesByItem, setPricesByItem] = useState<Record<string, PriceCity[]>>(
    {},
  );
  const [loadingItems, setLoadingItems] = useState(true);
  const [loadingPrices, setLoadingPrices] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;

    async function loadItems() {
      setLoadingItems(true);
      setError("");
      try {
        const response = await fetch(backendUrl("/api/items"), {
          cache: "no-store",
        });
        if (!response.ok) {
          throw new Error(`Items API ${response.status} dondu`);
        }
        const payload = (await response.json()) as ItemInfo[];
        if (!cancelled) {
          setAllItems(Array.isArray(payload) ? payload : []);
          setCityVisibility(
            Object.fromEntries(
              Object.keys(cityColors).map((cityName) => [cityName, true]),
            ),
          );
        }
      } catch (err) {
        if (!cancelled) {
          setError(
            err instanceof Error ? err.message : "Esya listesi alinamadi",
          );
        }
      } finally {
        if (!cancelled) {
          setLoadingItems(false);
        }
      }
    }

    loadItems();
    return () => {
      cancelled = true;
    };
  }, []);

  const categories = useMemo(() => {
    const counts = new Map<string, number>();
    for (const item of allItems) {
      if (!item.category) {
        continue;
      }
      counts.set(item.category, (counts.get(item.category) ?? 0) + 1);
    }

    return [...counts.entries()]
      .map(([name, count]) => ({ name, count }))
      .sort(
        (left, right) =>
          categoryWeight(left.name) - categoryWeight(right.name) ||
          right.count - left.count ||
          left.name.localeCompare(right.name),
      );
  }, [allItems]);

  const filteredByCategory = useMemo(() => {
    return selectedCategory
      ? allItems.filter((item) => item.category === selectedCategory)
      : allItems;
  }, [allItems, selectedCategory]);

  const subCategories = useMemo(() => {
    const counts = new Map<string, number>();
    for (const item of filteredByCategory) {
      if (!item.subCategory) {
        continue;
      }
      counts.set(item.subCategory, (counts.get(item.subCategory) ?? 0) + 1);
    }

    return [...counts.entries()]
      .map(([name, count]) => ({ name, count }))
      .sort(
        (left, right) =>
          right.count - left.count || left.name.localeCompare(right.name),
      );
  }, [filteredByCategory]);

  const filteredItems = useMemo(() => {
    const query = normalizeText(itemSearch.trim());
    return allItems.filter((item) => {
      if (selectedCategory && item.category !== selectedCategory) {
        return false;
      }
      if (
        selectedSubCategory &&
        item.subCategory !== selectedSubCategory &&
        item.subCategory2 !== selectedSubCategory
      ) {
        return false;
      }
      if (selectedTiers.length > 0 && !selectedTiers.includes(item.tier)) {
        return false;
      }
      if (
        selectedEnchants.length > 0 &&
        !selectedEnchants.includes(item.enchant)
      ) {
        return false;
      }
      if (query) {
        const haystack = normalizeText(`${item.name} ${item.id}`);
        if (!haystack.includes(query)) {
          return false;
        }
      }
      return true;
    });
  }, [
    allItems,
    itemSearch,
    selectedCategory,
    selectedEnchants,
    selectedSubCategory,
    selectedTiers,
  ]);

  const selectedItems = useMemo(() => {
    const byID = new Map(allItems.map((item) => [item.id, item]));
    return selectedItemIds
      .map((itemID) => byID.get(itemID))
      .filter((item): item is ItemInfo => Boolean(item));
  }, [allItems, selectedItemIds]);

  useEffect(() => {
    async function fetchPrices() {
      if (selectedItemIds.length === 0) {
        setPricesByItem({});
        return;
      }

      setLoadingPrices(true);
      setError("");
      try {
        const normalizedAge = Math.max(1, Number(maxAgeMins) || 1440);
        const entries = await Promise.all(
          selectedItemIds.map(async (itemID) => {
            const params = new URLSearchParams({
              q: selectedQuality,
              max_age_mins: String(normalizedAge),
            });
            const response = await fetch(
              `${backendUrl(`/api/pricecheck/${encodeURIComponent(itemID)}`)}?${params}`,
              { cache: "no-store" },
            );
            if (!response.ok) {
              throw new Error(`Pricecheck ${response.status} dondu`);
            }
            const payload = (await response.json()) as PriceCity[];
            return [itemID, Array.isArray(payload) ? payload : []] as const;
          }),
        );
        setPricesByItem(Object.fromEntries(entries));
      } catch (err) {
        setError(err instanceof Error ? err.message : "Fiyat verisi alinamadi");
      } finally {
        setLoadingPrices(false);
      }
    }

    fetchPrices();
  }, [maxAgeMins, selectedItemIds, selectedQuality]);

  function toggleTier(tier: number) {
    setSelectedTiers((current) =>
      current.includes(tier)
        ? current.filter((value) => value !== tier)
        : [...current, tier],
    );
  }

  function toggleEnchant(enchant: number) {
    setSelectedEnchants((current) =>
      current.includes(enchant)
        ? current.filter((value) => value !== enchant)
        : [...current, enchant],
    );
  }

  function toggleCity(cityName: string) {
    setCityVisibility((current) => ({
      ...current,
      [cityName]: !(current[cityName] ?? true),
    }));
  }

  function addItem(itemID: string) {
    setSelectedItemIds((current) => {
      if (current.includes(itemID) || current.length >= maxSelectedItems) {
        return current;
      }
      return [...current, itemID];
    });
  }

  function removeItem(itemID: string) {
    setSelectedItemIds((current) => current.filter((value) => value !== itemID));
  }

  function clearFilters() {
    setSelectedCategory("");
    setSelectedSubCategory("");
    setSelectedTiers([]);
    setSelectedEnchants([]);
    setItemSearch("");
  }

  function visibleCitiesFor(itemID: string) {
    return (pricesByItem[itemID] ?? []).filter((city) => {
      if (!(cityVisibility[city.name] ?? true)) {
        return false;
      }
      if (hideEmptyCities && !cityHasData(city)) {
        return false;
      }
      return true;
    });
  }

  return (
    <main className="page-shell">
      <section className="hero hero-dark">
        <p className="eyebrow">Market</p>
        <h1>Local market verisini tara.</h1>
        <p className="lede">
          Oyun clientinin topladigi market emirlerini local backend uzerinden
          okur. Esyalari kategori, tier, enchant ve kaliteye gore filtreleyip
          sehir bazli alis/satis fiyatlarini gosterir.
        </p>
      </section>

      {error && (
        <section className="status-panel error-panel">
          <strong>Baglanti sorunu</strong>
          <p>{error}</p>
        </section>
      )}

      <section className="market-layout">
        <aside className="market-sidebar card dark-card">
          <div className="sidebar-head">
            <h2>Kategoriler</h2>
            <button className="mini-link-button" type="button" onClick={clearFilters}>
              temizle
            </button>
          </div>

          <div className="category-grid">
            <button
              className={`category-card ${selectedCategory === "" ? "active" : ""}`}
              type="button"
              onClick={() => {
                setSelectedCategory("");
                setSelectedSubCategory("");
              }}
            >
              Hepsi
              <span>{allItems.length.toLocaleString("tr-TR")} esya</span>
            </button>
            {categories.map((category) => (
              <button
                className={`category-card ${
                  selectedCategory === category.name ? "active" : ""
                }`}
                key={category.name}
                type="button"
                onClick={() => {
                  setSelectedCategory(category.name);
                  setSelectedSubCategory("");
                }}
              >
                {category.name}
                <span>{category.count.toLocaleString("tr-TR")} esya</span>
              </button>
            ))}
          </div>
        </aside>

        <section className="market-main-stack">
          <section className="control-surface">
            <div className="market-controls market-enhanced-grid">
              <label className="field">
                <span>Alt kategori</span>
                <select
                  value={selectedSubCategory}
                  onChange={(event) => setSelectedSubCategory(event.target.value)}
                >
                  <option value="">Hepsi</option>
                  {subCategories.map((subCategory) => (
                    <option key={subCategory.name} value={subCategory.name}>
                      {subCategory.name} ({subCategory.count})
                    </option>
                  ))}
                </select>
              </label>

              <label className="field">
                <span>Arama</span>
                <input
                  value={itemSearch}
                  onChange={(event) => setItemSearch(event.target.value)}
                  placeholder="Esya adi veya item id"
                />
              </label>

              <label className="field">
                <span>Kalite</span>
                <select
                  value={selectedQuality}
                  onChange={(event) => setSelectedQuality(event.target.value)}
                >
                  {qualityOptions.map((quality) => (
                    <option key={quality.value} value={quality.value}>
                      {quality.label}
                    </option>
                  ))}
                </select>
              </label>

              <label className="field">
                <span>Maksimum veri yasi (dk)</span>
                <input
                  min="1"
                  type="number"
                  value={maxAgeMins}
                  onChange={(event) => setMaxAgeMins(event.target.value)}
                />
              </label>

              <label className="field">
                <span>Fiyat gorunumu</span>
                <select
                  value={priceView}
                  onChange={(event) => setPriceView(event.target.value as PriceView)}
                >
                  <option value="both">Alis + satis</option>
                  <option value="sell">Sadece satis</option>
                  <option value="buy">Sadece alis</option>
                </select>
              </label>
            </div>

            <div className="tier-bar">
              {tierOptions.map((tier) => (
                <button
                  className={`tier-pill ${
                    selectedTiers.includes(tier) ? "active" : ""
                  }`}
                  key={tier}
                  type="button"
                  onClick={() => toggleTier(tier)}
                >
                  T{tier}
                </button>
              ))}
              {enchantOptions.map((enchant) => (
                <button
                  className={`tier-pill ${
                    selectedEnchants.includes(enchant) ? "active" : ""
                  }`}
                  key={enchant}
                  type="button"
                  onClick={() => toggleEnchant(enchant)}
                >
                  .{enchant}
                </button>
              ))}
            </div>

            <div className="toggle-row">
              <label className="checkbox-pill">
                <input
                  checked={hideEmptyCities}
                  type="checkbox"
                  onChange={(event) => setHideEmptyCities(event.target.checked)}
                />
                Bos sehirleri gizle
              </label>
            </div>

            <div className="city-toggle-row">
              {Object.keys(cityColors).map((cityName) => (
                <button
                  className={`city-chip-button ${
                    cityVisibility[cityName] ?? true ? "active" : ""
                  }`}
                  key={cityName}
                  type="button"
                  onClick={() => toggleCity(cityName)}
                >
                  {cityName}
                </button>
              ))}
            </div>
          </section>

          <section className="item-chooser-grid">
            <div className="card dark-card search-result-card">
              <div className="market-item-result-head">
                <h2>Esyalar</h2>
                <span className="muted-line">
                  {loadingItems
                    ? "Yukleniyor"
                    : `${filteredItems.length.toLocaleString("tr-TR")} sonuc`}
                </span>
              </div>

              <div className="item-result-list">
                {filteredItems.slice(0, 160).map((item) => {
                  const selected = selectedItemIds.includes(item.id);
                  return (
                    <button
                      className={`item-result-button ${selected ? "active" : ""}`}
                      key={item.id}
                      type="button"
                      onClick={() =>
                        selected ? removeItem(item.id) : addItem(item.id)
                      }
                    >
                      <img
                        alt=""
                        className="market-item-thumb"
                        src={itemImageUrl(item.id, selectedQuality)}
                      />
                      <span className="item-result-copy">
                        <strong>{item.name || item.id}</strong>
                        <small>
                          {item.id} | T{item.tier}.{item.enchant}
                        </small>
                      </span>
                    </button>
                  );
                })}
              </div>
            </div>

            <div className="card dark-card selected-item-card">
              <div className="market-item-result-head">
                <h2>Secilenler</h2>
                <span className="muted-line">
                  {selectedItems.length}/{maxSelectedItems}
                </span>
              </div>

              {selectedItems.length === 0 ? (
                <p>Fiyat gormek icin soldan esya sec.</p>
              ) : (
                <div className="selected-item-chip-list">
                  {selectedItems.map((item) => (
                    <div className="selected-item-chip" key={item.id}>
                      <img
                        alt=""
                        className="market-item-thumb"
                        src={itemImageUrl(item.id, selectedQuality)}
                      />
                      <span className="selected-item-chip-copy">
                        <strong>{item.name || item.id}</strong>
                        <span>{item.id}</span>
                      </span>
                      <button
                        className="mini-link-button"
                        type="button"
                        onClick={() => removeItem(item.id)}
                      >
                        kaldir
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </section>

          <section className="market-results-stack">
            {loadingPrices && (
              <div className="status-panel dark-panel">
                Fiyat verisi yukleniyor...
              </div>
            )}

            {selectedItems.map((item) => {
              const cities = visibleCitiesFor(item.id);
              return (
                <div className="card dark-card" key={item.id}>
                  <div className="selected-item-shell compact-selected-item">
                    <img
                      alt=""
                      className="selected-item-image compact-item-image"
                      src={itemImageUrl(item.id, selectedQuality)}
                    />
                    <div className="selected-item-copy">
                      <strong>{item.name || item.id}</strong>
                      <span>{item.id}</span>
                    </div>
                  </div>

                  {cities.length === 0 ? (
                    <p className="market-section-title">
                      Secili filtrelerle fiyat yok.
                    </p>
                  ) : (
                    <div className="price-card-grid">
                      {cities.map((city) => (
                        <article className="city-card" key={`${item.id}-${city.id}`}>
                          <div
                            className="city-card-header"
                            style={{
                              background: cityColors[city.name] ?? "#3a313d",
                              color:
                                city.name === "Fort Sterling" ? "#201b1d" : "#fff",
                            }}
                          >
                            {city.name}
                          </div>
                          <div
                            className={`city-card-body ${
                              priceView === "both" ? "" : "single-column"
                            }`}
                          >
                            {(priceView === "both" || priceView === "sell") && (
                              <div className="market-column">
                                <span className="market-label">Satis</span>
                                <strong className="market-price sell-text">
                                  {formatSilver(city.sell.price)}
                                </strong>
                                <span className="market-meta">
                                  {city.sell.amount || "-"} adet
                                </span>
                                <span className="market-meta">
                                  {timeAgo(city.sell.time)}
                                </span>
                              </div>
                            )}
                            {(priceView === "both" || priceView === "buy") && (
                              <div className="market-column">
                                <span className="market-label">Alis</span>
                                <strong className="market-price buy-text">
                                  {formatSilver(city.buy.price)}
                                </strong>
                                <span className="market-meta">
                                  {city.buy.amount || "-"} adet
                                </span>
                                <span className="market-meta">
                                  {timeAgo(city.buy.time)}
                                </span>
                              </div>
                            )}
                          </div>
                        </article>
                      ))}
                    </div>
                  )}
                </div>
              );
            })}
          </section>
        </section>
      </section>
    </main>
  );
}
