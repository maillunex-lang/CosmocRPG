// Клиент API AI Training Studio. Сессия — HttpOnly-cookie; все запросы идут на тот же источник.
import { API, ApiError } from "../api";

export interface Issue { level: string; code: string; path: string; message: string }
export interface Report {
  ok: boolean; errors: Issue[]; warnings: Issue[]; stats: Record<string, number>;
  pack_id: string; kind: string; integrity: string;
}
export interface CaptainChange { id: string; name: string; kind: string; fields?: string[]; reason?: string }
export interface Plan {
  captains: CaptainChange[] | null; sections: { name: string; added: number; changed: number; unchanged: number }[] | null;
  added: number; changed: number; unchanged: number; conflicts: number;
}
export interface TestResult { kind: string; title: string; total: number; passed: number; failures: string[]; gate: boolean }
export interface ImportRec {
  id: number; created_at: string; admin: string; filename: string; sha256: string; status: string; pack_id: string; kind: string;
  gate_ok: boolean; version_id: number | null; report: Report; plan: Plan; gate: TestResult[] | null;
}
export interface VersionInfo { id: number; created_at: string; admin: string; note: string; source: string; active: boolean; captains: number }
export interface Session { id: number; created_at: string; label: string; batch_size: number; pack_id: string; sha256: string }
export interface TestRun { id: number; created_at: string; total: number; passed: number; sample: number; results: TestResult[] }
export interface Dashboard {
  captains: number; target: number; by_source: Record<string, number>; active_version: number; versions: VersionInfo[] | null;
  imports: ImportRec[] | null; sessions: Session[] | null; last_tests: TestRun | null; validation_errors: number;
  quality: Record<string, unknown>; statuses: Record<string, number>;
}
export interface CaptainSummary {
  id: string; name: string; call_sign: string; profession: string; faction: string; corporation: string;
  formality: string; humor: string; source: string; manual: boolean; revision: number;
}
export interface Captain {
  id: string; revision: number; name: string; call_sign: string; gender: string; age: number; bio: string; profession: string;
  faction: string; corporation: string | null;
  personality: { traits: string[]; scores: Record<string, number>; temper: string };
  speech: { formality: string; verbosity: string; slang_level: number; humor: string; catchphrases: string[]; quirks: string[]; address_player: string; avoid_words: string[] };
  interests: string[]; goals: { id: string; text: string; priority: number }[]; strategies: string[];
  relationships: { captain_id: string; kind: string; note: string }[]; replies: Record<string, string[]>;
  meta: { manual: boolean; source: string; seed: number };
}
export interface Transcript { speaker: string; name: string; text: string; intent?: string; confidence?: number; mood?: string; clarify?: boolean; source?: string }

async function call<T>(method: string, path: string, body?: unknown, raw?: { data: BodyInit; type: string }): Promise<T> {
  const res = await send(method, path, body, raw);
  try {
    return (await res.json()) as T;
  } catch {
    throw new ApiError(res.status, "Ответ сервера не распознан");
  }
}

async function send(method: string, path: string, body?: unknown, raw?: { data: BodyInit; type: string }): Promise<Response> {
  let res: Response;
  try {
    res = await fetch(`${API}${path}`, {
      method,
      credentials: "same-origin",
      headers: raw ? { "Content-Type": raw.type } : body === undefined ? undefined : { "Content-Type": "application/json" },
      body: raw ? raw.data : body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, "Нет связи с сервером");
  }
  if (!res.ok) {
    let msg = `Ошибка ${res.status}`;
    try {
      const d = (await res.clone().json()) as { error?: string };
      if (d.error) msg = d.error;
    } catch { /* не JSON */ }
    throw new ApiError(res.status, msg);
  }
  return res;
}

/** Скачивает ответ как файл. */
export async function download(path: string, body: unknown, fallbackName: string): Promise<string> {
  const res = await send("POST", path, body);
  const blob = await res.blob();
  const cd = res.headers.get("Content-Disposition") ?? "";
  const m = /filename="([^"]+)"/.exec(cd);
  const name = m ? m[1] : fallbackName;
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 5000);
  return name;
}

export async function downloadGet(path: string, fallbackName: string): Promise<void> {
  const res = await send("GET", path);
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = fallbackName;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 5000);
}

export const ai = {
  dashboard: () => call<Dashboard>("GET", "/ai/dashboard"),
  generate: (count: number) => call<{ added: number; skipped: number; version: number }>("POST", "/ai/generate", { count }),
  nicknames: () => call<{ renamed: number; skipped: number; version: number }>("POST", "/ai/nicknames", {}),
  captains: (q: URLSearchParams) =>
    call<{ total: number; items: CaptainSummary[]; version: number; factions: { id: string; name: string }[]; corporations: { id: string; name: string; faction: string }[]; professions: string[]; strategies: { id: string; title: string }[] }>("GET", `/ai/captains?${q}`),
  captain: (id: string) => call<Captain>("GET", `/ai/captains/${id}`),
  saveCaptain: (c: Captain) => call<{ version: number }>("PUT", `/ai/captains/${c.id}`, c),
  importCaptain: (c: unknown, overwrite: boolean) => call<{ version: number }>("POST", `/ai/captains/import${overwrite ? "?overwrite=1" : ""}`, c),
  uniqueness: () => call<{ captains: number; duplicates: string[]; similar: { a: string; b: string; score: number }[]; max_similarity: number }>("GET", "/ai/uniqueness"),
  section: (s: string) => call<{ version: number; doc: unknown }>("GET", `/ai/dialogue/${s}`),
  saveSection: (s: string, doc: unknown) => call<{ version: number }>("PUT", `/ai/dialogue/${s}`, doc),
  importZip: (file: File, overwriteManual: boolean) =>
    call<ImportRec>("POST", `/ai/zip/import?filename=${encodeURIComponent(file.name)}${overwriteManual ? "&overwrite_manual=1" : ""}`, undefined, { data: file, type: "application/zip" }),
  imports: () => call<{ items: ImportRec[] | null }>("GET", "/ai/imports"),
  publish: (id: number, acceptWarnings: boolean, overwriteManual: boolean) =>
    call<{ version: number }>("POST", `/ai/imports/${id}/publish`, { accept_warnings: acceptWarnings, overwrite_manual: overwriteManual }),
  discard: (id: number) => call<{ status: string }>("POST", `/ai/imports/${id}/discard`, {}),
  versions: () => call<{ items: VersionInfo[] | null }>("GET", "/ai/versions"),
  rollback: (id: number) => call<{ version: number }>("POST", `/ai/versions/${id}/rollback`, {}),
  sessions: () => call<{ items: Session[] | null }>("GET", "/ai/sessions"),
  simulate: (captain_id: string, say: string[]) => call<{ transcript: Transcript[] }>("POST", "/ai/academy/simulate", { captain_id, say }),
  group: (ids: string[], opening: string, turns: number) =>
    call<{ transcript: Transcript[]; issues: string[] | null; distinct_replies: number }>("POST", "/ai/academy/group", { ids, opening, turns }),
  tests: (kinds: string[], sample: number) => call<TestRun>("POST", "/ai/academy/tests", { kinds, sample }),
  reports: () => call<{ runs: TestRun[] | null; quality: Record<string, unknown>; kinds: string[] }>("GET", "/ai/academy/reports"),
  prompt: (batch: number) => call<{ prompt: string }>("GET", `/ai/prompt?batch=${batch}`),
};

export function errText(e: unknown): string {
  return e instanceof ApiError ? e.message : "Неизвестная ошибка";
}

export function copyText(text: string): boolean {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      void navigator.clipboard.writeText(text);
      return true;
    }
  } catch { /* ниже запасной путь */ }
  const ta = document.createElement("textarea");
  ta.value = text;
  ta.style.position = "fixed";
  ta.style.opacity = "0";
  document.body.appendChild(ta);
  ta.select();
  let ok = false;
  try { ok = document.execCommand("copy"); } catch { ok = false; }
  ta.remove();
  return ok;
}
