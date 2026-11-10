// Клиент библиотеки музыки (R-37, этап М-1).
import { API, ApiError } from "./api";

export type TrackKind = "music" | "ad" | "news" | "sfx";

export interface MediaTrack {
  id: number;
  kind: TrackKind;
  title: string;
  artist: string;
  duration_ms: number;
  sha256: string;
  size: number;
  ext: string;
  mime: string;
  license: string;
  source: string;
  ai_generated: boolean;
  tags: string;
  enabled: boolean;
  uploaded_by?: string;
  created_at: string;
  url?: string;
}

async function call<T>(method: string, path: string, body?: BodyInit, headers?: Record<string, string>): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`${API}${path}`, { method, credentials: "same-origin", headers, body });
  } catch {
    throw new ApiError(0, "Нет связи с сервером");
  }
  if (!res.ok) {
    let msg = `Ошибка ${res.status}`;
    try {
      const d = (await res.json()) as { error?: string };
      if (d.error) msg = d.error;
    } catch { /* не JSON */ }
    throw new ApiError(res.status, msg);
  }
  return (await res.json()) as T;
}

export const KIND_NAMES: Record<TrackKind, string> = {
  music: "Музыка",
  ad: "Реклама",
  news: "Новости",
  sfx: "Эффект",
};

export function formatDuration(ms: number): string {
  const s = Math.max(0, Math.round(ms / 1000));
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
}

export const radioApi = {
  list: () => call<{ items: MediaTrack[] }>("GET", "/media/tracks"),
  upload: (file: File, fields: { title: string; artist: string; kind: TrackKind; license: string; source: string; ai: boolean; tags: string }) => {
    const p = new URLSearchParams();
    p.set("name", file.name);
    p.set("title", fields.title);
    p.set("artist", fields.artist);
    p.set("kind", fields.kind);
    p.set("license", fields.license);
    p.set("source", fields.source || "Google Gemini");
    p.set("ai", fields.ai ? "1" : "0");
    p.set("tags", fields.tags);
    p.set("enabled", "1");
    return call<MediaTrack>("POST", `/media/tracks?${p.toString().replace(/\+/g, "%20")}`, file, { "Content-Type": "application/octet-stream" });
  },
  update: (id: number, track: MediaTrack) =>
    call<MediaTrack>("POST", `/media/tracks/${id}`, JSON.stringify({
      kind: track.kind, title: track.title, artist: track.artist, license: track.license,
      source: track.source, ai_generated: track.ai_generated, tags: track.tags, enabled: track.enabled,
    }), { "Content-Type": "application/json" }),
  remove: (id: number) => call<{ ok: boolean }>("DELETE", `/media/tracks/${id}`),
  fileUrl: (id: number) => `${API}/media/tracks/${id}/file`,
};
