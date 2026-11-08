import { useState } from "react";
import { Academy } from "./Academy";
import { Captains } from "./Captains";
import { ChatGPT } from "./ChatGPT";
import { Dialogue } from "./Dialogue";
import { Overview } from "./Overview";
import { ZipManager } from "./ZipManager";

const TABS = [
  { id: "overview", title: "Обзор" },
  { id: "captains", title: "Капитаны" },
  { id: "dialogue", title: "Диалоги" },
  { id: "zip", title: "ZIP" },
  { id: "academy", title: "Академия" },
  { id: "chatgpt", title: "ChatGPT" },
] as const;
type Tab = (typeof TABS)[number]["id"];

export function Studio({ username, onBack, onLoggedOut }: { username: string; onBack: () => void; onLoggedOut: () => void }) {
  const [tab, setTab] = useState<Tab>("overview");
  return (
    <main className="dashboard studio">
      <header>
        <h1>AI Civilization Training Studio</h1>
        <div><span className="muted">{username}</span><button className="secondary" onClick={onBack}>Сервер</button><button onClick={onLoggedOut}>Выйти</button></div>
      </header>
      <nav className="tabs" aria-label="Разделы Studio">
        {TABS.map((t) => <button key={t.id} aria-current={tab === t.id ? "page" : undefined} className={tab === t.id ? "" : "secondary"} onClick={() => setTab(t.id)}>{t.title}</button>)}
      </nav>
      {tab === "overview" && <Overview />}
      {tab === "captains" && <Captains />}
      {tab === "dialogue" && <Dialogue />}
      {tab === "zip" && <ZipManager />}
      {tab === "academy" && <Academy />}
      {tab === "chatgpt" && <ChatGPT />}
    </main>
  );
}
