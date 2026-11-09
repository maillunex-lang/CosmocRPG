import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { AiPlayers } from "./AiPlayers";
import { API } from "./api";

const data = {
  status: { enabled: true, paused: false, players: 1, online: 3, target: 22, profiles: 300, by_behavior: { miner: 1 } },
  settings: { enabled: true, paused: false, min_online: 20, max_online: 50, abs_max: 200, players_for_max: 10, spawn_per_min: 12, despawn_per_min: 30, empty_grace_sec: 30, view_radius: 450, shares: { miner: 30 } },
  captains: [],
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
  });

  it("показывает ошибку сервера, а не пустую страницу", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 404, json: async () => ({}), clone() { return this; }, headers: new Headers() }));
    render(<AiPlayers username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    expect(await screen.findByText(/404/)).toBeTruthy();
  });
});
