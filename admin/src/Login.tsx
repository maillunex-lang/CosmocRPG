import { useState, type FormEvent } from "react";
import { api, ApiError } from "./api";

export function Login({ onLoggedIn }: { onLoggedIn: (username: string) => void }) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const res = await api.login(username, password);
      setPassword("");
      onLoggedIn(res.username);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Неизвестная ошибка");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="card login">
      <h1>Universal Space Studio</h1>
      <form onSubmit={submit}>
        <label>
          Имя пользователя
          <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" autoFocus required />
        </label>
        <label>
          Пароль
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            required
          />
        </label>
        {error && (
          <p role="alert" className="error">
            {error}
          </p>
        )}
        <button type="submit" disabled={busy}>
          {busy ? "Вход…" : "Войти"}
        </button>
      </form>
    </main>
  );
}
