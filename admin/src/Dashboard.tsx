import { useCallback, useEffect, useState } from "react";
import { api, ApiError, formatUptime, type AuditEntry, type Status } from "./api";
import { ChangePassword } from "./ChangePassword";

const REFRESH_MS = 5000;

export function Dashboard({ username, onLoggedOut, onOpenStudio, onOpenShips, onOpenWorld, onOpenChat }: { username: string; onLoggedOut: () => void; onOpenStudio?: () => void; onOpenShips?: () => void; onOpenWorld?: () => void; onOpenChat?: () => void }) {
  const [status, setStatus] = useState<Status | null>(null);
  const [audit, setAudit] = useState<AuditEntry[]>([]);
  const [error, setError] = useState("");

  const refresh = useCallback(async () => {
    try {
      const [s, a] = await Promise.all([api.status(), api.audit(20)]);
      setStatus(s);
      setAudit(a.entries);
      setError("");
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        onLoggedOut(); // сессия истекла
        return;
      }
      setError(err instanceof ApiError ? err.message : "Неизвестная ошибка");
    }
  }, [onLoggedOut]);

  useEffect(() => {
    void refresh();
    const t = setInterval(() => void refresh(), REFRESH_MS);
    return () => clearInterval(t);
  }, [refresh]);

  async function logout() {
    try {
      await api.logout();
    } finally {
      onLoggedOut();
    }
  }

  return (
    <main className="dashboard">
      <header>
        <h1>Universal Space Studio</h1>
        <div>
          <span className="muted">{username}</span>
          {onOpenStudio && <button onClick={onOpenStudio}>AI Studio</button>}
          {onOpenShips && <button onClick={onOpenShips}>Корабли</button>}
          {onOpenWorld && <button onClick={onOpenWorld}>Локации</button>}
          {onOpenChat && <button onClick={onOpenChat}>Чат</button>}
          <button onClick={() => void logout()}>Выйти</button>
        </div>
      </header>

      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}

      <section className="card" aria-label="Состояние сервера">
        <h2>Состояние сервера</h2>
        {status ? (
          <dl>
            <dt>Версия</dt>
            <dd>{status.version}</dd>
            <dt>База данных</dt>
            <dd className={status.database === "ok" ? "ok" : "error"}>{status.database === "ok" ? "работает" : "недоступна"}</dd>
            <dt>Время работы</dt>
            <dd>{formatUptime(status.uptime_seconds)}</dd>
            <dt>Игровые сессии</dt>
            <dd>{status.ws_sessions}</dd>
            <dt>Запусков сервера</dt>
            <dd>{status.boots_total}</dd>
          </dl>
        ) : (
          <p className="muted">Загрузка…</p>
        )}
      </section>

      <ChangePassword />

      <section className="card" aria-label="Журнал действий">
        <h2>Журнал административных действий</h2>
        <table>
          <thead>
            <tr>
              <th>Время</th>
              <th>Пользователь</th>
              <th>Действие</th>
              <th>IP</th>
            </tr>
          </thead>
          <tbody>
            {audit.map((e) => (
              <tr key={e.id}>
                <td>{new Date(e.at).toLocaleString("ru-RU")}</td>
                <td>{e.username}</td>
                <td>{e.action}</td>
                <td>{e.ip}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </main>
  );
}
