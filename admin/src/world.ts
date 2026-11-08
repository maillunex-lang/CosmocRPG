// Клиент API раздела «Локации» (конфигурация мира: системы, враги, ресурсы, графика; версии и откат).
import { API, ApiError } from "./api";

export interface SystemSummary { id: string; name: string; bodies: number; stations: number; zones: number; deposits: number; spawns: number; points: number }
export interface WorldCheck { ok: boolean; errors: string[]; systems: SystemSummary[] }
export interface WorldVersion { id: number; created_at: string; admin: string; note: string; source: string; active: boolean }
export interface WorldState { version: number; config: unknown; versions: WorldVersion[]; living_enabled: boolean; preview: WorldCheck }

async function call<T>(method: string, path: string, body?: unknown, raw?: string): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`${API}${path}`, {
      method, credentials: "same-origin",
      headers: raw !== undefined || body !== undefined ? { "Content-Type": "application/json" } : undefined,
      body: raw !== undefined ? raw : body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, "Нет связи с сервером");
  }
  if (!res.ok) {
    let msg = `Ошибка ${res.status}`;
    try { const d = (await res.clone().json()) as { error?: string }; if (d.error) msg = d.error; } catch { /* не JSON */ }
    throw new ApiError(res.status, msg);
  }
  return (await res.json()) as T;
}

export const worldApi = {
  get: () => call<WorldState>("GET", "/world"),
  check: (config: string) => call<WorldCheck>("POST", "/world/preview", undefined, config),
  publish: (config: unknown, note: string, base: number) => call<{ version: number; living_enabled: boolean }>("PUT", "/world", { config, note, base_version: base }),
  rollback: (id: number) => call<{ version: number }>("POST", `/world/versions/${id}/rollback`, {}),
};
