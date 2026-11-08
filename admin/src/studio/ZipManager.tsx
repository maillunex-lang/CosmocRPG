import { useEffect, useState } from "react";
import { ai, download, errText } from "./aiapi";
import { ImportPanel } from "./ImportPanel";

export function ZipManager() {
  const [total, setTotal] = useState(0);
  const [size, setSize] = useState(25);
  const [index, setIndex] = useState(1);
  const [ids, setIds] = useState("");
  const [msg, setMsg] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const batches = total > 0 ? Math.ceil(total / size) : 0;

  useEffect(() => {
    ai.dashboard().then((d) => setTotal(d.captains)).catch((e) => setErr(errText(e)));
  }, []);
  useEffect(() => { if (index > batches && batches > 0) setIndex(1); }, [batches, index]);

  async function get(body: Record<string, unknown>, fallback: string) {
    setBusy(true); setErr(""); setMsg("");
    try {
      const name = await download("/ai/zip/export", body, fallback);
      setMsg(`Скачан файл ${name}`);
    } catch (e) { setErr(errText(e)); } finally { setBusy(false); }
  }

  return (
    <>
      <section className="card" aria-label="Выгрузка пакетов">
        <h2>Скачать ZIP</h2>
        {err && <p role="alert" className="error">{err}</p>}
        {msg && <p className="ok">{msg}</p>}
        <div className="row wrap">
          <button disabled={busy} onClick={() => void get({ kind: "template" }, "template.zip")}>Пустой шаблон</button>
          <button disabled={busy} onClick={() => void get({ kind: "examples" }, "examples.zip")}>Шаблон с примерами</button>
          <button disabled={busy || total === 0} onClick={() => void get({ kind: "all" }, "full.zip")}>Полный пакет ({total})</button>
        </div>
        <h3>Партия для обучения</h3>
        <div className="row wrap">
          <label className="inline">Размер партии
            <select value={size} onChange={(e) => setSize(Number(e.target.value))}>{[10, 25, 50].map((n) => <option key={n} value={n}>{n}</option>)}</select>
          </label>
          <label className="inline">Партия
            <select value={index} onChange={(e) => setIndex(Number(e.target.value))} disabled={batches === 0}>
              {Array.from({ length: batches }, (_, i) => <option key={i} value={i + 1}>{i + 1} из {batches}</option>)}
            </select>
          </label>
          <button disabled={busy || batches === 0} onClick={() => void get({ kind: "batch", size, index }, "batch.zip")}>Скачать партию</button>
        </div>
        {total === 0 && <p className="muted">В реестре нет капитанов: сначала сгенерируйте их на вкладке «Обзор».</p>}
        <h3>Выбранные капитаны</h3>
        <div className="row wrap">
          <input value={ids} onChange={(e) => setIds(e.target.value)} placeholder="cpt-0001, cpt-0007, cpt-0042" aria-label="Идентификаторы капитанов" style={{ flex: 1, minWidth: 240 }} />
          <button disabled={busy || ids.trim() === ""} onClick={() => void get({ kind: "selected", ids: ids.split(/[\s,;]+/).filter(Boolean) }, "selected.zip")}>Скачать выбранных</button>
        </div>
        <p className="muted">В каждый пакет входят профили, словари, факты, схемы, тесты и README_FOR_CHATGPT.md с мастер-промптом.</p>
      </section>
      <ImportPanel />
    </>
  );
}
