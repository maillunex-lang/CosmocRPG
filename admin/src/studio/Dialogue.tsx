import { useCallback, useEffect, useState } from "react";
import { ai, errText, type TestResult } from "./aiapi";

const SECTIONS: { id: string; title: string; hint: string }[] = [
  { id: "intents", title: "Намерения", hint: "Что игрок может спросить: примеры фраз и ключевые слова." },
  { id: "synonyms", title: "Синонимы, сленг, опечатки", hint: "Как приводить слова игроков к каноническому виду." },
  { id: "templates", title: "Шаблоны ответов", hint: "Ответы по намерениям и стилям речи. Подстановки: {player}, {captain}, {topics}, {fact:id}." },
  { id: "rules", title: "Контекстные правила", hint: "Как предыдущая тема меняет трактовку реплики («расскажи подробнее»)." },
  { id: "emotions", title: "Эмоциональные реакции", hint: "Какие реплики меняют настроение капитана и на сколько реплик." },
  { id: "memory_rules", title: "Память и отношения", hint: "Что капитан запоминает о собеседнике и как долго." },
  { id: "facts", title: "Игровые факты", hint: "Единственный источник правды о игре. Выдумывать факты запрещено." },
  { id: "dialogue_tests", title: "Тестовые диалоги", hint: "Реплики игрока и ожидания к ответу; используются как регрессионные тесты." },
  { id: "lessons", title: "Заметки обучения", hint: "Что усвоено и чему учили капитанов." },
];

export function Dialogue() {
  const [sec, setSec] = useState("intents");
  const [text, setText] = useState("");
  const [err, setErr] = useState("");
  const [msg, setMsg] = useState("");
  const [res, setRes] = useState<TestResult | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(() => {
    setErr(""); setMsg("");
    ai.section(sec).then((r) => setText(JSON.stringify(r.doc, null, 2))).catch((e) => setErr(errText(e)));
  }, [sec]);
  useEffect(load, [load]);

  async function save() {
    setErr(""); setMsg("");
    let doc: unknown;
    try { doc = JSON.parse(text); } catch { setErr("Это не корректный JSON"); return; }
    setBusy(true);
    try { const r = await ai.saveSection(sec, doc); setMsg(`Сохранено, версия ${r.version}`); } catch (e) { setErr(errText(e)); } finally { setBusy(false); }
  }

  async function runRegression() {
    setErr(""); setBusy(true);
    try { const run = await ai.tests(["regression", "facts", "context"], 25); setRes({ kind: "all", title: "Регрессионные диалоги, факты, контекст", total: run.total, passed: run.passed, failures: run.results.flatMap((r) => r.failures), gate: true }); } catch (e) { setErr(errText(e)); } finally { setBusy(false); }
  }

  const cur = SECTIONS.find((s) => s.id === sec);
  return (
    <section className="card" aria-label="Обучение диалогов">
      <h2>Диалоги: обучение движка</h2>
      <div className="row wrap" role="tablist">
        {SECTIONS.map((s) => <button key={s.id} role="tab" aria-selected={sec === s.id} className={sec === s.id ? "" : "secondary"} onClick={() => setSec(s.id)}>{s.title}</button>)}
      </div>
      <p className="muted">{cur?.hint}</p>
      <textarea className="json" value={text} onChange={(e) => setText(e.target.value)} spellCheck={false} aria-label="JSON раздела" />
      {err && <p role="alert" className="error">{err}</p>}{msg && <p className="ok">{msg}</p>}
      <div className="row wrap">
        <button disabled={busy} onClick={() => void save()}>Проверить и сохранить</button>
        <button className="secondary" disabled={busy} onClick={load}>Отменить правки</button>
        <button className="secondary" disabled={busy} onClick={() => void runRegression()}>Проверить тестовые диалоги</button>
      </div>
      <p className="muted">Сохранение создаёт новую версию только после проверки по JSON Schema, ссылок, фактов и регрессионных диалогов. Откат доступен на вкладке «Обзор».</p>
      {res && (
        <div className="report"><h3 className={res.passed === res.total ? "ok" : "error"}>{res.title}: {res.passed} из {res.total}</h3>
          {res.failures.length > 0 && <ul>{res.failures.slice(0, 20).map((f, i) => <li key={i} className="wrap">{f}</li>)}</ul>}</div>
      )}
    </section>
  );
}
