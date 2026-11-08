import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { WorldAdmin } from "./WorldAdmin";

const state = (enabled = true) => ({
  version: 2, config: { systems: [{ id: "xi" }] }, living_enabled: enabled,
  versions: [{ id: 2, created_at: "2026-11-08T10:00:00Z", admin: "Lunex", note: "правка", source: "edit", active: true }, { id: 1, created_at: "2026-11-08T09:00:00Z", admin: "система", note: "Начальная", source: "seed", active: false }],
  preview: { ok: true, errors: [], systems: [{ id: "xi", name: "Система Кси", bodies: 5, stations: 3, zones: 20, deposits: 30, spawns: 20, points: 8 }] },
});

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

describe("WorldAdmin", () => {
  it("показывает версии и сводку по системам", async () => {
    route({ "GET /world": state() });
    render(<WorldAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    expect(await screen.findByText(/Конфигурация \(версия 2\)/)).toBeTruthy();
    expect(screen.getByText("Проверка пройдена")).toBeTruthy();
    expect(screen.getByText("Система Кси (xi)")).toBeTruthy();
    expect(screen.getByText("Начальная")).toBeTruthy();
  });

  it("предупреждает, если живые локации выключены флагом", async () => {
    route({ "GET /world": state(false) });
    render(<WorldAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    expect(await screen.findByText(/выключены \(COSMOC_FEATURE_LIVING\)/)).toBeTruthy();
  });

  it("публикация доступна только после успешной проверки текущего текста", async () => {
    const f = route({ "GET /world": state(), "POST /world/preview": { ok: true, errors: [], systems: [] }, "PUT /world": { version: 3, living_enabled: true } });
    render(<WorldAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    const area = (await screen.findByLabelText("Конфигурация мира (JSON)")) as HTMLTextAreaElement;
    const publish = screen.getByText("Опубликовать") as HTMLButtonElement;
    expect(publish.disabled).toBe(false); // загруженная версия уже проверена
    fireEvent.change(area, { target: { value: '{"systems":[]}' } });
    expect(publish.disabled).toBe(true);
    expect(screen.getByText(/проверьте заново/)).toBeTruthy();
    await userEvent.click(screen.getByText("Проверить"));
    await waitFor(() => expect(publish.disabled).toBe(false));
    await userEvent.click(publish);
    expect(await screen.findByText(/Опубликована версия 3/)).toBeTruthy();
    const put = f.mock.calls.find((c) => (c[1] as RequestInit | undefined)?.method === "PUT");
    expect(JSON.parse(String((put?.[1] as RequestInit).body))).toMatchObject({ base_version: 2, config: { systems: [] } });
  });

  it("битый JSON не отправляется на проверку, ошибки сервера показываются", async () => {
    const f = route({ "GET /world": state(), "POST /world/preview": { ok: false, errors: ["system xi: radius вне диапазона"], systems: [] } });
    render(<WorldAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    const area = await screen.findByLabelText("Конфигурация мира (JSON)");
    fireEvent.change(area, { target: { value: "{не json" } });
    await userEvent.click(screen.getByText("Проверить"));
    expect(await screen.findByText("Это не корректный JSON")).toBeTruthy();
    expect(f.mock.calls.some((c) => String(c[0]).includes("/world/preview"))).toBe(false);
    fireEvent.change(area, { target: { value: "{}" } });
    await userEvent.click(screen.getByText("Проверить"));
    expect(await screen.findByText(/radius вне диапазона/)).toBeTruthy();
    expect((screen.getByText("Опубликовать") as HTMLButtonElement).disabled).toBe(true);
  });

  it("откат запрашивает подтверждение", async () => {
    const f = route({ "GET /world": state(), "POST /world/versions/1/rollback": { version: 3 } });
    vi.stubGlobal("confirm", () => true);
    render(<WorldAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    await screen.findByText("Начальная");
    await userEvent.click(screen.getByText("Откатить"));
    expect(await screen.findByText(/Откат к версии 1 выполнен/)).toBeTruthy();
    expect(f.mock.calls.some((c) => String(c[0]).includes("/world/versions/1/rollback"))).toBe(true);
  });
});
