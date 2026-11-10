import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { AiPlayers } from "./AiPlayers";
import { API } from "./api";

const data = {
  status: { enabled: true, paused: false, players: 1, online: 1, target: 3, profiles: 500, by_behavior: { miner: 1 } },
  settings: {
    enabled: true, paused: false, min_online: 20, max_online: 50, abs_max: 200, players_for_max: 10,
    spawn_per_min: 12, despawn_per_min: 30, empty_grace_sec: 30, view_radius: 450, shares: { miner: 30 },
    chat_reply_enabled: false, chat_reply_chance: 0, chat_delay_min_ms: 0, chat_delay_max_ms: 0,
    chat_captain_cooldown_sec: 0, chat_max_per_min: 0, chat_ambient_every_sec: 0, chat_follow_chance: 0,
    disabled_captains: [], civ_enabled: true, civ_cast: 3, civ_chat: true, civ_talk_min: 3, civ_talk_max: 15,
    civ_interval_sec: 12, civ_retreat_pct: 30, civ_help: true,
  },
  captains: [
    { id: "cpt-0001", name: "Кира Роан", profession: "разведчик", behavior: "scout", since: "2026-11-10T12:00:00Z" },
    { id: "cpt-0002", name: "Тихий", profession: "шахтёр", behavior: "miner", since: "" },
  ],
};

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

describe("AiPlayers", () => {
  // Регрессия: страница обращалась к /api/…, а админ-API живёт по /admin/api/… (на боевом сайте был 404)
  it("запрашивает данные по адресу админ-API (с префиксом /admin)", async () => {
    const f = vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => data, clone() { return this; }, headers: new Headers() });
    vi.stubGlobal("fetch", f);
    render(<AiPlayers username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    await vi.waitFor(() => expect(f).toHaveBeenCalled());
    const url = String(f.mock.calls[0][0]);
    expect(url).toBe(`${API}/ai-players`);
    expect(url.startsWith("/admin/api/")).toBe(true);
    const online = await screen.findByRole("region", { name: "Кто онлайн" });
    expect(within(online).getByText("Кира Роан")).toBeTruthy();
    expect(within(online).queryByText("Тихий")).toBeNull();
    expect(screen.getByLabelText("Сколько ИИ должно быть")).toBeTruthy();
  });

  it("показывает ошибку сервера, а не пустую страницу", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 404, json: async () => ({}), clone() { return this; }, headers: new Headers() }));
    render(<AiPlayers username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    expect(await screen.findByText(/404/)).toBeTruthy();
  });
});
