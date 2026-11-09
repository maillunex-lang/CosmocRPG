import { Suspense, lazy, useCallback, useEffect, useState } from "react";
import { ApiError } from "./api";
import {
  CATEGORY_NAMES, STATE_NAMES, STATUS_NAMES, UploadError, designApi, formatBytes,
  type DesignAsset, type DesignElement, type DesignFilter, type DesignStatus, type Facets, type Report,
} from "./design";

const ModelPreview = lazy(() => import("./ModelPreview").then((m) => ({ default: m.ModelPreview })));

const err = (e: unknown) => (e instanceof ApiError ? e.message : "Неизвестная ошибка");
const EMPTY: DesignFilter = { category: "", group: "", state: "", tag: "", q: "" };
const isImage = (mime: string) => mime.startsWith("image/");

function ReportView({ r }: { r: Report }) {
  return (
    <div className="report" aria-label="Отчёт проверки файла">
      <p className={r.ok ? "ok" : "error"}>{r.ok ? "Проверка пройдена" : "Файл не прошёл проверку"}</p>
      {r.errors.length > 0 && <ul>{r.errors.map((e, i) => <li key={i} className="error">{e}</li>)}</ul>}
      {r.warnings.length > 0 && <ul>{r.warnings.map((e, i) => <li key={i} className="warn">{e}</li>)}</ul>}
      {Object.keys(r.info ?? {}).length > 0 && <p className="muted mono">{Object.entries(r.info).map(([k, v]) => `${k}: ${Array.isArray(v) ? v.join("×") : String(v)}`).join(" · ")}</p>}
    </div>
  );
}

function Thumb({ el }: { el: DesignElement }) {
  if (el.published && isImage(el.published.mime)) return <img className="thumb" alt="" src={designApi.fileUrl(el.published.id)} />;
  if (el.preview_svg) return <img className="thumb" alt="" src={`data:image/svg+xml;utf8,${encodeURIComponent(el.preview_svg)}`} />;
  return <div className="thumb placeholder" aria-hidden="true">{el.category === "model" ? "3D" : el.category === "ship_data" ? "JSON" : "IMG"}</div>;
}

export function DesignAdmin({ username, onBack, onLoggedOut }: { username: string; onBack: () => void; onLoggedOut: () => void }) {
  const [filter, setFilter] = useState<DesignFilter>(EMPTY);
  const [items, setItems] = useState<DesignElement[]>([]);
  const [facets, setFacets] = useState<Facets | null>(null);
  const [status, setStatus] = useState<DesignStatus | null>(null);
  const [selected, setSelected] = useState<string>("");
  const [detail, setDetail] = useState<{ element: DesignElement; assets: DesignAsset[] } | null>(null);
  const [prompt, setPrompt] = useState<{ id: string; text: string } | null>(null);
  const [notes, setNotes] = useState("");
  const [report, setReport] = useState<Report | null>(null);
  const [preview3d, setPreview3d] = useState<number | null>(null);
  const [msg, setMsg] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const loadList = useCallback(() => {
    designApi.elements(filter).then((d) => { setItems(d.items ?? []); setFacets(d.facets); }).catch((e) => setError(err(e)));
  }, [filter]);
  const loadStatus = useCallback(() => { designApi.status().then(setStatus).catch((e) => setError(err(e))); }, []);
  const loadDetail = useCallback((id: string) => {
    if (!id) { setDetail(null); return; }
    designApi.element(id).then((d) => setDetail({ element: d.element, assets: d.assets ?? [] })).catch((e) => setError(err(e)));
  }, []);
  useEffect(() => { loadList(); }, [loadList]);
  useEffect(() => { loadStatus(); }, [loadStatus]);
  useEffect(() => { loadDetail(selected); setReport(null); setPreview3d(null); setNotes(""); }, [selected, loadDetail]);

  const refresh = () => { loadList(); loadStatus(); loadDetail(selected); };

  async function copyPrompt(id: string) {
    setError(""); setMsg("");
    try {
      const { prompt: text } = await designApi.prompt(id);
      setPrompt({ id, text });
      try { await navigator.clipboard.writeText(text); setMsg("Промпт скопирован в буфер обмена"); }
      catch { setMsg("Не удалось скопировать автоматически: выделите текст ниже и скопируйте вручную"); }
    } catch (e) { setError(err(e)); }
  }

  async function upload(file: File | undefined) {
    if (!file || !selected) return;
    setBusy(true); setError(""); setMsg(""); setReport(null);
    try {
      const a = await designApi.upload(selected, file, notes);
      setReport(a.report);
      setMsg(`Загружена версия ${a.version} (черновик). Проверьте предпросмотр и опубликуйте`);
      setNotes("");
      refresh();
    } catch (e) {
      setError(err(e));
      if (e instanceof UploadError && e.report) setReport(e.report);
    } finally { setBusy(false); }
  }

  async function act(fn: () => Promise<unknown>, ok: string) {
    setError(""); setMsg("");
    try { await fn(); setMsg(ok); refresh(); } catch (e) { setError(err(e)); }
  }

  const el = detail?.element;
  const setF = (k: keyof DesignFilter) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => setFilter({ ...filter, [k]: e.target.value });
  const opts = (m?: Record<string, number>, names?: Record<string, string>) =>
    Object.entries(m ?? {}).sort((a, b) => a[0].localeCompare(b[0], "ru")).map(([k, n]) => <option key={k} value={k}>{`${names?.[k] ?? k} (${n})`}</option>);
  const tagOpts = Object.entries(facets?.tags ?? {}).filter(([t]) => t).sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0], "ru")).slice(0, 60);

  return (
    <main className="dashboard studio design">
      <header>
        <h1>Дизайн: элементы игры</h1>
        <div><span className="muted">{username}</span><button className="secondary" onClick={onBack}>Сервер</button><button onClick={onLoggedOut}>Выйти</button></div>
      </header>
      {error && <p role="alert" className="error">{error}</p>}
      {msg && <p className="ok">{msg}</p>}

      <section className="card" aria-label="Состояние библиотеки">
        {status ? (
          <p>
            {status.can_publish ? <span className="ok">Публикация включена</span> : <span className="warn">Публикация выключена: ключ подписи не подключён</span>}
            {" · "}Элементов в каталоге: {status.elements} · Заменено в игре: {status.published} · Версия набора ресурсов: {status.manifest_version}
          </p>
        ) : <p className="muted">Загрузка…</p>}
        <p className="muted">Замены попадают в игру без переустановки приложения: игра докачивает подписанный набор ресурсов при запуске и по кнопке «Проверить внутренние файлы» в настройках.</p>
      </section>

      <section className="card" aria-label="Фильтры">
        <div className="row wrap">
          <input aria-label="Поиск" placeholder="Поиск: название, корабль, тег…" value={filter.q} onChange={setF("q")} />
          <select aria-label="Категория" value={filter.category} onChange={setF("category")}><option value="">Все категории</option>{opts(facets?.categories, CATEGORY_NAMES)}</select>
          <select aria-label="Группа" value={filter.group} onChange={setF("group")}><option value="">Все группы</option>{opts(facets?.groups)}</select>
          <select aria-label="Состояние" value={filter.state} onChange={setF("state")}><option value="">Любое состояние</option>{opts(facets?.states, STATE_NAMES)}</select>
          <select aria-label="Тег" value={filter.tag} onChange={setF("tag")}><option value="">Любой тег</option>{tagOpts.map(([t, n]) => <option key={t} value={t}>{`${t} (${n})`}</option>)}</select>
          <button className="secondary" onClick={() => setFilter(EMPTY)}>Сбросить</button>
        </div>
        <p className="muted">Найдено: {items.length}</p>
      </section>

      <section aria-label="Каталог" className="design-grid">
        {items.length === 0 && <p className="muted">Ничего не найдено</p>}
        {items.map((it) => (
          <article key={it.id} className={`design-card${selected === it.id ? " selected" : ""}`}>
            <button className="card-main" aria-label={`Открыть: ${it.name}`} onClick={() => setSelected(it.id)}>
              <Thumb el={it} />
              <strong>{it.name}</strong>
              <span className="muted mono">{it.id}</span>
              <span><span className={`badge ${it.state}`}>{STATE_NAMES[it.state]}</span>{it.placeholder && <span className="badge warnb">заглушка</span>}</span>
            </button>
            <button className="secondary small" aria-label={`Скопировать промпт: ${it.name}`} onClick={() => void copyPrompt(it.id)}>Скопировать промпт</button>
          </article>
        ))}
      </section>

      {prompt && (
        <section className="card" aria-label="Промпт">
          <h2>Промпт для создания: {prompt.id}</h2>
          <textarea className="prompt" readOnly value={prompt.text} onFocus={(e) => e.currentTarget.select()} aria-label="Текст промпта" />
          <div className="row"><button onClick={() => void copyPrompt(prompt.id)}>Скопировать ещё раз</button><button className="secondary" onClick={() => setPrompt(null)}>Закрыть</button></div>
        </section>
      )}

      {el && detail && (
        <section className="card" aria-label="Карточка элемента">
          <h2>{el.name}</h2>
          <dl>
            <dt>Ключ</dt><dd className="mono">{el.id}</dd>
            <dt>Категория</dt><dd>{CATEGORY_NAMES[el.category]} · {el.group}</dd>
            <dt>Состояние</dt><dd>{STATE_NAMES[el.state]}{el.published ? ` (версия ${el.published.version})` : ""}</dd>
            <dt>Встроенный файл</dt><dd className="mono">{el.builtin} · {formatBytes(el.bytes)}{el.dims ? ` · ${el.dims.join("×")}` : ""}</dd>
            {el.ship && <><dt>Корабль</dt><dd>{el.ship}{el.skin ? `, облик «${el.skin}»` : ""}{el.class ? ` · ${el.class}` : ""}</dd></>}
            {el.model_length ? <><dt>Ожидаемая модель</dt><dd>длина {el.model_length}, нос по оси {el.nose_axis}</dd></> : null}
            {el.used_by && el.used_by.length > 0 && <><dt>Где используется</dt><dd>{el.used_by.join(", ")}</dd></>}
            <dt>Теги</dt><dd>{el.tags.filter(Boolean).join(", ") || "—"}</dd>
            {el.note && <><dt>Заметка</dt><dd>{el.note}</dd></>}
          </dl>
          {el.placeholder && <p className="warn">Сейчас используется временная модель-заглушка. Загрузите собственную модель этого корабля.</p>}

          <h2>Загрузить замену</h2>
          <div className="row wrap">
            <input aria-label="Заметка к версии" placeholder="Заметка к версии (необязательно)" maxLength={500} value={notes} onChange={(e) => setNotes(e.target.value)} />
            <input aria-label="Файл замены" type="file" disabled={busy} accept={el.category === "model" ? ".glb" : el.category === "ship_data" ? ".json" : el.category === "icon" ? ".svg,.png,.jpg,.jpeg" : ".webp,.png,.jpg,.jpeg"} onChange={(e) => { void upload(e.target.files?.[0]); e.target.value = ""; }} />
            <button className="secondary" onClick={() => void copyPrompt(el.id)}>Скопировать промпт</button>
          </div>
          <p className="muted">Файл проверяется на сервере: формат, размер, безопасность. Сначала он сохраняется как черновик; в игру попадает только после публикации.</p>
          {report && <ReportView r={report} />}

          <h2>История замен</h2>
          <table aria-label="История замен">
            <thead><tr><th>Версия</th><th>Файл</th><th>Размер</th><th>Статус</th><th>Загружена</th><th>Проверка</th><th /></tr></thead>
            <tbody>
              {detail.assets.length === 0 ? <tr><td colSpan={7} className="muted">Замен нет: в игре встроенный элемент</td></tr> : detail.assets.map((a) => (
                <tr key={a.id}>
                  <td>{a.version}</td>
                  <td className="wrap">{a.file_name}{a.notes ? <div className="muted">{a.notes}</div> : null}</td>
                  <td>{formatBytes(a.size)}</td>
                  <td><span className={`badge ${a.status}`}>{STATUS_NAMES[a.status]}</span></td>
                  <td>{new Date(a.created_at).toLocaleString("ru-RU")} · {a.uploaded_by}</td>
                  <td className="wrap">{a.report.warnings.length > 0 ? <span className="warn">предупреждений: {a.report.warnings.length}</span> : <span className="ok">без замечаний</span>}</td>
                  <td className="actions">
                    {isImage(a.mime) && <a href={designApi.fileUrl(a.id)} target="_blank" rel="noreferrer">Открыть</a>}
                    {a.mime === "model/gltf-binary" && <button className="secondary small" onClick={() => setPreview3d(preview3d === a.id ? null : a.id)}>{preview3d === a.id ? "Скрыть 3D" : "Показать 3D"}</button>}
                    <a href={designApi.fileUrl(a.id)} download={a.file_name}>Скачать</a>
                    {a.status !== "published" && <button disabled={!status?.can_publish} title={status?.can_publish ? "" : "Ключ подписи не подключён"} onClick={() => void act(() => designApi.publish(a.id), `Версия ${a.version} опубликована: игроки получат её при следующей проверке файлов`)}>Опубликовать</button>}
                    {a.status === "published" && <button className="secondary" onClick={() => void act(() => designApi.unpublish(el.id), "Замена снята: в игре снова встроенный элемент")}>Снять с публикации</button>}
                    {a.status !== "published" && <button className="secondary" onClick={() => void act(() => designApi.remove(a.id), `Версия ${a.version} удалена`)}>Удалить</button>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {preview3d !== null && <Suspense fallback={<p className="muted">Загрузка предпросмотра…</p>}><ModelPreview url={designApi.fileUrl(preview3d)} /></Suspense>}
        </section>
      )}
    </main>
  );
}
