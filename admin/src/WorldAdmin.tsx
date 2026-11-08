import { useCallback, useEffect, useState } from "react";
import { ApiError } from "./api";
import { worldApi, type WorldCheck, type WorldVersion } from "./world";

const err = (e: unknown) => (e instanceof ApiError ? e.message : "Неизвестная ошибка");

export function WorldAdmin({ username, onBack, onLoggedOut }: { username: string; onBack: () => void; onLoggedOut: () => void }) {
  const [text, setText] = useState("");
  const [version, setVersion] = useState(0);
  const [versions, setVersions] = useState<WorldVersion[]>([]);
  const [enabled, setEnabled] = useState(true);
  const [check, setCheck] = useState<WorldCheck | null>(null);
  const [checked, setChecked] = useState("");
  const [note, setNote] = useState("");
  const [msg, setMsg] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(() => {
    worldApi.get().then((d) => {
      setText(JSON.stringify(d.config, null, 2));
      setVersion(d.version); setVersions(d.versions ?? []); setEnabled(d.living_enabled); setCheck(d.preview); setChecked(JSON.stringify(d.config, null, 2));
    }).catch((e) => setError(err(e)));
  }, []);
  useEffect(load, [load]);

  async function runCheck() {
    setError(""); setMsg("");
    try { JSON.parse(text); } catch { setError("Это не корректный JSON"); return; }
    setBusy(true);
    try { const r = await worldApi.check(text); setCheck(r); setChecked(text); setMsg(r.ok ? "Конфигурация корректна" : ""); } catch (e) { setError(err(e)); } finally { setBusy(false); }
  }

  async function publish() {
    setError(""); setMsg("");
    let cfg: unknown;
    try { cfg = JSON.parse(text); } catch { setError("Это не корректный JSON"); return; }
    setBusy(true);
    try {
      const r = await worldApi.publish(cfg, note, version);
      setMsg(r.living_enabled ? `Опубликована версия ${r.version}. Локации и враги применены в игре сразу (враги и месторождения возродятся заново); графика у игроков обновится при следующем входе.` : `Опубликована версия ${r.version}. Живые локации на сервере выключены флагом, конфигурация сохранена и применится после включения.`);
      setNote(""); load();
    } catch (e) { setError(err(e)); } finally { setBusy(false); }
  }

  async function rollback(id: number) {
    if (!window.confirm(`Откатить конфигурацию мира к версии ${id}? Будет создана новая версия.`)) return;
    try { await worldApi.rollback(id); setMsg(`Откат к версии ${id} выполнен`); load(); } catch (e) { setError(err(e)); }
  }

  const canPublish = !busy && check?.ok === true && checked === text;

  return (
    <main className="dashboard studio">
      <header>
        <h1>Локации: конфигурация мира</h1>
        <div><span className="muted">{username}</span><button className="secondary" onClick={onBack}>Сервер</button><button onClick={onLoggedOut}>Выйти</button></div>
      </header>
      {error && <p role="alert" className="error">{error}</p>}
      {msg && <p className="ok">{msg}</p>}
      {!enabled && <p className="warn">Живые локации на сервере выключены (COSMOC_FEATURE_LIVING): правки сохраняются, но в игре не действуют.</p>}

      <section className="card" aria-label="Конфигурация мира">
        <h2>Конфигурация (версия {version})</h2>
        <p className="muted">Системы (радиус, seed, число планет, поясов, пиратских зон, станций, аномалий), враги по уровням, ресурсы и цены, графические пресеты low/medium/high. Сначала «Проверить», затем «Опубликовать».</p>
        <textarea className="prompt mono" rows={22} value={text} aria-label="Конфигурация мира (JSON)" spellCheck={false} onChange={(e) => setText(e.target.value)} />
        <div className="row wrap">
          <button disabled={busy} onClick={() => void runCheck()}>Проверить</button>
          <input aria-label="Комментарий к версии" placeholder="Комментарий к версии" maxLength={200} value={note} onChange={(e) => setNote(e.target.value)} />
          <button disabled={!canPublish} onClick={() => void publish()}>Опубликовать</button>
          <button className="secondary" disabled={busy} onClick={load}>Отменить правки</button>
        </div>
        {check && (
          <div className="report">
            <h3 className={check.ok ? "ok" : "error"}>{check.ok ? "Проверка пройдена" : `Ошибки (${check.errors.length})`}</h3>
            {check.errors.length > 0 && <ul>{check.errors.slice(0, 30).map((m, i) => <li key={i} className="wrap error">{m}</li>)}</ul>}
            {check.systems.length > 0 && (
              <table><thead><tr><th>Система</th><th>Тел</th><th>Станций</th><th>Зон</th><th>Месторождений</th><th>Врагов</th><th>Точек</th></tr></thead>
                <tbody>{check.systems.map((s) => <tr key={s.id}><td>{s.name} ({s.id})</td><td>{s.bodies}</td><td>{s.stations}</td><td>{s.zones}</td><td>{s.deposits}</td><td>{s.spawns}</td><td>{s.points}</td></tr>)}</tbody></table>
            )}
            {checked !== text && <p className="warn">Текст изменён после проверки: проверьте заново перед публикацией.</p>}
          </div>
        )}
      </section>

      <section className="card" aria-label="Версии конфигурации">
        <h2>Версии конфигурации</h2>
        <table><thead><tr><th>№</th><th>Время</th><th>Кто</th><th>Что</th><th /></tr></thead>
          <tbody>{versions.map((v) => (
            <tr key={v.id}><td>{v.id}{v.active ? " ●" : ""}</td><td>{new Date(v.created_at).toLocaleString("ru-RU")}</td><td>{v.admin}</td><td className="wrap">{v.note}</td>
              <td>{!v.active && <button className="secondary" onClick={() => void rollback(v.id)}>Откатить</button>}</td></tr>))}</tbody></table>
      </section>
    </main>
  );
}
