import { useCallback, useEffect, useState } from "react";
import { ApiError } from "./api";
import { chatApi, type ChatMessage, type ChatMute, type ChatSettings } from "./chat";

const err = (e: unknown) => (e instanceof ApiError ? e.message : "Неизвестная ошибка");
const CHANNELS: Record<string, string> = { "": "Все", general: "Общий", dm: "Личные", system: "Системные" };

export function ChatAdmin({ username, onBack, onLoggedOut }: { username: string; onBack: () => void; onLoggedOut: () => void }) {
  const [msgs, setMsgs] = useState<ChatMessage[]>([]);
  const [mutes, setMutes] = useState<ChatMute[]>([]);
  const [st, setSt] = useState<ChatSettings | null>(null);
  const [channel, setChannel] = useState("");
  const [q, setQ] = useState("");
  const [who, setWho] = useState("");
  const [minutes, setMinutes] = useState(60);
  const [reason, setReason] = useState("");
  const [words, setWords] = useState("");
  const [msg, setMsg] = useState("");
  const [error, setError] = useState("");

  const loadMsgs = useCallback(() => { chatApi.messages(channel, q).then((d) => setMsgs(d.items ?? [])).catch((e) => setError(err(e))); }, [channel, q]);
  const loadMutes = useCallback(() => { chatApi.mutes().then((d) => setMutes(d.items ?? [])).catch((e) => setError(err(e))); }, []);
  useEffect(() => { loadMsgs(); }, [loadMsgs]);
  useEffect(() => { loadMutes(); chatApi.settings().then((s) => { setSt(s); setWords((s.extra_blocked ?? []).join("\n")); }).catch((e) => setError(err(e))); }, [loadMutes]);

  async function mute(m: number) {
    setError(""); setMsg("");
    try { const r = await chatApi.mute(who, m, reason); setMsg(`${r.player_name}: ${m > 0 ? `запрет писать на ${m} мин` : "бессрочный запрет писать"}`); setWho(""); loadMutes(); } catch (e) { setError(err(e)); }
  }
  async function unmute(id: string) { try { await chatApi.unmute(id); loadMutes(); } catch (e) { setError(err(e)); } }
  async function save() {
    if (!st) return;
    setError(""); setMsg("");
    try { const s = await chatApi.saveSettings({ ...st, extra_blocked: words.split("\n").map((w) => w.trim()).filter(Boolean) }); setSt(s); setMsg("Настройки чата сохранены и уже действуют"); } catch (e) { setError(err(e)); }
  }
  const num = (k: keyof ChatSettings) => (e: React.ChangeEvent<HTMLInputElement>) => st && setSt({ ...st, [k]: Number(e.target.value) });
  const chk = (k: keyof ChatSettings) => (e: React.ChangeEvent<HTMLInputElement>) => st && setSt({ ...st, [k]: e.target.checked });

  return (
    <main className="dashboard studio">
      <header>
        <h1>Чат: модерация и настройки</h1>
        <div><span className="muted">{username}</span><button className="secondary" onClick={onBack}>Сервер</button><button onClick={onLoggedOut}>Выйти</button></div>
      </header>
      {error && <p role="alert" className="error">{error}</p>}
      {msg && <p className="ok">{msg}</p>}

      <section className="card" aria-label="Запреты">
        <h2>Запретить писать в чат</h2>
        <div className="row wrap">
          <input aria-label="Игрок (ник или идентификатор)" placeholder="Ник или идентификатор игрока" value={who} onChange={(e) => setWho(e.target.value)} />
          <input aria-label="Срок, минут" type="number" min={1} max={525600} value={minutes} onChange={(e) => setMinutes(Number(e.target.value))} style={{ width: 110 }} />
          <input aria-label="Причина" placeholder="Причина" maxLength={200} value={reason} onChange={(e) => setReason(e.target.value)} />
          <button disabled={!who.trim()} onClick={() => void mute(minutes)}>Заглушить</button>
          <button className="secondary" disabled={!who.trim()} onClick={() => void mute(0)}>Бан в чате (бессрочно)</button>
        </div>
        <table><thead><tr><th>Игрок</th><th>До</th><th>Причина</th><th>Кто</th><th /></tr></thead>
          <tbody>{mutes.length === 0 ? <tr><td colSpan={5} className="muted">Запретов нет</td></tr> : mutes.map((m) => (
            <tr key={m.player_id}><td>{m.player_name}</td><td>{m.until ? new Date(m.until).toLocaleString("ru-RU") : "бессрочно"}</td><td className="wrap">{m.reason}</td><td>{m.admin}</td>
              <td><button className="secondary" onClick={() => void unmute(m.player_id)}>Снять</button></td></tr>))}</tbody></table>
      </section>

      <section className="card" aria-label="Журнал сообщений">
        <h2>Журнал сообщений</h2>
        <div className="row wrap">
          <select aria-label="Канал" value={channel} onChange={(e) => setChannel(e.target.value)}>{Object.entries(CHANNELS).map(([k, v]) => <option key={k} value={k}>{v}</option>)}</select>
          <input aria-label="Поиск" placeholder="Поиск по тексту и именам" value={q} onChange={(e) => setQ(e.target.value)} />
          <button className="secondary" onClick={loadMsgs}>Обновить</button>
        </div>
        <table><thead><tr><th>Время</th><th>Канал</th><th>От</th><th>Кому</th><th>Текст</th></tr></thead>
          <tbody>{msgs.length === 0 ? <tr><td colSpan={5} className="muted">Сообщений нет</td></tr> : msgs.map((m) => (
            <tr key={m.id}><td>{new Date(m.at).toLocaleString("ru-RU")}</td><td>{CHANNELS[m.channel] ?? m.channel}</td><td>{m.from_name}{m.from_ai ? " (ИИ)" : ""}</td><td>{m.to_name ?? ""}</td><td className="wrap">{m.text}</td></tr>))}</tbody></table>
      </section>

      {st && (
        <section className="card" aria-label="Настройки чата">
          <h2>Настройки чата</h2>
          <div className="stack">
            <label className="inline"><input type="checkbox" checked={st.enabled} onChange={chk("enabled")} /> Чат включён</label>
            <label className="inline"><input type="checkbox" checked={st.profanity_filter} onChange={chk("profanity_filter")} /> Фильтр недопустимых слов</label>
            <label className="inline"><input type="checkbox" checked={st.links_allowed} onChange={chk("links_allowed")} /> Разрешить ссылки</label>
            <label>Хранить историю, дней<input type="number" min={1} max={90} value={st.retention_days} onChange={num("retention_days")} /></label>
            <label>Интервал между сообщениями, мс<input type="number" min={0} max={60000} value={st.min_interval_ms} onChange={num("min_interval_ms")} /></label>
            <label>Максимальная длина сообщения<input type="number" min={20} max={500} value={st.max_len} onChange={num("max_len")} /></label>
            <label>Сообщений в истории при входе<input type="number" min={10} max={200} value={st.history_limit} onChange={num("history_limit")} /></label>
            <label>Дополнительные запрещённые слова (по одному в строке)<textarea rows={5} value={words} onChange={(e) => setWords(e.target.value)} /></label>
            <button onClick={() => void save()}>Сохранить</button>
          </div>
        </section>
      )}
    </main>
  );
}
