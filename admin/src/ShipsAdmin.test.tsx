import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ShipsAdmin } from "./ShipsAdmin";

const ship = (id: string, temporary: boolean) => ({
  id, name: id === "FTR-001" ? "Стриж" : "Грузовик", class: "Класс", role: "", temporary, specs: ["a"], default_spec: "a", skins: ["a"], dimensions_m: [1, 1, 1], crew: 1, engines: 2, weapons: "",
  hull: 1200, shield: 600, armor: 120, energy: 100, cargo: 8, speed: 14, shield_regen: 1, shield_regen_delay: 1, energy_regen: 1, abilities: {},
});
const list = { version: 3, ships: [ship("FTR-001", false), ship("TMP-HAUL", true)], starters: { trader: { ship: "TMP-HAUL", spec: "hauler", skin: "a" } }, versions: [], pilot_specs: [] };

function route(map: Record<string, unknown>) {
  const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
    const key = `${init?.method ?? "GET"} ${url.replace(/^.*\/api/, "")}`;
    const hit = Object.keys(map).find((k) => key.startsWith(k));
    return Promise.resolve({ ok: true, status: 200, json: async () => (hit ? map[hit] : {}), clone() { return this; }, headers: new Headers() });
  });
  vi.stubGlobal("fetch", f);
  return f;
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("ShipsAdmin", () => {
  it("показывает каталог, помечает временные модели и открывает редактор", async () => {
    route({ "GET /ships": list });
    render(<ShipsAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    expect(await screen.findByText("Стриж")).toBeTruthy();
    expect(screen.getByText("временная модель")).toBeTruthy();
    expect(screen.getByText(/Каталог \(версия 3\)/)).toBeTruthy();
    await userEvent.click(screen.getByText("Стриж"));
    expect(await screen.findByLabelText("Паспорт корабля (JSON)")).toBeTruthy();
  });

  it("сохраняет правку и сообщает, что она применена в игре; битый JSON не отправляется", async () => {
    const f = route({ "GET /ships": list, "PUT /ships/FTR-001": { version: 4 } });
    render(<ShipsAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    await userEvent.click(await screen.findByText("Стриж"));
    const area = (await screen.findByLabelText("Паспорт корабля (JSON)")) as HTMLTextAreaElement;
    fireEvent.change(area, { target: { value: "{не json" } });
    await userEvent.click(screen.getByRole("button", { name: "Проверить и сохранить" }));
    expect(await screen.findByText("Это не корректный JSON")).toBeTruthy();
    expect(f.mock.calls.some((c) => (c[1] as RequestInit | undefined)?.method === "PUT")).toBe(false);
    fireEvent.change(area, { target: { value: JSON.stringify(ship("FTR-001", false)) } });
    await userEvent.click(screen.getByRole("button", { name: "Проверить и сохранить" }));
    expect(await screen.findByText(/Сохранено, версия 4/)).toBeTruthy();
  });

  it("патч с ошибками нельзя опубликовать; корректный показывает изменения параметров", async () => {
    const bad = { id: "p1", filename: "x.zip", sha256: "ab".repeat(32), status: "rejected", pack_id: "", integrity: "failed", errors: [{ code: "integrity", path: "ships/FTR-001.json", message: "сумма не совпадает" }], warnings: [], changes: [], base_version: 3 };
    const ok = { ...bad, id: "p2", status: "ready", integrity: "verified", errors: [], changes: [{ id: "FTR-001", kind: "changed", fields: [{ path: "speed", old: "14", new: "16.5" }] }] };
    let next: unknown = bad;
    const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${url.replace(/^.*\/api/, "")}`;
      const body = key.startsWith("GET /ships") ? list : key.startsWith("POST /ships/patch/import") ? next : key.includes("/publish") ? { version: 5 } : {};
      return Promise.resolve({ ok: true, status: 200, json: async () => body, clone() { return this; }, headers: new Headers() });
    });
    vi.stubGlobal("fetch", f);
    render(<ShipsAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    await screen.findByText("Стриж");
    const input = screen.getByLabelText("ZIP-патч");
    fireEvent.change(input, { target: { files: [new File(["x"], "x.zip", { type: "application/zip" })] } });
    expect(await screen.findByText(/сумма не совпадает/)).toBeTruthy();
    expect((screen.getByRole("button", { name: "Подтвердить публикацию" }) as HTMLButtonElement).disabled).toBe(true);
    next = ok;
    fireEvent.change(input, { target: { files: [new File(["y"], "y.zip", { type: "application/zip" })] } });
    expect(await screen.findByText("16.5")).toBeTruthy();
    const pub = screen.getByRole("button", { name: "Подтвердить публикацию" }) as HTMLButtonElement;
    await waitFor(() => expect(pub.disabled).toBe(false));
    await userEvent.click(pub);
    expect(await screen.findByText(/Патч опубликован, версия 5/)).toBeTruthy();
  });
});
