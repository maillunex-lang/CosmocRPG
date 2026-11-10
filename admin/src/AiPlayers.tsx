import { useCallback, useEffect, useState } from "react";
import { aiPlayersApi, type AiPlayersData, type AiSettings } from "./aiplayers";

// «ИИ-игроки» (ИИ-3): состояние агента мира, настройки (число онлайн, доли поведений, реплики в чате),
// живой статус капитанов и индивидуальное включение/отключение.

const BEHAVIOR_LABEL: Record<string, string> = {
  miner: "Шахтёры",
  trader: "Торговцы",
  scout: "Разведчики",
  patrol: "Патрульные",
  support: "Поддержка",
};

export function AiPlayers({ username, onBack, onLoggedOut }: { username: string; onBack: () => void; onLoggedOut: () => void }) {
  const [data, setData] = useState<AiPlayersData | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [filter, setFilter] = useState("");
  const [onlyOnline, setOnlyOnline] = useState(false);

  const reload = useCallback(() => {
    aiPlayersApi.get().then(setData, (e: Error) => setError(e.message));
  }, []);

  useEffect(() => {
    reload();
    const t = setInterval(reload, 5000); // живой статус
    return () => clearInterval(t);
  }, [reload]);

  if (error && !data) return <p className="muted center">Ошибка: {error}</p>;
  if (!data) return <p className="muted center">Загрузка…</p>;

  const save = (settings: AiSettings) => {
    setNotice("");
    aiPlayersApi.saveSettings(settings).then(
      () => {
        setNotice("Настройки применены");
        reload();
      },
      (e: Error) => setError(e.message),
    );
  };

  const toggleCaptain = (id: string, disable: boolean) => {
    aiPlayersApi.setCaptain(id, disable).then(reload, (e: Error) => setError(e.message));
  };

  const disabled = new Set(data.settings.disabled_captains);
  const captains = data.captains.filter((c) => {
    if (onlyOnline && !c.since) return false;
    const q = filter.trim().toLowerCase();
    if (!q) return true;
    return c.name.toLowerCase().includes(q) || c.id.toLowerCase().includes(q) || c.profession.toLowerCase().includes(q);
  });

  return (
    <div className="page">
      <header className="row">
        <h1>ИИ-игроки</h1>
        <span className="muted">вход: {username}</span>
        <button onClick={onBack}>← Назад</button>
        <button onClick={() => void onLoggedOut()}>Выйти</button>
      </header>
      {error && <p className="error">{error}</p>}
      {notice && <p className="muted">{notice}</p>}

      <section className="cards">
        <div className="card">
          <b>{data.status.online}</b>
          <span>онлайн (цель {data.status.target})</span>
        </div>
        <div className="card">
          <b>{data.status.players}</b>
          <span>живых игроков</span>
        </div>
        <div className="card">
          <b>{data.status.profiles}</b>
          <span>профилей в AI Studio</span>
        </div>
        <div className="card">
          <b>выключены</b>
          <span>старые простые корабли</span>
        </div>
        <div className="card">
          <b>{data.settings.civ_enabled ? "включена" : "выключена"}</b>
          <span>цивилизация, в мире {data.settings.civ_cast}</span>
        </div>
      </section>

      <h2>Поведение онлайн</h2>
      <p className="muted">
        {Object.entries(data.status.by_behavior)
          .map(([k, v]) => `${BEHAVIOR_LABEL[k] ?? k}: ${v}`)
          .join(" · ") || "никого нет онлайн"}
      </p>

      <SettingsForm settings={data.settings} onSave={save} />

      <h2>Капитаны ({captains.length} из {data.captains.length})</h2>
      <div className="row">
        <input placeholder="Поиск: ник, id, профессия" value={filter} onChange={(e) => setFilter(e.target.value)} />
        <label>
          <input type="checkbox" checked={onlyOnline} onChange={(e) => setOnlyOnline(e.target.checked)} /> только онлайн
        </label>
      </div>
      <table className="table">
        <thead>
          <tr>
            <th>Ник</th>
            <th>Профессия</th>
            <th>Поведение</th>
            <th>Статус</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {captains.map((c) => (
            <tr key={c.id} className={disabled.has(c.id) ? "disabled-row" : ""}>
              <td>{c.name}</td>
              <td className="muted">{c.profession}</td>
              <td className="muted">{BEHAVIOR_LABEL[c.behavior] ?? c.behavior}</td>
              <td>{c.since ? "🟢 онлайн" : "—"}</td>
              <td>
                <button onClick={() => toggleCaptain(c.id, !disabled.has(c.id))}>
                  {disabled.has(c.id) ? "Включить" : "Отключить"}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function withCiv(settings: AiSettings): AiSettings {
  return {
    ...settings,
    civ_enabled: settings.civ_enabled ?? true,
    civ_cast: settings.civ_cast || 3,
    civ_chat: settings.civ_chat ?? true,
    civ_talk_min: settings.civ_talk_min || 3,
    civ_talk_max: settings.civ_talk_max || 15,
    civ_interval_sec: settings.civ_interval_sec || 12,
    civ_retreat_pct: settings.civ_retreat_pct || 30,
    civ_help: settings.civ_help ?? true,
  };
}

function SettingsForm({ settings, onSave }: { settings: AiSettings; onSave: (s: AiSettings) => void }) {
  const [s, setS] = useState<AiSettings>(withCiv(settings));
  useEffect(() => setS(withCiv(settings)), [settings]);
  const num = (key: keyof AiSettings, min: number, max: number) => (e: React.ChangeEvent<HTMLInputElement>) => {
    const v = Math.max(min, Math.min(max, Number(e.target.value) || 0));
    setS({ ...s, [key]: v });
  };
  return (
    <section>
      <h2>Настройки</h2>
      <p className="muted">Старые простые корабли и их шаблонные реплики выключены. В мире только цивилизация.</p>
      <h3>Цивилизация (шлюз)</h3>
      <p className="muted">Настоящие капитаны в мире. Меняется сразу, перезапуск сервера не нужен.</p>
      <div className="row wrap">
        <label>
          <input type="checkbox" checked={s.civ_enabled} onChange={(e) => setS({ ...s, civ_enabled: e.target.checked })} /> цивилизация включена
        </label>
        <label>
          <input type="checkbox" checked={s.civ_chat} onChange={(e) => setS({ ...s, civ_chat: e.target.checked })} /> чат
        </label>
        <label>
          <input type="checkbox" checked={s.civ_help} onChange={(e) => setS({ ...s, civ_help: e.target.checked })} /> помощь из чата (реально летят)
        </label>
      </div>
      <div className="row wrap">
        <label>Капитанов в мире, 1–500 <input type="number" value={s.civ_cast} onChange={num("civ_cast", 1, 500)} /></label>
        <label>Реплик в разговоре от <input type="number" value={s.civ_talk_min} onChange={num("civ_talk_min", 1, 30)} /></label>
        <label>до <input type="number" value={s.civ_talk_max} onChange={num("civ_talk_max", 1, 30)} /></label>
        <label>Пауза между репликами, с <input type="number" value={s.civ_interval_sec} onChange={num("civ_interval_sec", 4, 120)} /></label>
        <label>Отступление при корпусе, % <input type="number" value={s.civ_retreat_pct} onChange={num("civ_retreat_pct", 5, 90)} /></label>
      </div>
      <button className="primary" onClick={() => onSave(s)}>Сохранить настройки</button>
    </section>
  );
}
