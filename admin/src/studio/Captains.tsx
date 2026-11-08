import { useCallback, useEffect, useState } from "react";
import { ai, downloadGet, errText, type Captain, type CaptainSummary } from "./aiapi";

type Meta = Awaited<ReturnType<typeof ai.captains>>;
const lines = (s: string) => s.split("\n").map((x) => x.trim()).filter(Boolean);

export function Captains() {
  const [q, setQ] = useState("");
  const [faction, setFaction] = useState("");
  const [profession, setProfession] = useState("");
  const [source, setSource] = useState("");
  const [formality, setFormality] = useState("");
  const [offset, setOffset] = useState(0);
  const [data, setData] = useState<Meta | null>(null);
  const [sel, setSel] = useState<string | null>(null);
  const [err, setErr] = useState("");
  const [uniq, setUniq] = useState<string>("");
  const limit = 25;

  const load = useCallback(() => {
    const p = new URLSearchParams({ limit: String(limit), offset: String(offset) });
    if (q) p.set("q", q);
    if (faction) p.set("faction", faction);
    if (profession) p.set("profession", profession);
    if (source) p.set("source", source);
    if (formality) p.set("formality", formality);
    ai.captains(p).then((d) => { setData(d); setErr(""); }).catch((e) => setErr(errText(e)));
  }, [q, faction, profession, source, formality, offset]);
  useEffect(load, [load]);

  async function checkUniq() {
    try {
      const r = await ai.uniqueness();
      setUniq(`Капитанов: ${r.captains}. Повторов имён, биографий и характеров: ${r.duplicates.length}. Максимальная схожесть пары: ${Math.round(r.max_similarity * 100)}%.` +
        (r.duplicates.length ? ` ${r.duplicates.slice(0, 5).join("; ")}` : "") +
        (r.similar.length ? ` Самые похожие: ${r.similar.slice(0, 3).map((s) => `${s.a}/${s.b} ${Math.round(s.score * 100)}%`).join(", ")}.` : ""));
    } catch (e) { setUniq(errText(e)); }
  }

  const reset = (f: () => void) => { f(); setOffset(0); };
  const facName = (id: string) => data?.factions.find((f) => f.id === id)?.name ?? id;
  return (
    <>
      <section className="card" aria-label="Реестр капитанов">
        <h2>Реестр капитанов{data ? ` (${data.total})` : ""}</h2>
        {err && <p role="alert" className="error">{err}</p>}
        <div className="row wrap">
          <input placeholder="Поиск: имя, позывной, id, биография" value={q} onChange={(e) => reset(() => setQ(e.target.value))} aria-label="Поиск" style={{ flex: 1, minWidth: 220 }} />
          <select value={faction} onChange={(e) => reset(() => setFaction(e.target.value))} aria-label="Фракция">
            <option value="">все фракции</option>{data?.factions.map((f) => <option key={f.id} value={f.id}>{f.name}</option>)}
          </select>
          <select value={profession} onChange={(e) => reset(() => setProfession(e.target.value))} aria-label="Профессия">
            <option value="">все профессии</option>{data?.professions.map((p) => <option key={p} value={p}>{p}</option>)}
          </select>
          <select value={source} onChange={(e) => reset(() => setSource(e.target.value))} aria-label="Источник">
            <option value="">любой источник</option><option value="generator">генератор</option><option value="chatgpt">ChatGPT</option><option value="manual">вручную</option>
          </select>
          <select value={formality} onChange={(e) => reset(() => setFormality(e.target.value))} aria-label="Формальность">
            <option value="">любая речь</option><option value="formal">формальная</option><option value="neutral">обычная</option><option value="casual">разговорная</option>
          </select>
          <button className="secondary" onClick={() => void checkUniq()}>Проверить уникальность</button>
        </div>
        {uniq && <p className="muted" role="status">{uniq}</p>}
        <table><thead><tr><th>ID</th><th>Имя</th><th>Позывной</th><th>Профессия</th><th>Фракция</th><th>Речь</th><th>Источник</th></tr></thead>
          <tbody>{(data?.items ?? []).map((c: CaptainSummary) => (
            <tr key={c.id} className={sel === c.id ? "selected" : ""} onClick={() => setSel(c.id)} tabIndex={0} onKeyDown={(e) => { if (e.key === "Enter") setSel(c.id); }}>
              <td>{c.id}</td><td>{c.name}</td><td>{c.call_sign || "—"}</td><td>{c.profession}</td><td>{facName(c.faction)}</td><td>{c.formality}/{c.humor}</td><td>{c.manual ? "вручную" : c.source}</td>
            </tr>))}</tbody></table>
        <div className="row">
          <button className="secondary" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - limit))}>Назад</button>
          <span className="muted">{data ? `${Math.min(offset + 1, data.total)}–${Math.min(offset + limit, data.total)} из ${data.total}` : ""}</span>
          <button className="secondary" disabled={!data || offset + limit >= data.total} onClick={() => setOffset(offset + limit)}>Далее</button>
        </div>
        <ImportCaptain onDone={load} />
      </section>
      {sel && data && <CaptainEditor id={sel} meta={data} onSaved={load} onClose={() => setSel(null)} />}
    </>
  );
}

function ImportCaptain({ onDone }: { onDone: () => void }) {
  const [msg, setMsg] = useState("");
  const [err, setErr] = useState("");
  async function pick(f: File | undefined) {
    if (!f) return;
    setMsg(""); setErr("");
    try {
      const obj = JSON.parse(await f.text()) as { id?: string };
      try {
        await ai.importCaptain(obj, false);
      } catch (e) {
        if (!window.confirm(`${errText(e)}\nЗаменить профиль ${obj.id ?? ""}?`)) return;
        await ai.importCaptain(obj, true);
      }
      setMsg("Капитан импортирован (помечен как правленный вручную)");
      onDone();
    } catch (e) { setErr(e instanceof SyntaxError ? "Файл не является JSON" : errText(e)); }
  }
  return (
    <div className="row wrap">
      <label className="inline">Импорт капитана из JSON <input type="file" accept=".json,application/json" onChange={(e) => void pick(e.target.files?.[0])} /></label>
      {msg && <span className="ok">{msg}</span>}{err && <span className="error" role="alert">{err}</span>}
    </div>
  );
}

function CaptainEditor({ id, meta, onSaved, onClose }: { id: string; meta: Meta; onSaved: () => void; onClose: () => void }) {
  const [c, setC] = useState<Captain | null>(null);
  const [err, setErr] = useState("");
  const [msg, setMsg] = useState("");
  useEffect(() => { setC(null); setMsg(""); setErr(""); ai.captain(id).then(setC).catch((e) => setErr(errText(e))); }, [id]);
  if (!c) return <section className="card"><p className={err ? "error" : "muted"}>{err || "Загрузка…"}</p></section>;
  const set = (patch: Partial<Captain>) => setC({ ...c, ...patch });
  const sp = (patch: Partial<Captain["speech"]>) => setC({ ...c, speech: { ...c.speech, ...patch } });
  const corps = meta.corporations.filter((k) => k.faction === c.faction);

  async function save() {
    if (!c) return;
    setErr(""); setMsg("");
    try { const r = await ai.saveCaptain(c); setMsg(`Сохранено, версия ${r.version}`); onSaved(); } catch (e) { setErr(errText(e)); }
  }
  return (
    <section className="card" aria-label={`Карточка ${c.id}`}>
      <div className="row"><h2 style={{ flex: 1 }}>{c.id}: {c.name}{c.call_sign ? ` «${c.call_sign}»` : ""} <span className="muted">ревизия {c.revision}, {c.meta.manual ? "правка вручную" : c.meta.source}</span></h2>
        <button className="secondary" onClick={() => void downloadGet(`/ai/captains/${c.id}/export`, `${c.id}.json`)}>Экспорт JSON</button>
        <button className="secondary" onClick={onClose}>Закрыть</button></div>
      <div className="grid2">
        <label>Имя<input value={c.name} onChange={(e) => set({ name: e.target.value })} /></label>
        <label>Позывной<input value={c.call_sign} onChange={(e) => set({ call_sign: e.target.value })} /></label>
        <label>Возраст<input type="number" min={18} max={90} value={c.age} onChange={(e) => set({ age: Number(e.target.value) })} /></label>
        <label>Профессия<select value={c.profession} onChange={(e) => set({ profession: e.target.value })}>{meta.professions.map((p) => <option key={p}>{p}</option>)}</select></label>
        <label>Фракция<select value={c.faction} onChange={(e) => set({ faction: e.target.value, corporation: null })}>{meta.factions.map((f) => <option key={f.id} value={f.id}>{f.name}</option>)}</select></label>
        <label>Корпорация<select value={c.corporation ?? ""} onChange={(e) => set({ corporation: e.target.value || null })}>
          <option value="">независимый</option>{corps.map((k) => <option key={k.id} value={k.id}>{k.name}</option>)}</select></label>
      </div>
      <label>Биография<textarea rows={5} value={c.bio} onChange={(e) => set({ bio: e.target.value })} /></label>
      <h3>Характер</h3>
      <div className="grid2">
        <label>Черты (через запятую, 3–6)<input value={c.personality.traits.join(", ")} onChange={(e) => set({ personality: { ...c.personality, traits: e.target.value.split(",").map((x) => x.trim()).filter(Boolean) } })} /></label>
        <label>Темперамент<select value={c.personality.temper} onChange={(e) => set({ personality: { ...c.personality, temper: e.target.value } })}>
          {[["calm", "спокойный"], ["hot", "вспыльчивый"], ["cold", "холодный"], ["cheerful", "весёлый"]].map(([v, t]) => <option key={v} value={v}>{t}</option>)}</select></label>
        {Object.entries(c.personality.scores).map(([k, v]) => (
          <label key={k}>{{ friendliness: "Дружелюбие", caution: "Осторожность", curiosity: "Любопытство", greed: "Жадность", loyalty: "Верность" }[k] ?? k}: {v}
            <input type="range" min={0} max={100} value={v} onChange={(e) => set({ personality: { ...c.personality, scores: { ...c.personality.scores, [k]: Number(e.target.value) } } })} /></label>
        ))}
      </div>
      <h3>Речевой стиль</h3>
      <div className="grid2">
        <label>Формальность<select value={c.speech.formality} onChange={(e) => sp({ formality: e.target.value })}><option value="formal">формальная</option><option value="neutral">обычная</option><option value="casual">разговорная</option></select></label>
        <label>Многословие<select value={c.speech.verbosity} onChange={(e) => sp({ verbosity: e.target.value })}><option value="terse">лаконичный</option><option value="normal">обычный</option><option value="talkative">разговорчивый</option></select></label>
        <label>Юмор<select value={c.speech.humor} onChange={(e) => sp({ humor: e.target.value })}><option value="none">без юмора</option><option value="dry">сухой</option><option value="playful">игривый</option><option value="sarcastic">колкий</option></select></label>
        <label>Уровень сленга (0–3)<input type="number" min={0} max={3} value={c.speech.slang_level} onChange={(e) => sp({ slang_level: Number(e.target.value) })} /></label>
        <label>Обращение к игроку<input value={c.speech.address_player} onChange={(e) => sp({ address_player: e.target.value })} /></label>
        <label>Избегаемые слова (через запятую)<input value={c.speech.avoid_words.join(", ")} onChange={(e) => sp({ avoid_words: e.target.value.split(",").map((x) => x.trim()).filter(Boolean) })} /></label>
      </div>
      <label>Коронные фразы (по одной в строке, 2–6)<textarea rows={3} value={c.speech.catchphrases.join("\n")} onChange={(e) => sp({ catchphrases: lines(e.target.value) })} /></label>
      <label>Особенности речи (по одной в строке)<textarea rows={2} value={c.speech.quirks.join("\n")} onChange={(e) => sp({ quirks: lines(e.target.value) })} /></label>
      <h3>Интересы, цели, стратегии</h3>
      <label>Интересы (по одному в строке, 2–8)<textarea rows={3} value={c.interests.join("\n")} onChange={(e) => set({ interests: lines(e.target.value) })} /></label>
      <label>Цели (в строке: приоритет 1–5, пробел, текст)<textarea rows={3} value={c.goals.map((g) => `${g.priority} ${g.text}`).join("\n")} onChange={(e) =>
        set({ goals: lines(e.target.value).map((l, i) => { const m = /^(\d)\s+(.*)$/.exec(l); const old = c.goals[i]; return { id: old?.id ?? `goal-${i + 1}`, priority: m ? Number(m[1]) : i + 1, text: m ? m[2] : l }; }) })} /></label>
      <fieldset><legend>Стратегии</legend>
        {meta.strategies.map((s) => (
          <label key={s.id} className="inline"><input type="checkbox" checked={c.strategies.includes(s.id)} onChange={(e) => set({ strategies: e.target.checked ? [...c.strategies, s.id] : c.strategies.filter((x) => x !== s.id) })} /> {s.title}</label>
        ))}
      </fieldset>
      <h3>Личные ответы</h3>
      {Object.entries(c.replies).sort().map(([k, v]) => (
        <label key={k}>{k}<textarea rows={2} value={v.join("\n")} onChange={(e) => set({ replies: { ...c.replies, [k]: lines(e.target.value) } })} /></label>
      ))}
      <p className="muted">Подстановки: {"{player}"}, {"{captain}"}, {"{topics}"}, {"{fact:id}"}. Игровые факты — только из раздела «Диалоги → факты».</p>
      <h3>Отношения</h3>
      {c.relationships.length === 0 ? <p className="muted">Нет.</p> : (
        <table><tbody>{c.relationships.map((r) => <tr key={r.captain_id}><td>{r.captain_id}</td><td>{{ friend: "друг", rival: "соперник", partner: "партнёр", neutral: "знакомый", enemy: "враг" }[r.kind] ?? r.kind}</td><td className="wrap">{r.note}</td></tr>)}</tbody></table>
      )}
      {err && <p role="alert" className="error">{err}</p>}{msg && <p className="ok">{msg}</p>}
      <div className="row"><button onClick={() => void save()}>Сохранить как правку вручную</button></div>
      <p className="muted">Профиль, сохранённый здесь, помечается «правка вручную»: генератор и импорты ChatGPT его не перезапишут без вашего разрешения.</p>
    </section>
  );
}
