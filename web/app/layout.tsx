import "./globals.css";
import type { Metadata } from "next";
import Link from "next/link";
import type { ReactNode } from "react";

export const metadata: Metadata = {
  title: "Albion Personal Web",
  description: "Future frontend for Albion Personal",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>
        <header className="site-header">
          <div className="site-header-inner">
            <Link href="/" className="brand-mark">
              Albion Personal
            </Link>
            <nav className="site-nav">
              <Link href="/">Home</Link>
              <Link href="/market">Market</Link>
              <Link href="/flipper">Flipper</Link>
              <Link href="/dashboard">Dashboard</Link>
            </nav>
          </div>
        </header>
        {children}
      </body>
    </html>
  );
}
