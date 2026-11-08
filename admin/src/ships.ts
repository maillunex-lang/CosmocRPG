// Клиент API раздела «Корабли» (каталог, правки, ZIP-патчи, версии).
import { API, ApiError } from "./api";

export interface Ability { name: string; spec: string; damage: number; shots: number; range: number; period: number; energy: number; cooldown: number; duration: number; speed_mul: number; max_dist: number; min_dist: number; damage_mul: number; radius: number; reveal: number }
export interface ShipSpec {
  id: string; name: string; class: string; role: string; temporary: boolean; specs: string[]; default_spec: string; skins: string[]; dimensions_m: number[];
  crew: number; engines: number; weapons: string; hull: number; shield: number; armor: number; energy: number; cargo: number; speed: number;
  shield_regen: number; shield_regen_delay: number; energy_regen: number; abilities: Record<string, Ability>;
}
export interface VersionInfo { id: number; created_at: string; admin: string; note: string; source: string; active: boolean; ships: number }
export interface Issue { code: string; path: string; message: string }
export interface FieldChange { path: string; old: string; new: string }
export interface ShipChange { id: string; kind: string; fields: FieldChange[] }
export interface Preview { id: string; filename: string; sha256: string; status: string; pack_id: string; integrity: string; errors: Issue[]; warnings: Issue[]; changes: ShipChange[]; base_version: number }

async function send(method: string, path: string, body?: unknown, raw?: { data: BodyInit; type: string }): Promise<Response> {
  let res: Response;
  try {
    res = await fetch(`${API}${path}`, {
      method, credentials: "same-origin",
      headers: raw ? { "Content-Type": raw.type } : body === undefined ? undefined : { "Content-Type": "application/json" },
      body: raw ? raw.data : body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, "Нет связи с сервером");
  }
  if (!res.ok) {
    let msg = `Ошибка ${res.status}`;
    try { const d = (await res.clone().json()) as { error?: string }; if (d.error) msg = d.error; } catch { /* не JSON */ }
    throw new ApiError(res.status, msg);
  }
  return res;
}

async function call<T>(method: string, path: string, body?: unknown, raw?: { data: BodyInit; type: string }): Promise<T> {
  const res = await send(method, path, body, raw);
  return (await res.json()) as T;
}

export const shipsApi = {
  list: () => call<{ version: number; ships: ShipSpec[]; starters: Record<string, { ship: string; spec: string; skin: string }>; versions: VersionInfo[]; pilot_specs: string[] }>("GET", "/ships"),
  save: (s: ShipSpec) => call<{ version: number }>("PUT", `/ships/${s.id}`, s),
  importPatch: (file: File) => call<Preview>("POST", `/ships/patch/import?filename=${encodeURIComponent(file.name)}`, undefined, { data: file, type: "application/zip" }),
  publish: (id: string) => call<{ version: number }>("POST", `/ships/patch/${id}/publish`, {}),
  discard: (id: string) => call<{ status: string }>("POST", `/ships/patch/${id}/discard`, {}),
  rollback: (id: number) => call<{ version: number }>("POST", `/ships/versions/${id}/rollback`, {}),
  prompt: () => call<{ prompt: string }>("GET", "/ships/prompt"),
  async exportPatch(ids: string[]): Promise<string> {
    const res = await send("POST", "/ships/patch/export", { ids });
    const blob = await res.blob();
    const m = /filename="([^"]+)"/.exec(res.headers.get("Content-Disposition") ?? "");
    const name = m ? m[1] : "ships-patch.zip";
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url; a.download = name; document.body.appendChild(a); a.click(); a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 5000);
    return name;
  },
};
