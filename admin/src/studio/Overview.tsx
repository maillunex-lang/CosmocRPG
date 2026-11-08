import { useCallback, useEffect, useState } from "react";
import { ai, errText, type Dashboard } from "./aiapi";

export function Overview() {
  const [d, setD] = useState<Dashboard | null>(null);
  const [err, setErr] = useState("");
  const [msg, setMsg] = useState("");
  const [count, setCount] = useState(300);
  const [busy, setBusy] = useState(false);

  const load = useCallback(() => {
    ai.dashboard().then(setD).catch((e) => setErr(errText(e)));
  }, []);
  useEffect(load, [load]);

  async function generate() {
    setBusy(true); setErr(""); setMsg("");
    try {
      const r = await ai.generate(count);
      setMsg(r.added > 0 ? `Добавлено капитанов: ${r.added}. Существующие профили не менялись.` : "Новых капитанов нет: все номера уже есть в реестре.");
      load();
    } catch (e) { setErr(errText(e)); } finally { setBusy(false); }
  }

  async function nicknames() {
    if (!window.confirm("Заменить имена капитанов игровыми никами (русские и английские)? Профили, правленные вручную, не изменятся. Версию можно откатить.")) return;
    setBusy(true); setErr(""); setMsg("");
    try {
      const r = await ai.nicknames();
      setMsg(r.renamed > 0 ? `Переименовано капитанов: ${r.renamed} (версия ${r.version})` : "Переименовывать нечего: все ники уже применены.");
      load();
    } catch (e) { setErr(errText(e)); } finally { setBusy(false); }
  }

  async function rollback(id: number) {
    if (!window.confirm(`Откатить конфигурацию к версии ${id}? История сохранится, будет создана новая версия.`)) return;
    try { await ai.rollback(id); setMsg(`Откат к версии ${id} выполнен`); load(); } catch (e) { setErr(errText(e)); }
  }

  if (!d) return <p className="muted">{err || "Загрузка…"}</p>;
  const pct = Math.min(100, Math.round((d.captains / d.target) * 100));
  const lt = d.last_tests;
  return (
    <>
      {err && <p role="alert" className="error">{err}</p>}
      {msg && <p className="ok">{msg}</p>}
      <section className="card" aria-label="Реестр капитанов">
        <h2>Капитаны: {d.captains} из {d.target}</h2>
        <div className="bar" role="progressbar" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100}><div style={{ width: `${pct}%` }} /></div>
        <dl>
          <dt>Источники профилей</dt>
          <dd>генератор {d.by_source.generator ?? 0}, ChatGPT {d.by_source.chatgpt ?? 0}, вручную {d.by_source.manual ?? 0}</dd>
          <dt>Активная версия</dt><dd>{d.active_version || "ещё нет"}</dd>
          <dt>Ошибок валидации в последних импортах</dt><dd className={d.validation_errors > 0 ? "warn" : ""}>{d.validation_errors}</dd>
          <dt>Импорты</dt>
          <dd>{Object.entries(d.statuses).map(([k, v]) => `${{ validated: "проверено", rejected: "отклонено", published: "опубликовано", discarded: "отменено" }[k] ?? k}: ${v}`).join(", ") || "нет"}</dd>
          <dt>Последние тесты</dt>
          <dd className={lt && lt.passed === lt.total ? "ok" : "warn"}>{lt ? `${lt.passed} из ${lt.total} (${new Date(lt.created_at).toLocaleString("ru-RU")})` : "ещё не запускались"}</dd>
        </dl>
        <div className="row wrap">
          <label className="inline">Всего капитанов в реестре
            <input type="number" min={1} max={400} value={count} onChange={(e) => setCount(Number(e.target.value))} style={{ width: 90 }} />
          </label>
          <button disabled={busy} onClick={() => void generate()}>{busy ? "Генерация…" : "Сгенерировать недостающих"}</button>
          <button className="secondary" disabled={busy} onClick={() => void nicknames()}>Применить игровые ники</button>
        </div>
        <p className="muted">Генерация детерминирована и безопасна для повтора: существующие и вручную правленные профили не перезаписываются.</p>
      </section>

      <section className="card" aria-label="История версий">
        <h2>Версии и публикации</h2>
        {(d.versions ?? []).length === 0 ? <p className="muted">Версий ещё нет. Сгенерируйте капитанов.</p> : (
          <table><thead><tr><th>№</th><th>Время</th><th>Кто</th><th>Что</th><th>Капитанов</th><th /></tr></thead>
            <tbody>{(d.versions ?? []).map((v) => (
              <tr key={v.id}><td>{v.id}{v.active ? " ●" : ""}</td><td>{new Date(v.created_at).toLocaleString("ru-RU")}</td><td>{v.admin}</td><td className="wrap">{v.note}</td><td>{v.captains}</td>
                <td>{!v.active && <button className="secondary" onClick={() => void rollback(v.id)}>Откатить</button>}</td></tr>
            ))}</tbody></table>
        )}
      </section>

      <section className="card" aria-label="Версии обучающих пакетов">
        <h2>Обучающие пакеты</h2>
        <h3>Выгружено</h3>
        {(d.sessions ?? []).length === 0 ? <p className="muted">Нет.</p> : (
          <table><thead><tr><th>Время</th><th>Пакет</th><th>Капитанов</th><th>SHA-256</th></tr></thead>
            <tbody>{(d.sessions ?? []).map((s) => <tr key={s.id}><td>{new Date(s.created_at).toLocaleString("ru-RU")}</td><td>{s.label}</td><td>{s.batch_size}</td><td className="mono">{s.sha256.slice(0, 12)}…</td></tr>)}</tbody></table>
        )}
        <h3>Загружено</h3>
        {(d.imports ?? []).length === 0 ? <p className="muted">Нет.</p> : (
          <table><thead><tr><th>Время</th><th>Файл</th><th>Статус</th><th>Ошибок</th><th>Предупр.</th></tr></thead>
            <tbody>{(d.imports ?? []).map((i) => <tr key={i.id}><td>{new Date(i.created_at).toLocaleString("ru-RU")}</td><td>{i.filename}</td><td>{{ validated: "проверен", rejected: "отклонён", published: "опубликован", discarded: "отменён" }[i.status] ?? i.status}</td><td>{i.report.errors.length}</td><td>{i.report.warnings.length}</td></tr>)}</tbody></table>
        )}
      </section>
    </>
  );
}
