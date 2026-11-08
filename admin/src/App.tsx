import { useCallback, useEffect, useState } from "react";
import { api } from "./api";
import { Dashboard } from "./Dashboard";
import { Login } from "./Login";

type Session = { state: "loading" } | { state: "out" } | { state: "in"; username: string };

export function App() {
  const [session, setSession] = useState<Session>({ state: "loading" });

  useEffect(() => {
    api
      .me()
      .then((r) => setSession({ state: "in", username: r.username }))
      .catch(() => setSession({ state: "out" }));
  }, []);

  const onLoggedOut = useCallback(() => setSession({ state: "out" }), []);

  if (session.state === "loading") return <p className="muted center">Загрузка…</p>;
  if (session.state === "out") return <Login onLoggedIn={(username) => setSession({ state: "in", username })} />;
  return <Dashboard username={session.username} onLoggedOut={onLoggedOut} />;
}
