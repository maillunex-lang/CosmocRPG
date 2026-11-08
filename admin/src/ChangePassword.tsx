import { useState, type FormEvent } from "react";
import { api, ApiError } from "./api";

export const MIN_PASSWORD_LEN = 12;

/** Проверка нового пароля на клиенте (сервер проверяет то же самое независимо). */
export function validateNewPassword(current: string, next: string, repeat: string): string {
  if ([...next].length < MIN_PASSWORD_LEN) return `Новый пароль короче ${MIN_PASSWORD_LEN} символов`;
  if (next !== repeat) return "Новые пароли не совпадают";
  if (next === current) return "Новый пароль совпадает с текущим";
  return "";
}

export function ChangePassword() {
  const [open, setOpen] = useState(false);
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [repeat, setRepeat] = useState("");
  const [error, setError] = useState("");
  const [done, setDone] = useState("");
  const [busy, setBusy] = useState(false);

  function close() {
    setOpen(false);
    setCurrent("");
    setNext("");
    setRepeat("");
    setError("");
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    setDone("");
    const problem = validateNewPassword(current, next, repeat);
    if (problem) {
      setError(problem);
      return;
    }
    setBusy(true);
    setError("");
    try {
      const res = await api.changePassword(current, next);
      close();
      setDone(
        res.other_sessions_revoked > 0
          ? `Пароль изменён. Завершено других сессий: ${res.other_sessions_revoked}.`
          : "Пароль изменён.",
      );
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Неизвестная ошибка");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="card" aria-label="Смена пароля">
      <h2>Пароль</h2>
      {done && (
        <p role="status" className="ok">
          {done}
        </p>
      )}
      {!open ? (
        <button type="button" onClick={() => setOpen(true)}>
          Сменить пароль
        </button>
      ) : (
        <form onSubmit={submit} className="stack">
          <label>
            Текущий пароль
            <input type="password" value={current} onChange={(e) => setCurrent(e.target.value)} autoComplete="current-password" required />
          </label>
          <label>
            Новый пароль (не короче {MIN_PASSWORD_LEN} символов)
            <input type="password" value={next} onChange={(e) => setNext(e.target.value)} autoComplete="new-password" required />
          </label>
          <label>
            Повторите новый пароль
            <input type="password" value={repeat} onChange={(e) => setRepeat(e.target.value)} autoComplete="new-password" required />
          </label>
          {error && (
            <p role="alert" className="error">
              {error}
            </p>
          )}
          <div className="row">
            <button type="submit" disabled={busy}>
              {busy ? "Сохранение…" : "Сохранить"}
            </button>
            <button type="button" className="secondary" onClick={close} disabled={busy}>
              Отмена
            </button>
          </div>
        </form>
      )}
    </section>
  );
}
