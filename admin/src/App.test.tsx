import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { App } from "./App";

type Handler = (method: string, path: string, body?: string) => { status: number; body: unknown };

function mockServer(handler: Handler) {
  const f = vi.fn(async (path: string, init?: RequestInit) => {
    const r = handler(init?.method ?? "GET", path, init?.body as string | undefined);
    return { ok: r.status >= 200 && r.status < 300, status: r.status, json: async () => r.body };
  });
  vi.stubGlobal("fetch", f);
  return f;
}

const status = {
  version: "1.2.3", database: "ok", started_at: "", uptime_seconds: 3700, server_time: "", ws_sessions: 4, boots_total: 9,
};
const audit = { entries: [{ id: 1, at: "2026-11-08T10:00:00Z", username: "admin", action: "login", ip: "203.0.113.7" }] };

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("App", () => {
  it("без сессии показывает форму входа и не обращается к защищённым данным", async () => {
    const f = mockServer(() => ({ status: 401, body: { error: "требуется вход" } }));
    render(<App />);
    expect(await screen.findByRole("button", { name: "Войти" })).toBeTruthy();
    expect(f.mock.calls.map((c) => c[0])).toEqual(["/admin/api/me"]);
  });

  it("неверный пароль показывает ошибку и остаётся на форме", async () => {
    mockServer((m, p) =>
      p === "/admin/api/login" && m === "POST"
        ? { status: 401, body: { error: "неверное имя пользователя или пароль" } }
        : { status: 401, body: { error: "требуется вход" } },
    );
    render(<App />);
    await userEvent.type(await screen.findByLabelText("Имя пользователя"), "admin");
    await userEvent.type(screen.getByLabelText("Пароль"), "wrong");
    await userEvent.click(screen.getByRole("button", { name: "Войти" }));
    expect((await screen.findByRole("alert")).textContent).toBe("неверное имя пользователя или пароль");
    expect(screen.queryByText("Состояние сервера")).toBeNull();
  });

  it("успешный вход открывает панель со статусом и журналом", async () => {
    let loggedIn = false;
    mockServer((_m, p) => {
      if (p === "/admin/api/login") {
        loggedIn = true;
        return { status: 200, body: { username: "admin" } };
      }
      if (!loggedIn) return { status: 401, body: { error: "требуется вход" } };
      if (p === "/admin/api/status") return { status: 200, body: status };
      if (p.startsWith("/admin/api/audit")) return { status: 200, body: audit };
      return { status: 404, body: {} };
    });
    render(<App />);
    await userEvent.type(await screen.findByLabelText("Имя пользователя"), "admin");
    await userEvent.type(screen.getByLabelText("Пароль"), "correct horse battery");
    await userEvent.click(screen.getByRole("button", { name: "Войти" }));
    expect(await screen.findByText("1.2.3")).toBeTruthy();
    expect(screen.getByText("работает")).toBeTruthy();
    expect(screen.getByText("1 ч 1 мин")).toBeTruthy();
    expect(screen.getByText("203.0.113.7")).toBeTruthy();
    // поле пароля после входа не остаётся в DOM
    expect(screen.queryByLabelText("Пароль")).toBeNull();
  });

  it("истёкшая сессия возвращает на форму входа", async () => {
    let calls = 0;
    mockServer((_m, p) => {
      if (p === "/admin/api/me") return { status: 200, body: { username: "admin" } };
      calls++;
      return { status: 401, body: { error: "требуется вход" } };
    });
    render(<App />);
    expect(await screen.findByRole("button", { name: "Войти" })).toBeTruthy();
    expect(calls).toBeGreaterThan(0);
  });

  it("выход возвращает на форму входа", async () => {
    mockServer((m, p) => {
      if (p === "/admin/api/me") return { status: 200, body: { username: "admin" } };
      if (p === "/admin/api/status") return { status: 200, body: status };
      if (p.startsWith("/admin/api/audit")) return { status: 200, body: audit };
      if (p === "/admin/api/logout" && m === "POST") return { status: 200, body: { status: "ok" } };
      return { status: 404, body: {} };
    });
    render(<App />);
    await userEvent.click(await screen.findByRole("button", { name: "Выйти" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Войти" })).toBeTruthy());
  });

  it("недоступность базы показывается в статусе", async () => {
    mockServer((_m, p) => {
      if (p === "/admin/api/me") return { status: 200, body: { username: "admin" } };
      if (p === "/admin/api/status") return { status: 200, body: { ...status, database: "unavailable" } };
      return { status: 200, body: audit };
    });
    render(<App />);
    expect((await screen.findByText("недоступна")).className).toBe("error");
  });
});
