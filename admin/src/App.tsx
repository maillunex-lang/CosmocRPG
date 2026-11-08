import { useCallback, useEffect, useState } from "react";
import { api } from "./api";
import { Dashboard } from "./Dashboard";
import { Login } from "./Login";
import { Studio } from "./studio/Studio";
import { ShipsAdmin } from "./ShipsAdmin";
import { WorldAdmin } from "./WorldAdmin";
import { ChatAdmin } from "./ChatAdmin";

type View = "server" | "studio" | "ships" | "world" | "chat";
type Session = { state: "loading" } | { state: "out" } | { state: "in"; username: string };

export function App() {
  const [session, setSession] = useState<Session>({ state: "loading" });
  const [view, setView] = useState<View>("server");

  useEffect(() => {
    api
      .me()
      .then((r) => setSession({ state: "in", username: r.username }))
      .catch(() => setSession({ state: "out" }));
  }, []);

  const onLoggedOut = useCallback(() => setSession({ state: "out" }), []);

  if (session.state === "loading") return <p className="muted center">Загрузка…</p>;
  if (session.state === "out") return <Login onLoggedIn={(username) => setSession({ state: "in", username })} />;
  if (view === "studio") {
    return <Studio username={session.username} onBack={() => setView("server")} onLoggedOut={() => void api.logout().finally(onLoggedOut)} />;
  }
  if (view === "ships") {
    return <ShipsAdmin username={session.username} onBack={() => setView("server")} onLoggedOut={() => void api.logout().finally(onLoggedOut)} />;
  }
  if (view === "world") {
    return <WorldAdmin username={session.username} onBack={() => setView("server")} onLoggedOut={() => void api.logout().finally(onLoggedOut)} />;
  }
  if (view === "chat") {
    return <ChatAdmin username={session.username} onBack={() => setView("server")} onLoggedOut={() => void api.logout().finally(onLoggedOut)} />;
  }
  return <Dashboard username={session.username} onLoggedOut={onLoggedOut} onOpenStudio={() => setView("studio")} onOpenShips={() => setView("ships")} onOpenWorld={() => setView("world")} onOpenChat={() => setView("chat")} />;
}
