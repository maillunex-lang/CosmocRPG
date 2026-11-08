import { afterEach, describe, expect, it, vi } from "vitest";
import { api, ApiError, formatUptime } from "./api";

function mockFetch(status: number, body: unknown) {
  const f = vi.fn().mockResolvedValue({ ok: status >= 200 && status < 300, status, json: async () => body });
  vi.stubGlobal("fetch", f);
  return f;
}

afterEach(() => vi.unstubAllGlobals());

describe("formatUptime", () => {
  it("форматирует минуты, часы и дни", () => {
    expect(formatUptime(90)).toBe("1 мин");
    expect(formatUptime(3600 * 5 + 60 * 7)).toBe("5 ч 7 мин");
    expect(formatUptime(86400 * 2 + 3600)).toBe("2 д 1 ч 0 мин");
    expect(formatUptime(-5)).toBe("0 мин");
  });
});

describe("api", () => {
  it("login шлёт JSON и cookie только своего origin", async () => {
    const f = mockFetch(200, { username: "admin" });
    await expect(api.login("admin", "pw")).resolves.toEqual({ username: "admin" });
    const [url, init] = f.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/admin/api/login");
    expect(init.method).toBe("POST");
    expect(init.credentials).toBe("same-origin");
    expect(init.body).toBe(JSON.stringify({ username: "admin", password: "pw" }));
  });

  it("берёт сообщение об ошибке из ответа сервера", async () => {
    mockFetch(401, { error: "неверное имя пользователя или пароль" });
    await expect(api.login("a", "b")).rejects.toMatchObject({ status: 401, message: "неверное имя пользователя или пароль" });
  });

  it("ошибка без JSON получает общее сообщение", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 502, json: async () => Promise.reject(new Error("no json")) }));
    await expect(api.status()).rejects.toMatchObject({ status: 502, message: "Ошибка 502" });
  });

  it("сетевой сбой превращается в ApiError со статусом 0", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("failed")));
    const err = await api.me().catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 0, message: "Нет связи с сервером" });
  });
});
