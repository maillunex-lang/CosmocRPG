import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ChatAdmin } from "./ChatAdmin";

const settings = { enabled: true, retention_days: 7, min_interval_ms: 2000, max_len: 200, history_limit: 50, profanity_filter: true, links_allowed: false, extra_blocked: ["слово"] };
const messages = { items: [{ id: 1, channel: "general", from_id: "p1", from_name: "Ковбой", text: "Привет всем", at: 1794160000000 }, { id: 2, channel: "general", from_id: "ai1", from_name: "Орион-7", from_ai: true, text: "Добрый вечер", at: 1794160001000 }] };

function route(map: Record<string, unknown>) {
  const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
    const key = `${init?.method ?? "GET"} ${url.replace(/^.*\/api/, "")}`;
    const hit = Object.keys(map).find((k) => key.startsWith(k));
    return Promise.resolve({ ok: true, status: 200, json: async () => (hit ? map[hit] : {}), clone() { return this; }, headers: new Headers() });
  });
  vi.stubGlobal("fetch", f);
  return f;
}
afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

describe("ChatAdmin", () => {
  it("показывает журнал с пометкой ИИ, запреты и настройки", async () => {
    route({ "GET /chat/messages": messages, "GET /chat/mutes": { items: [{ player_id: "p9", player_name: "Нарушитель", until: null, reason: "спам", admin: "lunex", created_at: "2026-11-08T10:00:00Z" }] }, "GET /chat/settings": settings });
    render(<ChatAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    expect(await screen.findByText("Привет всем")).toBeTruthy();
    expect(screen.getByText("Орион-7 (ИИ)")).toBeTruthy();
    expect(await screen.findByText("Нарушитель")).toBeTruthy();
    expect(screen.getByText("бессрочно")).toBeTruthy();
    expect((await screen.findByLabelText("Хранить историю, дней")) as HTMLInputElement).toHaveProperty("value", "7");
  });

  it("мьют и бан отправляют верные запросы; кнопки недоступны без игрока", async () => {
    const f = route({ "GET /chat/messages": messages, "GET /chat/mutes": { items: [] }, "GET /chat/settings": settings, "POST /chat/mutes": { player_id: "p1", player_name: "Ковбой", until: null, reason: "", admin: "a", created_at: "2026-11-08T10:00:00Z" } });
    render(<ChatAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    await screen.findByText("Привет всем");
    expect((screen.getByText("Заглушить") as HTMLButtonElement).disabled).toBe(true);
    await userEvent.type(screen.getByLabelText("Игрок (ник или идентификатор)"), "Ковбой");
    await userEvent.click(screen.getByText("Бан в чате (бессрочно)"));
    await waitFor(() => expect(screen.getByText(/бессрочный запрет писать/)).toBeTruthy());
    const post = f.mock.calls.find((c) => (c[1] as RequestInit | undefined)?.method === "POST");
    expect(JSON.parse(String((post?.[1] as RequestInit).body))).toMatchObject({ player: "Ковбой", minutes: 0 });
  });

  it("сохранение настроек отправляет слова по строкам", async () => {
    const f = route({ "GET /chat/messages": messages, "GET /chat/mutes": { items: [] }, "GET /chat/settings": settings, "PUT /chat/settings": settings });
    render(<ChatAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    const area = await screen.findByLabelText(/Дополнительные запрещённые слова/);
    fireEvent.change(area, { target: { value: "один\n\n  два  \n" } });
    await userEvent.click(screen.getByText("Сохранить"));
    await waitFor(() => expect(screen.getByText(/Настройки чата сохранены/)).toBeTruthy());
    const put = f.mock.calls.find((c) => (c[1] as RequestInit | undefined)?.method === "PUT");
    expect(JSON.parse(String((put?.[1] as RequestInit).body)).extra_blocked).toEqual(["один", "два"]);
  });
});
