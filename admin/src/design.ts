// Клиент API раздела «Дизайн»: каталог элементов, замены, публикация, промпты (R-53, R-54).
import { API, ApiError } from "./api";

export type DesignCategory = "icon" | "background" | "model" | "ship_data";
export type DesignState = "builtin" | "draft" | "replaced";

export interface Report { ok: boolean; errors: string[]; warnings: string[]; info: Record<string, unknown> }
export interface DesignAsset {
  id: number; element_id: string; version: number; file_name: string; mime: string; sha256: string; size: number;
  status: "draft" | "published" | "archived"; report: Report; notes: string; uploaded_by: string; created_at: string; published_at?: string;
}
export interface DesignElement {
  id: string; category: DesignCategory; name: string; group: string; tags: string[]; builtin: string; bytes: number;
  dims?: number[]; used_by?: string[]; preview_svg?: string; ship?: string; skin?: string; class?: string; placeholder?: boolean;
  model_length?: number; nose_axis?: string; current?: string; note?: string;
  state: DesignState; published?: DesignAsset; drafts: number; versions: number;
}
export interface Facets { categories: Record<string, number>; groups: Record<string, number>; states: Record<string, number>; tags: Record<string, number> }
export interface DesignStatus { can_publish: boolean; manifest_version: number; published: number; elements: number; public_key?: string }
export interface DesignFilter { category: string; group: string; state: string; tag: string; q: string }

/** Ошибка загрузки: сообщение сервера и отчёт проверки файла (если есть). */
export class UploadError extends ApiError {
  constructor(status: number, message: string, public report?: Report) { super(status, message); }
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
    let report: Report | undefined;
    try {
      const d = (await res.clone().json()) as { error?: string; report?: Report };
      if (d.error) msg = d.error;
      report = d.report;
    } catch { /* не JSON */ }
    throw new UploadError(res.status, msg, report);
  }
  return (await res.json()) as T;
}

const q = encodeURIComponent;

export function filterQuery(f: DesignFilter): string {
  const p = new URLSearchParams();
  (Object.keys(f) as (keyof DesignFilter)[]).forEach((k) => { if (f[k].trim()) p.set(k, f[k].trim()); });
  const s = p.toString();
  return s ? `?${s}` : "";
}

export const designApi = {
  status: () => call<DesignStatus>("GET", "/design/status"),
  elements: (f: DesignFilter) => call<{ items: DesignElement[]; facets: Facets }>("GET", `/design/elements${filterQuery(f)}`),
  element: (id: string) => call<{ element: DesignElement; assets: DesignAsset[] }>("GET", `/design/elements/${q(id)}`),
  upload: (id: string, file: File, notes: string) =>
    call<DesignAsset>("POST", `/design/elements/${q(id)}/upload?name=${q(file.name)}&notes=${q(notes)}`, file, { "Content-Type": "application/octet-stream" }),
  publish: (assetId: number) => call<{ manifest_version: number }>("POST", `/design/assets/${assetId}/publish`),
  unpublish: (id: string) => call<{ manifest_version: number }>("POST", `/design/elements/${q(id)}/unpublish`),
  remove: (assetId: number) => call<{ ok: boolean }>("DELETE", `/design/assets/${assetId}`),
  prompt: (id: string) => call<{ prompt: string }>("GET", `/design/prompt?element=${q(id)}`),
  fileUrl: (assetId: number) => `${API}/design/assets/${assetId}/file`,
};

export const CATEGORY_NAMES: Record<string, string> = { icon: "Иконки", background: "Фоны", model: "3D-модели", ship_data: "Паспорта кораблей" };
export const STATE_NAMES: Record<string, string> = { builtin: "Встроенный", draft: "Есть черновик", replaced: "Заменён" };
export const STATUS_NAMES: Record<string, string> = { draft: "черновик", published: "опубликована", archived: "в архиве" };

export function formatBytes(n: number): string {
  if (n >= 1 << 20) return `${(n / (1 << 20)).toFixed(1)} МБ`;
  if (n >= 1 << 10) return `${Math.round(n / 1024)} КБ`;
  return `${n} Б`;
}
