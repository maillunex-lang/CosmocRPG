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
          <b>{data.status.enabled ? (data.status.paused ? "пауза" : "включены") : "выключены"}</b>
          <span>состояние</span>
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

function SettingsForm({ settings, onSave }: { settings: AiSettings; onSave: (s: AiSettings) => void }) {
  const [s, setS] = useState<AiSettings>(settings);
  useEffect(() => setS(settings), [settings]);
  const num = (key: keyof AiSettings, min: number, max: number) => (e: React.ChangeEvent<HTMLInputElement>) => {
    const v = Math.max(min, Math.min(max, Number(e.target.value) || 0));
    setS({ ...s, [key]: v });
  };
  const share = (k: string) => (e: React.ChangeEvent<HTMLInputElement>) => {
    setS({ ...s, shares: { ...s.shares, [k]: Math.max(0, Math.min(100, Number(e.target.value) || 0)) } });
  };
  return (
    <section>
      <h2>Настройки</h2>
      <div className="row wrap">
        <label>
          <input type="checkbox" checked={s.enabled} onChange={(e) => setS({ ...s, enabled: e.target.checked })} /> ИИ включены
        </label>
        <label>
          <input type="checkbox" checked={s.paused} onChange={(e) => setS({ ...s, paused: e.target.checked })} /> пауза (число не меняется)
        </label>
      </div>
      <div className="row wrap">
        <label>Мин. онлайн <input type="number" value={s.min_online} onChange={num("min_online", 0, 200)} /></label>
        <label>Макс. онлайн <input type="number" value={s.max_online} onChange={num("max_online", 0, 200)} /></label>
        <label>Потолок <input type="number" value={s.abs_max} onChange={num("abs_max", 0, 200)} /></label>
        <label>Игроков для макс. <input type="number" value={s.players_for_max} onChange={num("players_for_max", 1, 50)} /></label>
      </div>
      <div className="row wrap">
        <label>Набор/мин <input type="number" value={s.spawn_per_min} onChange={num("spawn_per_min", 1, 600)} /></label>
        <label>Снятие/мин <input type="number" value={s.despawn_per_min} onChange={num("despawn_per_min", 1, 600)} /></label>
        <label>Задержка снятия, с <input type="number" value={s.empty_grace_sec} onChange={num("empty_grace_sec", 0, 3600)} /></label>
        <label>Радиус видимости <input type="number" value={s.view_radius} onChange={num("view_radius", 100, 2000)} /></label>
      </div>
      <h3>Доли поведений, %</h3>
      <div className="row wrap">
        {Object.keys(BEHAVIOR_LABEL).map((k) => (
          <label key={k}>
            {BEHAVIOR_LABEL[k]} <input type="number" value={s.shares[k] ?? 0} onChange={share(k)} />
          </label>
        ))}
      </div>
      <h3>Реплики в чате (ИИ-2)</h3>
      <div className="row wrap">
        <label>
          <input type="checkbox" checked={s.chat_reply_enabled} onChange={(e) => setS({ ...s, chat_reply_enabled: e.target.checked })} /> отвечают в чате
        </label>
        <label>Вероятность в общем, % <input type="number" value={s.chat_reply_chance} onChange={num("chat_reply_chance", 0, 100)} /></label>
        <label>Задержка от, мс <input type="number" value={s.chat_delay_min_ms} onChange={num("chat_delay_min_ms", 0, 30000)} /></label>
        <label>до, мс <input type="number" value={s.chat_delay_max_ms} onChange={num("chat_delay_max_ms", 0, 30000)} /></label>
      </div>
      <div className="row wrap">
        <label>Кулдаун капитана, с <input type="number" value={s.chat_captain_cooldown_sec} onChange={num("chat_captain_cooldown_sec", 0, 3600)} /></label>
        <label>Потолок реплик/мин <input type="number" value={s.chat_max_per_min} onChange={num("chat_max_per_min", 0, 120)} /></label>
      </div>
      <button className="primary" onClick={() => onSave(s)}>Сохранить настройки</button>
    </section>
  );
}
