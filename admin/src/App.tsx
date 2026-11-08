import { useCallback, useEffect, useState } from "react";
import { api } from "./api";
import { Dashboard } from "./Dashboard";
import { Login } from "./Login";
import { Studio } from "./studio/Studio";

type View = "server" | "studio";
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
  return <Dashboard username={session.username} onLoggedOut={onLoggedOut} onOpenStudio={() => setView("studio")} />;
}
