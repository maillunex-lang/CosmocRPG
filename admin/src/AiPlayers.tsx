import { useCallback, useEffect, useState } from "react";
import { aiPlayersApi, type AiCaptain, type AiPlayersData, type AiSettings } from "./aiplayers";

// Страница «ИИ-игроки»: цель цивилизации, кто реально в мире, и тонкие настройки разговоров.

const BEHAVIOR_LABEL: Record<string, string> = {
  miner: "Шахтёры",
  trader: "Торговцы",
  scout: "Разведчики",
  patrol: "Патрульные",
  support: "Поддержка",
};

function inWorld(since: string) {
  const t = Date.parse(since || "");
  return Number.isFinite(t) && t > Date.UTC(2000, 0, 1);
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

export function AiPlayers({ username, onBack, onLoggedOut }: { username: string; onBack: () => void; onLoggedOut: () => void }) {
  const [data, setData] = useState<AiPlayersData | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [filter, setFilter] = useState("");
  const [draft, setDraft] = useState<AiSettings | null>(null);
  const [saving, setSaving] = useState(false);

  const reload = useCallback(() => {
    aiPlayersApi.get().then(
      (next) => {
        setData(next);
        setError("");
      },
      (e: Error) => setError(e.message),
    );
  }, []);

  useEffect(() => {
    reload();
    const t = setInterval(reload, 5000);
    return () => clearInterval(t);
  }, [reload]);

  useEffect(() => {
    if (data) setDraft(withCiv(data.settings));
  }, [data]);

  if (error && !data) return <p className="muted center">Ошибка: {error}</p>;
  if (!data || !draft) return <p className="muted center">Загрузка…</p>;

  const save = (settings: AiSettings) => {
    setNotice("");
    setSaving(true);
    aiPlayersApi.saveSettings(settings).then(
      () => {
        setNotice("Число применено. Капитаны зайдут в разные точки и займутся своим контрактом.");
        setSaving(false);
        reload();
      },
      (e: Error) => {
        setSaving(false);
        setError(e.message);
      },
    );
  };

  const toggleCaptain = (id: string, disable: boolean) => {
    aiPlayersApi.setCaptain(id, disable).then(reload, (e: Error) => setError(e.message));
  };

  const disabled = new Set(data.settings.disabled_captains ?? []);
  const online = data.captains.filter((c) => inWorld(c.since) && !disabled.has(c.id));
  const q = filter.trim().toLowerCase();
  const roster = data.captains.filter((c) => {
    if (!q) return true;
    return [c.name, c.id, c.profession].some((v) => (v || "").toLowerCase().includes(q));
  });
  const behaviors = Object.entries(data.status.by_behavior ?? {}).filter(([, n]) => n > 0);

  return (
    <div className="ai-page">
      <header className="ai-top">
        <div>
          <p className="ai-kicker">Админка</p>
          <h1>ИИ-игроки</h1>
        </div>
        <div className="ai-top-actions">
          <span className="ai-user">{username}</span>
          <button type="button" className="glass-btn" onClick={onBack}>Назад</button>
          <button type="button" className="glass-btn" onClick={() => void onLoggedOut()}>Выйти</button>
        </div>
      </header>

      {error && <p className="ai-banner bad">{error}</p>}
      {notice && <p className="ai-banner ok">{notice}</p>}
      {data.civ_error && <p className="ai-banner bad">Не удалось выпустить: {data.civ_error}</p>}

      <section className="glass hero" aria-label="Сколько ИИ в мире">
        <div className="hero-now">
          <strong>{data.status.online}</strong>
          <span>сейчас в мире</span>
        </div>
        <div className="hero-target">
          <label htmlFor="ai-cast">Сколько ИИ должно быть</label>
          <div className="target-row">
            <input
              id="ai-cast"
              type="range"
              min={1}
              max={500}
              value={draft.civ_cast}
              onChange={(e) => setDraft({ ...draft, civ_cast: Number(e.target.value) })}
            />
            <input
              aria-label="Число ИИ"
              type="number"
              min={1}
              max={500}
              value={draft.civ_cast}
              onChange={(e) => setDraft({ ...draft, civ_cast: Math.max(1, Math.min(500, Number(e.target.value) || 1)) })}
            />
          </div>
          <p>Цель сейчас на сервере: {data.status.target}. Живых игроков: {data.status.players}. Профилей: {data.status.profiles}.</p>
          <p>Капитан заходит только в своё время по Уфе и играет всю смену, а не минуту. Жаворонки утром, дневные с 11 до 18, вечерние после 17:00. Ночные с 22:00 до 7:00 и ещё днём в субботу и воскресенье. Число сверху — потолок, не обязательная толпа. В игре их видно в той системе, где они сейчас.</p>
          <label className="check">
            <input type="checkbox" checked={draft.civ_enabled} onChange={(e) => setDraft({ ...draft, civ_enabled: e.target.checked })} />
            Выпускать капитанов в мир
          </label>
          <button type="button" className="primary" disabled={saving} onClick={() => save(draft)}>
            {saving ? "Применяю…" : "Применить"}
          </button>
        </div>
      </section>

      {behaviors.length > 0 && (
        <div className="chips" aria-label="Кто онлайн по ролям">
          {behaviors.map(([k, n]) => (
            <span key={k} className="chip">{BEHAVIOR_LABEL[k] ?? k} {n}</span>
          ))}
        </div>
      )}

      <section className="glass" aria-label="Кто онлайн">
        <h2>Кто онлайн</h2>
        {online.length === 0 ? (
          <p className="ai-empty">В мире никого нет. Поставьте число и нажмите «Применить» — капитаны зайдут через несколько секунд.</p>
        ) : (
          <ul className="online-list">
            {online.map((c) => (
              <OnlineRow key={c.id} captain={c} onToggle={() => toggleCaptain(c.id, true)} />
            ))}
          </ul>
        )}
      </section>

      <details className="glass">
        <summary>Разговоры и отступление</summary>
        <div className="fine">
          <label className="check">
            <input type="checkbox" checked={draft.civ_chat} onChange={(e) => setDraft({ ...draft, civ_chat: e.target.checked })} /> говорят в чат
          </label>
          <label className="check">
            <input type="checkbox" checked={draft.civ_help} onChange={(e) => setDraft({ ...draft, civ_help: e.target.checked })} /> летят на помощь из чата
          </label>
          <label>Реплик в разговоре, от
            <input type="number" min={1} max={30} value={draft.civ_talk_min} onChange={(e) => setDraft({ ...draft, civ_talk_min: Number(e.target.value) || 1 })} />
          </label>
          <label>до
            <input type="number" min={1} max={30} value={draft.civ_talk_max} onChange={(e) => setDraft({ ...draft, civ_talk_max: Number(e.target.value) || 1 })} />
          </label>
          <label>Пауза между репликами, секунды
            <input type="number" min={4} max={120} value={draft.civ_interval_sec} onChange={(e) => setDraft({ ...draft, civ_interval_sec: Number(e.target.value) || 4 })} />
          </label>
          <label>Отступление при корпусе, %
            <input type="number" min={5} max={90} value={draft.civ_retreat_pct} onChange={(e) => setDraft({ ...draft, civ_retreat_pct: Number(e.target.value) || 5 })} />
          </label>
        </div>
      </details>

      <details className="glass">
        <summary>Все капитаны ({data.captains.length})</summary>
        <div className="ai-search">
          <input placeholder="Поиск: ник, id, профессия" value={filter} onChange={(e) => setFilter(e.target.value)} />
        </div>
        <ul className="online-list">
          {roster.map((c) => (
            <li key={c.id} className={disabled.has(c.id) ? "is-off" : ""}>
              <div>
                <b>{c.name || c.id}</b>
                <span>{c.profession || "без профессии"} · {BEHAVIOR_LABEL[c.behavior] ?? c.behavior} · {inWorld(c.since) ? "в мире" : "не в мире"}</span>
              </div>
              <button type="button" className="glass-btn" onClick={() => toggleCaptain(c.id, !disabled.has(c.id))}>
                {disabled.has(c.id) ? "Включить" : "Отключить"}
              </button>
            </li>
          ))}
        </ul>
      </details>
    </div>
  );
}

function OnlineRow({ captain, onToggle }: { captain: AiCaptain; onToggle: () => void }) {
  return (
    <li>
      <div>
        <b>{captain.name || captain.id}</b>
        <span>{captain.profession || "капитан"} · {BEHAVIOR_LABEL[captain.behavior] ?? captain.behavior}</span>
      </div>
      <button type="button" className="glass-btn" onClick={onToggle}>Убрать из мира</button>
    </li>
  );
}
