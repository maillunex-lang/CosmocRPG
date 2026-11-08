import { useCallback, useEffect, useState } from "react";
import { ApiError } from "./api";
import { copyText } from "./studio/aiapi";
import { shipsApi, type Preview, type ShipSpec, type VersionInfo } from "./ships";

const err = (e: unknown) => (e instanceof ApiError ? e.message : "Неизвестная ошибка");

export function ShipsAdmin({ username, onBack, onLoggedOut }: { username: string; onBack: () => void; onLoggedOut: () => void }) {
  const [ships, setShips] = useState<ShipSpec[]>([]);
  const [versions, setVersions] = useState<VersionInfo[]>([]);
  const [version, setVersion] = useState(0);
  const [starters, setStarters] = useState<Record<string, { ship: string; spec: string; skin: string }>>({});
  const [sel, setSel] = useState<string>("");
  const [text, setText] = useState("");
  const [msg, setMsg] = useState("");
  const [error, setError] = useState("");
  const [preview, setPreview] = useState<Preview | null>(null);
  const [prompt, setPrompt] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(() => {
    shipsApi.list().then((d) => { setShips(d.ships); setVersions(d.versions ?? []); setVersion(d.version); setStarters(d.starters); }).catch((e) => setError(err(e)));
  }, []);
  useEffect(load, [load]);
  useEffect(() => {
    const s = ships.find((x) => x.id === sel);
    setText(s ? JSON.stringify(s, null, 2) : "");
  }, [sel, ships]);

  async function save() {
    setError(""); setMsg("");
    let s: ShipSpec;
    try { s = JSON.parse(text) as ShipSpec; } catch { setError("Это не корректный JSON"); return; }
    setBusy(true);
    try { const r = await shipsApi.save(s); setMsg(`Сохранено, версия ${r.version}. Изменения уже применены в игре.`); load(); } catch (e) { setError(err(e)); } finally { setBusy(false); }
  }

  async function exportPatch(all: boolean) {
    setError(""); setMsg("");
    try { const n = await shipsApi.exportPatch(all || !sel ? [] : [sel]); setMsg(`Скачан ${n}. Передайте его в ChatGPT вместе с промптом.`); } catch (e) { setError(err(e)); }
  }

  async function upload(f: File | undefined) {
    if (!f) return;
    setError(""); setMsg(""); setBusy(true);
    try { setPreview(await shipsApi.importPatch(f)); } catch (e) { setError(err(e)); } finally { setBusy(false); }
  }

  async function publish() {
    if (!preview) return;
    setBusy(true); setError("");
    try { const r = await shipsApi.publish(preview.id); setMsg(`Патч опубликован, версия ${r.version}`); setPreview({ ...preview, status: "published" }); load(); } catch (e) { setError(err(e)); } finally { setBusy(false); }
  }

  async function rollback(id: number) {
    if (!window.confirm(`Откатить каталог кораблей к версии ${id}? Будет создана новая версия.`)) return;
    try { await shipsApi.rollback(id); setMsg(`Откат к версии ${id} выполнен`); load(); } catch (e) { setError(err(e)); }
  }

  async function copyPrompt() {
    try {
      const r = await shipsApi.prompt();
      setPrompt(r.prompt);
      setMsg(copyText(r.prompt) ? "Промпт скопирован" : "Скопируйте промпт вручную из поля ниже");
    } catch (e) { setError(err(e)); }
  }

  return (
    <main className="dashboard studio">
      <header>
        <h1>Корабли: каталог и параметры</h1>
        <div><span className="muted">{username}</span><button className="secondary" onClick={onBack}>Сервер</button><button onClick={onLoggedOut}>Выйти</button></div>
      </header>
      {error && <p role="alert" className="error">{error}</p>}
      {msg && <p className="ok">{msg}</p>}

      <section className="card" aria-label="Каталог кораблей">
        <h2>Каталог (версия {version})</h2>
        <table>
          <thead><tr><th>ID</th><th>Название</th><th>Класс</th><th>Корпус</th><th>Щит</th><th>Броня</th><th>Энергия</th><th>Груз</th><th>Скорость</th><th /></tr></thead>
          <tbody>{ships.map((s) => (
            <tr key={s.id} className={sel === s.id ? "selected" : ""} onClick={() => setSel(s.id)} tabIndex={0} onKeyDown={(e) => { if (e.key === "Enter") setSel(s.id); }}>
              <td>{s.id}</td><td>{s.name}</td><td>{s.class}</td><td>{s.hull}</td><td>{s.shield}</td><td>{s.armor}</td><td>{s.energy}</td><td>{s.cargo}</td><td>{s.speed}</td>
              <td>{s.temporary ? <span className="warn">временная модель</span> : ""}</td>
            </tr>))}</tbody>
        </table>
        <h3>Стартовые корабли направлений пилота</h3>
        <p className="muted">{Object.entries(starters).map(([k, v]) => `${{ interceptor: "Перехватчик", scout: "Разведчик", trader: "Торговец", miner: "Шахтёр" }[k] ?? k} → ${v.ship} (${v.spec}, облик ${v.skin})`).join("; ")}</p>
      </section>

      {sel && (
        <section className="card" aria-label={`Редактор ${sel}`}>
          <h2>Параметры корабля {sel}</h2>
          <p className="muted">Значения проверяются по диапазонам. После сохранения новые параметры сразу применяются к кораблям в игре (текущий запас прочности ограничивается новыми максимумами). Версию можно откатить.</p>
          <textarea className="json" value={text} onChange={(e) => setText(e.target.value)} spellCheck={false} aria-label="Паспорт корабля (JSON)" />
          <div className="row wrap">
            <button disabled={busy} onClick={() => void save()}>Проверить и сохранить</button>
            <button className="secondary" disabled={busy} onClick={() => setSel(sel + "")}>Отменить правки</button>
          </div>
        </section>
      )}

      <section className="card" aria-label="ZIP-патчи">
        <h2>ZIP-патч параметров (через ChatGPT)</h2>
        <ol className="steps">
          <li>Скачайте патч (все корабли или выбранный) и скопируйте промпт.</li>
          <li>В ChatGPT приложите ZIP, вставьте промпт и опишите, что изменить (например: «увеличь скорость Стрижа на 10%»).</li>
          <li>Загрузите полученный ZIP ниже: система проверит его, покажет изменения параметров и применит только после подтверждения.</li>
        </ol>
        <div className="row wrap">
          <button disabled={busy} onClick={() => void exportPatch(true)}>Скачать патч: все корабли</button>
          <button className="secondary" disabled={busy || !sel} onClick={() => void exportPatch(false)}>Только {sel || "выбранный"}</button>
          <button className="secondary" onClick={() => void copyPrompt()}>Скопировать промпт ChatGPT</button>
        </div>
        {prompt && <textarea readOnly className="prompt" value={prompt} aria-label="Текст промпта" onFocus={(e) => e.currentTarget.select()} />}
        <div className="row wrap">
          <input type="file" accept=".zip,application/zip" aria-label="ZIP-патч" onChange={(e) => void upload(e.target.files?.[0])} />
        </div>
        {preview && (
          <div className="report">
            <h3>Проверка патча: {{ ready: "готов к публикации", rejected: "отклонён", published: "опубликован", discarded: "отменён" }[preview.status] ?? preview.status}</h3>
            <dl>
              <dt>Пакет</dt><dd>{preview.pack_id || "—"}, файл {preview.filename}</dd>
              <dt>SHA-256</dt><dd className="mono">{preview.sha256}</dd>
              <dt>Целостность</dt>
              <dd className={preview.integrity === "verified" ? "ok" : preview.integrity === "failed" ? "error" : "warn"}>
                {{ verified: "подтверждена", partial: "частично", none: "не подтверждена (нет сумм)", failed: "НАРУШЕНА", "": "—" }[preview.integrity] ?? preview.integrity}
              </dd>
            </dl>
            {preview.errors.length > 0 && (
              <><h4 className="error">Ошибки ({preview.errors.length})</h4>
                <table><thead><tr><th>Файл</th><th>Код</th><th>Сообщение</th></tr></thead>
                  <tbody>{preview.errors.slice(0, 50).map((i, k) => <tr key={k}><td>{i.path}</td><td>{i.code}</td><td className="wrap">{i.message}</td></tr>)}</tbody></table></>
            )}
            {preview.warnings.length > 0 && (
              <><h4 className="warn">Предупреждения ({preview.warnings.length})</h4>
                <ul>{preview.warnings.map((w, k) => <li key={k} className="wrap">{w.path}: {w.message}</li>)}</ul></>
            )}
            {preview.errors.length === 0 && (
              <>
                <h4>Изменения параметров</h4>
                {preview.changes.map((c) => (
                  <div key={c.id}>
                    <strong>{c.id}</strong> — {{ new: "новый корабль", changed: "изменяется", unchanged: "без изменений" }[c.kind] ?? c.kind}
                    {c.fields.length > 0 && (
                      <table><thead><tr><th>Параметр</th><th>Было</th><th>Станет</th></tr></thead>
                        <tbody>{c.fields.map((f) => <tr key={f.path}><td>{f.path}</td><td>{f.old}</td><td>{f.new}</td></tr>)}</tbody></table>
                    )}
                  </div>
                ))}
              </>
            )}
            <div className="row wrap">
              <button disabled={busy || preview.status !== "ready"} onClick={() => void publish()}>Подтвердить публикацию</button>
              <button className="secondary" disabled={busy || preview.status === "published"} onClick={() => void shipsApi.discard(preview.id).then(() => setPreview({ ...preview, status: "discarded" })).catch((e) => setError(err(e)))}>Отклонить</button>
            </div>
          </div>
        )}
      </section>

      <section className="card" aria-label="Версии каталога">
        <h2>Версии каталога</h2>
        <table><thead><tr><th>№</th><th>Время</th><th>Кто</th><th>Что</th><th>Кораблей</th><th /></tr></thead>
          <tbody>{versions.map((v) => (
            <tr key={v.id}><td>{v.id}{v.active ? " ●" : ""}</td><td>{new Date(v.created_at).toLocaleString("ru-RU")}</td><td>{v.admin}</td><td className="wrap">{v.note}</td><td>{v.ships}</td>
              <td>{!v.active && <button className="secondary" onClick={() => void rollback(v.id)}>Откатить</button>}</td></tr>))}</tbody></table>
      </section>
    </main>
  );
}
