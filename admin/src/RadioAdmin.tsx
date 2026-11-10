import { useCallback, useEffect, useState } from "react";
import { ApiError } from "./api";
import { KIND_NAMES, formatDuration, radioApi, type MediaTrack, type TrackKind } from "./radio";

const err = (e: unknown) => (e instanceof ApiError ? e.message : "Неизвестная ошибка");
const empty = { title: "", artist: "", kind: "music" as TrackKind, license: "", source: "Google Gemini", ai: true, tags: "" };

export function RadioAdmin({ username, onBack, onLoggedOut }: { username: string; onBack: () => void; onLoggedOut: () => void }) {
  const [items, setItems] = useState<MediaTrack[]>([]);
  const [form, setForm] = useState(empty);
  const [file, setFile] = useState<File | undefined>();
  const [msg, setMsg] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(() => {
    radioApi.list().then((d) => setItems(d.items ?? [])).catch((e) => {
      if (e instanceof ApiError && e.status === 401) onLoggedOut();
      else setError(err(e));
    });
  }, [onLoggedOut]);
  useEffect(() => { load(); }, [load]);

  async function upload() {
    if (!file) { setError("Выберите файл MP3 или OGG"); return; }
    setBusy(true); setError(""); setMsg("");
    try {
      const t = await radioApi.upload(file, form);
      setMsg(`«${t.title}» в библиотеке, ${formatDuration(t.duration_ms)}`);
      setForm(empty);
      setFile(undefined);
      load();
    } catch (e) { setError(err(e)); }
    finally { setBusy(false); }
  }

  async function toggle(t: MediaTrack) {
    setError("");
    try { await radioApi.update(t.id, { ...t, enabled: !t.enabled }); load(); }
    catch (e) { setError(err(e)); }
  }

  async function remove(t: MediaTrack) {
    if (!confirm(`Удалить «${t.title}»?`)) return;
    setError("");
    try { await radioApi.remove(t.id); load(); }
    catch (e) { setError(err(e)); }
  }

  return (
    <main className="dashboard studio">
      <header>
        <h1>Музыка и радио</h1>
        <div>
          <span className="muted">{username}</span>
          <button className="secondary" onClick={onBack}>Назад</button>
        </div>
      </header>
      <p className="muted">Этап М-1: библиотека и фоновая музыка в игре. Эфир, реклама и срочные новости — следующий этап, но виды треков уже можно хранить. Файлы до 12 МБ и 10 минут, MP3 или OGG. Пометка «создано ИИ» по умолчанию включена.</p>
      <p className="muted">Звуки игры (М-3) — это вид «Эффект». Тег решает, когда играть: ui — кнопки, engine — двигатель своего корабля, weapon — свой выстрел, hit — попадание по вам. Если такого включённого файла нет, игра молчит. В APK звуки не вшиваются.</p>
      {error && <p role="alert" className="error">{error}</p>}
      {msg && <p className="ok">{msg}</p>}

      <section className="card" aria-label="Загрузка трека">
        <h2>Добавить трек</h2>
        <div className="grid2">
          <label>Название<input value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} /></label>
          <label>Исполнитель<input value={form.artist} onChange={(e) => setForm({ ...form, artist: e.target.value })} /></label>
          <label>Вид
            <select value={form.kind} onChange={(e) => setForm({ ...form, kind: e.target.value as TrackKind })}>
              {(Object.keys(KIND_NAMES) as TrackKind[]).map((k) => <option key={k} value={k}>{KIND_NAMES[k]}</option>)}
            </select>
          </label>
          <label>Источник<input value={form.source} onChange={(e) => setForm({ ...form, source: e.target.value })} /></label>
          <label>Лицензия<input value={form.license} onChange={(e) => setForm({ ...form, license: e.target.value })} /></label>
          <label>Теги<input value={form.tags} onChange={(e) => setForm({ ...form, tags: e.target.value })} /></label>
        </div>
        <label className="inline"><input type="checkbox" checked={form.ai} onChange={(e) => setForm({ ...form, ai: e.target.checked })} /> Создано ИИ</label>
        <div className="row wrap">
          <input aria-label="Файл трека" type="file" accept=".mp3,.ogg,.opus,audio/mpeg,audio/ogg" onChange={(e) => setFile(e.target.files?.[0])} />
          <button disabled={busy} onClick={() => void upload()}>{busy ? "Загрузка…" : "Загрузить"}</button>
        </div>
      </section>

      <section className="card" aria-label="Библиотека">
        <h2>Библиотека</h2>
        {items.length === 0 ? <p className="muted">Пока пусто. Шесть присланных треков загружаются сюда вручную, в игру они не вшиты.</p> : (
          <table>
            <thead>
              <tr><th>Трек</th><th>Вид</th><th>Время</th><th>ИИ</th><th></th></tr>
            </thead>
            <tbody>
              {items.map((t) => (
                <tr key={t.id}>
                  <td className="wrap">{t.title}{t.artist ? <span className="muted"> — {t.artist}</span> : null}{!t.enabled && <span className="muted"> (выключен)</span>}</td>
                  <td>{KIND_NAMES[t.kind]}</td>
                  <td>{formatDuration(t.duration_ms)}</td>
                  <td>{t.ai_generated ? "да" : "нет"}</td>
                  <td className="actions">
                    <audio controls preload="none" src={radioApi.fileUrl(t.id)} />
                    <button className="small secondary" onClick={() => void toggle(t)}>{t.enabled ? "Выключить" : "Включить"}</button>
                    <button className="small secondary" onClick={() => void remove(t)}>Удалить</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </main>
  );
}
