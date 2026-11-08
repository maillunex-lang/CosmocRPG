import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ChangePassword, validateNewPassword } from "./ChangePassword";

function mockFetch(status: number, body: unknown) {
  const f = vi.fn().mockResolvedValue({ ok: status >= 200 && status < 300, status, json: async () => body });
  vi.stubGlobal("fetch", f);
  return f;
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

async function fill(current: string, next: string, repeat: string) {
  await userEvent.click(screen.getByRole("button", { name: "Сменить пароль" }));
  await userEvent.type(screen.getByLabelText("Текущий пароль"), current);
  await userEvent.type(screen.getByLabelText(/^Новый пароль/), next);
  await userEvent.type(screen.getByLabelText("Повторите новый пароль"), repeat);
  await userEvent.click(screen.getByRole("button", { name: "Сохранить" }));
}

describe("validateNewPassword", () => {
  it("проверяет длину, совпадение и отличие от текущего", () => {
    expect(validateNewPassword("old-password-1", "short", "short")).toMatch(/короче 12/);
    expect(validateNewPassword("old-password-1", "новый-пароль-123", "другой-пароль-123")).toMatch(/не совпадают/);
    expect(validateNewPassword("same-password-12", "same-password-12", "same-password-12")).toMatch(/совпадает с текущим/);
    expect(validateNewPassword("old-password-1", "новый-пароль-123", "новый-пароль-123")).toBe("");
  });

  it("считает длину в символах, а не в байтах", () => {
    // 12 русских букв = 24 байта в UTF-8, но ровно 12 символов
    expect(validateNewPassword("x", "абвгдежзийкл", "абвгдежзийкл")).toBe("");
    expect(validateNewPassword("x", "абвгдежзийк", "абвгдежзийк")).toMatch(/короче 12/);
  });
});

describe("ChangePassword", () => {
  it("форма скрыта, пока не нажата кнопка", () => {
    mockFetch(200, {});
    render(<ChangePassword />);
    expect(screen.queryByLabelText("Текущий пароль")).toBeNull();
    expect(screen.getByRole("button", { name: "Сменить пароль" })).toBeTruthy();
  });

  it("несовпадающие пароли не отправляются на сервер", async () => {
    const f = mockFetch(200, {});
    render(<ChangePassword />);
    await fill("old-password-1", "новый-пароль-123", "другой-пароль-123");
    expect((await screen.findByRole("alert")).textContent).toBe("Новые пароли не совпадают");
    expect(f).not.toHaveBeenCalled();
  });

  it("успех: отправляет current/new, закрывает форму и сообщает о завершённых сессиях", async () => {
    const f = mockFetch(200, { status: "ok", other_sessions_revoked: 2 });
    render(<ChangePassword />);
    await fill("old-password-1", "новый-пароль-123", "новый-пароль-123");
    expect((await screen.findByRole("status")).textContent).toBe("Пароль изменён. Завершено других сессий: 2.");
    const [url, init] = f.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/admin/api/password");
    expect(init.body).toBe(JSON.stringify({ current: "old-password-1", new: "новый-пароль-123" }));
    expect(screen.queryByLabelText("Текущий пароль")).toBeNull();
    expect(screen.queryByDisplayValue("новый-пароль-123")).toBeNull(); // пароли не остаются в полях
  });

  it("ошибка сервера (неверный текущий пароль) показывается, форма остаётся открытой", async () => {
    mockFetch(403, { error: "неверный текущий пароль" });
    render(<ChangePassword />);
    await fill("wrong-password-1", "новый-пароль-123", "новый-пароль-123");
    expect((await screen.findByRole("alert")).textContent).toBe("неверный текущий пароль");
    expect(screen.getByLabelText("Текущий пароль")).toBeTruthy();
  });
});
