// Клиент админ-API. Сессия хранится в HttpOnly-cookie, JS токен не видит.

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

export interface Status {
  version: string;
  database: string;
  started_at: string;
  uptime_seconds: number;
  server_time: string;
  ws_sessions: number;
  boots_total: number;
}

export interface AuditEntry {
  id: number;
  at: string;
  username: string;
  action: string;
  ip: string;
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      credentials: "same-origin",
      headers: body === undefined ? undefined : { "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, "Нет связи с сервером");
  }
  let data: unknown = null;
  try {
    data = await res.json();
  } catch {
    /* тело не JSON */
  }
  if (!res.ok) {
    const msg =
      data && typeof data === "object" && "error" in data ? String((data as { error: unknown }).error) : `Ошибка ${res.status}`;
    throw new ApiError(res.status, msg);
  }
  return data as T;
}

// Админка размещена под базовым путём (vite `base`, сейчас "/admin/"); API лежит под ним же.
export const API = `${import.meta.env.BASE_URL}api`;

export const api = {
  login: (username: string, password: string) => request<{ username: string }>("POST", `${API}/login`, { username, password }),
  logout: () => request<{ status: string }>("POST", `${API}/logout`),
  me: () => request<{ username: string }>("GET", `${API}/me`),
  changePassword: (current: string, next: string) =>
    request<{ status: string; other_sessions_revoked: number }>("POST", `${API}/password`, { current, new: next }),
  status: () => request<Status>("GET", `${API}/status`),
  audit: (limit = 20) => request<{ entries: AuditEntry[] }>("GET", `${API}/audit?limit=${limit}`),
};

export function formatUptime(seconds: number): string {
  const s = Math.max(0, Math.floor(seconds));
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  const parts: string[] = [];
  if (d) parts.push(`${d} д`);
  if (d || h) parts.push(`${h} ч`);
  parts.push(`${m} мин`);
  return parts.join(" ");
}
