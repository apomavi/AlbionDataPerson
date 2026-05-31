import Link from "next/link";

const cards = [
  {
    title: "Client",
    text: "Oyunu dinleyen collector katmani burada yasiyor.",
  },
  {
    title: "Backend",
    text: "Ingest, event store, private API, market API ve is mantigi burada toplaniyor.",
  },
  {
    title: "Web",
    text: "Yeni arayuz market, flipper ve dashboard sayfalarini backend API ile calistiriyor.",
  },
];

export default function HomePage() {
  return (
    <main className="page-shell">
      <section className="hero hero-dark">
        <p className="eyebrow">Albion Personal</p>
        <h1>Yeni omurga artik gercekten calismaya basladi.</h1>
        <p className="lede">
          Bu repo su an tek parca dursa da artik 3 net goreve ayriliyor:
          client veri topluyor, backend veriyi sahipleniyor, web de bunu modern
          arayuzde gosteriyor.
        </p>
        <p className="lede">
          Ilk canli gecis tamamlandi: yeni{" "}
          <Link href="/flipper">flipper</Link>,{" "}
          <Link href="/market">market</Link> ve{" "}
          <Link href="/dashboard">dashboard</Link> sayfalari backend ile
          konusuyor.
        </p>
      </section>

      <section className="grid">
        {cards.map((card) => (
          <article key={card.title} className="card dark-card">
            <h2>{card.title}</h2>
            <p>{card.text}</p>
          </article>
        ))}
      </section>

      <section className="grid quick-links">
        <article className="card dark-card">
          <h2>Market</h2>
          <p>Esya arama ve sehir bazli fiyat kontrolu burada.</p>
          <p>
            <Link href="/market">Market ekranina git</Link>
          </p>
        </article>
        <article className="card dark-card">
          <h2>Flipper</h2>
          <p>Daha zengin filtreli black market flip arayuzu burada.</p>
          <p>
            <Link href="/flipper">Flipper ekranina git</Link>
          </p>
        </article>
        <article className="card dark-card">
          <h2>Dashboard</h2>
          <p>Kullanici tokeni al, clienti kendine bagla ve ozel verini izle.</p>
          <p>
            <Link href="/dashboard">Dashboard ekranina git</Link>
          </p>
        </article>
      </section>
    </main>
  );
}
