import { useEffect, useState } from "react";
import { ai, errText, type TestRun, type Transcript } from "./aiapi";

const KIND_NAMES: Record<string, string> = {
  structure: "Структура и ссылки", uniqueness: "Уникальность капитанов", intents: "Распознавание намерений", character: "Согласованность характера",
  facts: "Отсутствие выдуманных фактов", context: "Работа контекста", memory: "Память о собеседнике", repeats: "Защита от повторов",
  safety: "Безопасные уточнения", regression: "Регрессионные диалоги",
};

function Chat({ items }: { items: Transcript[] }) {
  return (
    <ul className="chat">{items.map((t, i) => (
      <li key={i} className={t.speaker === "player" || t.speaker === "host" ? "me" : ""}>
        <strong>{t.name}:</strong> {t.text}
        {t.intent && <small className="muted"> [{t.intent}{t.confidence !== undefined ? ` ${t.confidence.toFixed(2)}` : ""}{t.clarify ? ", уточнение" : ""}{t.mood ? `, ${t.mood}` : ""}]</small>}
      </li>))}</ul>
  );
}

export function Academy() {
  const [cap, setCap] = useState("cpt-0001");
  const [say, setSay] = useState("привет\nчем занимаешься?\nкак включить автопилот\nменя зовут Алекс\nкак меня зовут\nрасскажи про древний храм");
  const [tr, setTr] = useState<Transcript[]>([]);
  const [group, setGroup] = useState("cpt-0001, cpt-0002, cpt-0003");
  const [opening, setOpening] = useState("привет всем");
  const [turns, setTurns] = useState(9);
  const [gtr, setGtr] = useState<Transcript[]>([]);
  const [gIssues, setGIssues] = useState<string[]>([]);
  const [kinds, setKinds] = useState<string[]>(Object.keys(KIND_NAMES));
  const [sample, setSample] = useState(25);
  const [run, setRun] = useState<TestRun | null>(null);
  const [hist, setHist] = useState<TestRun[]>([]);
  const [quality, setQuality] = useState<Record<string, unknown>>({});
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  function loadReports() { ai.reports().then((r) => { setHist(r.runs ?? []); setQuality(r.quality); }).catch((e) => setErr(errText(e))); }
  useEffect(loadReports, []);

  async function simulate() {
    setErr(""); setBusy(true);
    try { setTr((await ai.simulate(cap.trim(), say.split("\n").map((s) => s.trim()).filter(Boolean))).transcript); } catch (e) { setErr(errText(e)); } finally { setBusy(false); }
  }
  async function simGroup() {
    setErr(""); setBusy(true);
    try {
      const r = await ai.group(group.split(/[\s,;]+/).filter(Boolean), opening, turns);
      setGtr(r.transcript); setGIssues(r.issues ?? []);
    } catch (e) { setErr(errText(e)); } finally { setBusy(false); }
  }
  async function runTests() {
    setErr(""); setBusy(true);
    try { setRun(await ai.tests(kinds, sample)); loadReports(); } catch (e) { setErr(errText(e)); } finally { setBusy(false); }
  }

  return (
    <>
      {err && <p role="alert" className="error">{err}</p>}
      <section className="card" aria-label="Симулятор диалога">
        <h2>Симулятор диалога</h2>
        <div className="row wrap">
          <label className="inline">Капитан <input value={cap} onChange={(e) => setCap(e.target.value)} style={{ width: 110 }} /></label>
          <button disabled={busy} onClick={() => void simulate()}>Провести диалог</button>
        </div>
        <label>Реплики игрока (по одной в строке)<textarea rows={5} value={say} onChange={(e) => setSay(e.target.value)} /></label>
        {tr.length > 0 && <Chat items={tr} />}
      </section>

      <section className="card" aria-label="Групповой разговор">
        <h2>Групповой разговор капитанов</h2>
        <div className="row wrap">
          <input value={group} onChange={(e) => setGroup(e.target.value)} aria-label="Капитаны" style={{ flex: 1, minWidth: 220 }} />
          <input value={opening} onChange={(e) => setOpening(e.target.value)} aria-label="Вступительная реплика" style={{ minWidth: 160 }} />
          <label className="inline">Реплик <input type="number" min={2} max={30} value={turns} onChange={(e) => setTurns(Number(e.target.value))} style={{ width: 70 }} /></label>
          <button disabled={busy} onClick={() => void simGroup()}>Запустить</button>
        </div>
        {gtr.length > 0 && <Chat items={gtr} />}
        {gtr.length > 0 && (gIssues.length === 0 ? <p className="ok">Замечаний нет.</p> : <ul>{gIssues.map((x, i) => <li key={i} className="warn">{x}</li>)}</ul>)}
      </section>

      <section className="card" aria-label="Экзамены">
        <h2>Экзамены Академии</h2>
        <fieldset><legend>Проверки</legend>
          {Object.entries(KIND_NAMES).map(([k, n]) => (
            <label key={k} className="inline"><input type="checkbox" checked={kinds.includes(k)} onChange={(e) => setKinds(e.target.checked ? [...kinds, k] : kinds.filter((x) => x !== k))} /> {n}</label>
          ))}
        </fieldset>
        <div className="row wrap">
          <label className="inline">Капитанов в диалоговых тестах (0 — все)
            <input type="number" min={0} max={300} value={sample} onChange={(e) => setSample(Number(e.target.value))} style={{ width: 80 }} /></label>
          <button disabled={busy || kinds.length === 0} onClick={() => void runTests()}>{busy ? "Идёт проверка…" : "Запустить экзамен"}</button>
        </div>
        {run && (
          <div className="report">
            <h3 className={run.passed === run.total ? "ok" : "error"}>Итог: {run.passed} из {run.total}</h3>
            <table><thead><tr><th>Проверка</th><th>Результат</th><th>Блокирует публикацию</th></tr></thead>
              <tbody>{run.results.map((r) => (
                <tr key={r.kind}><td>{r.title}</td><td className={r.passed === r.total ? "ok" : "error"}>{r.passed}/{r.total}</td><td>{r.gate ? "да" : "нет"}</td></tr>))}</tbody></table>
            {run.results.filter((r) => r.failures.length > 0).map((r) => (
              <div key={r.kind}><h4>{r.title}: замечания</h4><ul>{r.failures.slice(0, 15).map((f, i) => <li key={i} className="wrap">{f}</li>)}</ul></div>))}
          </div>
        )}
      </section>

      <section className="card" aria-label="Отчёт качества">
        <h2>Отчёт качества</h2>
        <dl>
          <dt>Капитанов</dt><dd>{String(quality.captains ?? 0)}</dd>
          <dt>Средняя длина биографии</dt><dd>{String(quality.avg_bio_chars ?? 0)} знаков</dd>
          <dt>Личных ответов на капитана</dt><dd>{String(quality.avg_replies ?? 0)}</dd>
          <dt>Разных коронных фраз</dt><dd>{String(quality.distinct_catchphrases ?? 0)}</dd>
          <dt>Намерений / шаблонов / фактов / тестов</dt><dd>{String(quality.intents ?? 0)} / {String(quality.templates ?? 0)} / {String(quality.facts ?? 0)} / {String(quality.tests ?? 0)}</dd>
        </dl>
        <h3>История экзаменов</h3>
        {hist.length === 0 ? <p className="muted">Запусков ещё не было.</p> : (
          <table><thead><tr><th>Время</th><th>Выборка</th><th>Результат</th></tr></thead>
            <tbody>{hist.map((h) => <tr key={h.id}><td>{new Date(h.created_at).toLocaleString("ru-RU")}</td><td>{h.sample || "все"}</td><td className={h.passed === h.total ? "ok" : "error"}>{h.passed}/{h.total}</td></tr>)}</tbody></table>
        )}
      </section>
    </>
  );
}
