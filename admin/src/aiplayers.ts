// Клиент админ-API «ИИ-игроки» (ИИ-3).

import { API } from "./api";

export interface AiStatus {
  enabled: boolean;
  paused: boolean;
  players: number;
  online: number;
  target: number;
  profiles: number;
  by_behavior: Record<string, number>;
}

export interface AiSettings {
  enabled: boolean;
  paused: boolean;
  min_online: number;
  max_online: number;
  abs_max: number;
  players_for_max: number;
  spawn_per_min: number;
  despawn_per_min: number;
  empty_grace_sec: number;
  view_radius: number;
  shares: Record<string, number>;
  chat_reply_enabled: boolean;
  chat_reply_chance: number;
  chat_delay_min_ms: number;
  chat_delay_max_ms: number;
  chat_captain_cooldown_sec: number;
  chat_max_per_min: number;
  disabled_captains: string[];
}

export interface AiCaptain {
  id: string;
  name: string;
  profession: string;
  behavior: string;
  since: string; // пусто — не онлайн
}

export interface AiPlayersData {
  status: AiStatus;
  settings: AiSettings;
  captains: AiCaptain[];
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    credentials: "same-origin",
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) {
    let msg = `Ошибка ${res.status}`;
    try {
      const doc = await res.json();
      if (doc && typeof doc.error === "string") msg = doc.error;
    } catch {
      /* оставляем статус */
    }
    throw new Error(msg);
  }
  return (await res.json()) as T;
}

export const aiPlayersApi = {
  get: () => request<AiPlayersData>("GET", `${API}/ai-players`),
  saveSettings: (settings: AiSettings) => request<{ ok: boolean }>("PUT", `${API}/ai-players/settings`, settings),
  setCaptain: (id: string, disable: boolean) =>
    request<{ ok: boolean }>("POST", `${API}/ai-players/captains/${encodeURIComponent(id)}/${disable ? "disable" : "enable"}`),
};
