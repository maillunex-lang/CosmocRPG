// Клиент API раздела «Чат»: журнал, мьюты, настройки.
import { API, ApiError } from "./api";

export interface ChatMessage { id: number; channel: string; from_id: string; from_name: string; from_ai?: boolean; to_id?: string; to_name?: string; text: string; at: number }
export interface ChatMute { player_id: string; player_name: string; until: string | null; reason: string; admin: string; created_at: string }
export interface ChatSettings { enabled: boolean; retention_days: number; min_interval_ms: number; max_len: number; history_limit: number; profanity_filter: boolean; links_allowed: boolean; extra_blocked: string[] }

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`${API}${path}`, { method, credentials: "same-origin", headers: body === undefined ? undefined : { "Content-Type": "application/json" }, body: body === undefined ? undefined : JSON.stringify(body) });
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

export const chatApi = {
  messages: (channel: string, q: string) => call<{ items: ChatMessage[] }>("GET", `/chat/messages?limit=100&channel=${encodeURIComponent(channel)}&q=${encodeURIComponent(q)}`),
  mutes: () => call<{ items: ChatMute[] }>("GET", "/chat/mutes"),
  mute: (player: string, minutes: number, reason: string) => call<ChatMute>("POST", "/chat/mutes", { player, minutes, reason }),
  unmute: (id: string) => call<{ status: string }>("DELETE", `/chat/mutes/${encodeURIComponent(id)}`),
  settings: () => call<ChatSettings>("GET", "/chat/settings"),
  saveSettings: (s: ChatSettings) => call<ChatSettings>("PUT", "/chat/settings", s),
};
