import { useEffect, useState } from "react";
import { ai, copyText, download, errText, type Session } from "./aiapi";
import { ImportPanel } from "./ImportPanel";

export function ChatGPT() {
  const [total, setTotal] = useState(0);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [msg, setMsg] = useState("");
  const [err, setErr] = useState("");
  const [size, setSize] = useState(25);
  const [index, setIndex] = useState(1);

  function load() {
    ai.dashboard().then((d) => { setTotal(d.captains); setSessions(d.sessions ?? []); }).catch((e) => setErr(errText(e)));
  }
  useEffect(load, []);
  const batches = total > 0 ? Math.ceil(total / size) : 0;

  async function copy(batch: number) {
    setErr(""); setMsg("");
    try {
      const r = await ai.prompt(batch);
      setMsg(copyText(r.prompt) ? `Промпт скопирован (партия ${batch})` : "Не удалось скопировать автоматически: откройте промпт ниже и скопируйте вручную");
      setPromptText(r.prompt);
    } catch (e) { setErr(errText(e)); }
  }
  const [promptText, setPromptText] = useState("");

  async function dl() {
    setErr(""); setMsg("");
    try {
      const n = await download("/ai/zip/export", size === 500 ? { kind: "all" } : { kind: "batch", size, index }, "batch.zip");
      setMsg(`Скачан ${n}. Загрузите его в ChatGPT вместе с промптом.`);
      load();
    } catch (e) { setErr(errText(e)); }
  }

  return (
    <>
      <section className="card" aria-label="Инструкция">
        <h2>Как обучить 500 капитанов через ChatGPT</h2>
        <ol className="steps">
          <li>На вкладке «Обзор» сгенерируйте капитанов (до 300).</li>
          <li>Ниже выберите размер партии (рекомендуется 25 — это 12 партий) и скачайте ZIP партии.</li>
          <li>Нажмите «Скопировать промпт», откройте новый чат ChatGPT, вставьте промпт и приложите скачанный ZIP.</li>
          <li>Дождитесь ответа с файлом <code>trained_pack.zip</code> и скачайте его. Если ChatGPT не вернул файл, попросите: «Собери ZIP по инструкции из раздела 11».</li>
          <li>Загрузите результат ниже: система проверит структуру, схему, SHA-256, факты и тесты, покажет изменения.</li>
          <li>Проверьте предпросмотр и подтвердите публикацию. Версию всегда можно откатить на вкладке «Обзор».</li>
          <li>Переходите к следующей партии. Для каждой партии используйте новый чат, чтобы ChatGPT не терял контекст.</li>
        </ol>
        <p className="muted">В игре ChatGPT и любые языковые модели не используются: результат — готовые данные для движка на правилах.</p>
      </section>

      <section className="card" aria-label="Пакет и промпт">
        <h2>ZIP и промпт для обучения</h2>
        {err && <p role="alert" className="error">{err}</p>}
        {msg && <p className="ok">{msg}</p>}
        <div className="row wrap">
          <label className="inline">Размер партии
            <select value={size} onChange={(e) => { setSize(Number(e.target.value)); setIndex(1); }}>{[10, 25, 50, 300].map((n) => <option key={n} value={n}>{n === 300 ? "300 (полный пакет)" : n}</option>)}</select>
          </label>
          {size !== 300 && (
            <label className="inline">Партия
              <select value={index} onChange={(e) => setIndex(Number(e.target.value))} disabled={batches === 0}>
                {Array.from({ length: batches }, (_, i) => <option key={i} value={i + 1}>{i + 1} из {batches}</option>)}
              </select>
            </label>
          )}
          <button disabled={total === 0} onClick={() => void dl()}>Скачать ZIP для обучения</button>
          <button onClick={() => void copy(size)}>Скопировать промпт обучения ChatGPT</button>
        </div>
        {promptText && <textarea readOnly className="prompt" value={promptText} aria-label="Текст промпта" onFocus={(e) => e.currentTarget.select()} />}
      </section>

      <ImportPanel onChanged={load} />

      <section className="card" aria-label="История обучающих сессий">
        <h2>История обучающих сессий</h2>
        {sessions.length === 0 ? <p className="muted">Выгрузок пока нет.</p> : (
          <table><thead><tr><th>Время</th><th>Пакет</th><th>Капитанов</th><th>SHA-256</th></tr></thead>
            <tbody>{sessions.map((s) => <tr key={s.id}><td>{new Date(s.created_at).toLocaleString("ru-RU")}</td><td>{s.label}</td><td>{s.batch_size}</td><td className="mono">{s.sha256.slice(0, 12)}…</td></tr>)}</tbody></table>
        )}
      </section>
    </>
  );
}
