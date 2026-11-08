import { useRef, useState } from "react";
import { ai, errText, type ImportRec } from "./aiapi";

const STATUS: Record<string, string> = { validated: "проверен", rejected: "отклонён", published: "опубликован", discarded: "отменён" };
const KIND: Record<string, string> = { added: "добавится", changed: "изменится", unchanged: "без изменений", conflict: "конфликт" };

export function ImportPanel({ onChanged }: { onChanged?: () => void }) {
  const [rec, setRec] = useState<ImportRec | null>(null);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState("");
  const [err, setErr] = useState("");
  const [overwrite, setOverwrite] = useState(false);
  const [accept, setAccept] = useState(false);
  const file = useRef<HTMLInputElement>(null);

  async function upload() {
    const f = file.current?.files?.[0];
    if (!f) { setErr("Выберите ZIP-файл"); return; }
    setBusy(true); setErr(""); setMsg(""); setAccept(false);
    try {
      setRec(await ai.importZip(f, overwrite));
      onChanged?.();
    } catch (e) { setErr(errText(e)); } finally { setBusy(false); }
  }

  async function publish() {
    if (!rec) return;
    setBusy(true); setErr("");
    try {
      const r = await ai.publish(rec.id, accept, overwrite);
      setMsg(`Опубликовано: новая версия ${r.version}`);
      setRec({ ...rec, status: "published" });
      onChanged?.();
    } catch (e) { setErr(errText(e)); } finally { setBusy(false); }
  }

  async function discard() {
    if (!rec) return;
    setBusy(true);
    try { await ai.discard(rec.id); setRec({ ...rec, status: "discarded" }); onChanged?.(); } catch (e) { setErr(errText(e)); } finally { setBusy(false); }
  }

  const r = rec?.report;
  const canPublish = rec?.status === "validated" && !busy;
  const needAccept = (r?.warnings.length ?? 0) > 0;
  return (
    <section className="card" aria-label="Импорт обученного ZIP">
      <h2>Импорт обученного ZIP</h2>
      <p className="muted">Файл сначала только проверяется. Ничего не меняется, пока вы не подтвердите публикацию.</p>
      <div className="row wrap">
        <input type="file" accept=".zip,application/zip" ref={file} aria-label="ZIP-файл" />
        <label className="inline"><input type="checkbox" checked={overwrite} onChange={(e) => setOverwrite(e.target.checked)} /> разрешить перезапись профилей, правленных вручную</label>
        <button disabled={busy} onClick={() => void upload()}>{busy ? "Проверка…" : "Проверить"}</button>
      </div>
      {err && <p role="alert" className="error">{err}</p>}
      {msg && <p className="ok">{msg}</p>}
      {rec && r && (
        <div className="report">
          <h3>Отчёт проверки: {STATUS[rec.status] ?? rec.status}</h3>
          <dl>
            <dt>Пакет</dt><dd>{rec.pack_id || "—"} ({rec.kind || "—"}), файл {rec.filename}</dd>
            <dt>SHA-256 архива</dt><dd className="mono">{rec.sha256}</dd>
            <dt>Целостность</dt>
            <dd className={r.integrity === "verified" ? "ok" : r.integrity === "failed" ? "error" : "warn"}>
              {{ verified: "подтверждена (все суммы совпали)", partial: "подтверждена частично", none: "не подтверждена (нет сумм в манифесте)", failed: "НАРУШЕНА", "": "—" }[r.integrity] ?? r.integrity}
            </dd>
            <dt>Содержимое</dt><dd>файлов {r.stats.files ?? 0}, капитанов {r.stats.captains ?? 0}, разделов {r.stats.sections ?? 0}</dd>
            <dt>Проверки качества</dt>
            <dd>{rec.gate && rec.gate.length > 0 ? rec.gate.map((g) => `${g.title}: ${g.passed}/${g.total}`).join("; ") : "не запускались"}</dd>
          </dl>
          {r.errors.length > 0 && (
            <>
              <h4 className="error">Ошибки ({r.errors.length}) — публикация невозможна</h4>
              <table><thead><tr><th>Файл</th><th>Код</th><th>Сообщение</th></tr></thead>
                <tbody>{r.errors.slice(0, 60).map((i, k) => <tr key={k}><td>{i.path}</td><td>{i.code}</td><td className="wrap">{i.message}</td></tr>)}</tbody></table>
            </>
          )}
          {r.warnings.length > 0 && (
            <>
              <h4 className="warn">Предупреждения ({r.warnings.length})</h4>
              <table><thead><tr><th>Файл</th><th>Код</th><th>Сообщение</th></tr></thead>
                <tbody>{r.warnings.slice(0, 60).map((i, k) => <tr key={k}><td>{i.path}</td><td>{i.code}</td><td className="wrap">{i.message}</td></tr>)}</tbody></table>
            </>
          )}
          {r.errors.length === 0 && (
            <>
              <h4>Предпросмотр изменений: добавится {rec.plan.added}, изменится {rec.plan.changed}, без изменений {rec.plan.unchanged}, конфликтов {rec.plan.conflicts}</h4>
              {(rec.plan.captains ?? []).filter((c) => c.kind !== "unchanged").length > 0 && (
                <table><thead><tr><th>Капитан</th><th>Что</th><th>Изменяемые поля</th></tr></thead>
                  <tbody>{(rec.plan.captains ?? []).filter((c) => c.kind !== "unchanged").slice(0, 80).map((c) => (
                    <tr key={c.id}><td>{c.id} {c.name}</td><td className={c.kind === "conflict" ? "warn" : ""}>{KIND[c.kind] ?? c.kind}</td><td className="wrap">{(c.fields ?? []).join(", ")}</td></tr>
                  ))}</tbody></table>
              )}
              {(rec.plan.sections ?? []).length > 0 && (
                <table><thead><tr><th>Раздел</th><th>Добавится</th><th>Изменится</th></tr></thead>
                  <tbody>{(rec.plan.sections ?? []).map((s) => <tr key={s.name}><td>{s.name}</td><td>{s.added}</td><td>{s.changed}</td></tr>)}</tbody></table>
              )}
            </>
          )}
          <div className="row wrap">
            {needAccept && canPublish && (
              <label className="inline"><input type="checkbox" checked={accept} onChange={(e) => setAccept(e.target.checked)} /> принимаю предупреждения</label>
            )}
            <button disabled={!canPublish || (needAccept && !accept)} onClick={() => void publish()}>Подтвердить публикацию</button>
            <button className="secondary" disabled={busy || rec.status === "published" || rec.status === "discarded"} onClick={() => void discard()}>Отклонить пакет</button>
          </div>
        </div>
      )}
    </section>
  );
}
