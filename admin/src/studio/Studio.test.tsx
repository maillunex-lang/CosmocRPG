import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Studio } from "./Studio";
import { ImportPanel } from "./ImportPanel";

const dash = {
  captains: 300, target: 300, by_source: { generator: 290, chatgpt: 10, manual: 0 }, active_version: 3, versions: [], imports: [], sessions: [],
  last_tests: null, validation_errors: 0, quality: {}, statuses: {},
};

function route(map: Record<string, unknown>) {
  const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
    const key = `${init?.method ?? "GET"} ${url.replace(/^.*\/api/, "")}`;
    const hit = Object.keys(map).find((k) => key.startsWith(k));
    const body = hit ? map[hit] : {};
    return Promise.resolve({ ok: true, status: 200, json: async () => body, clone() { return this; }, headers: new Headers() });
  });
  vi.stubGlobal("fetch", f);
  return f;
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("Studio", () => {
  it("показывает все разделы и сводку", async () => {
    route({ "GET /ai/dashboard": dash });
    render(<Studio username="admin" onBack={() => {}} onLoggedOut={() => {}} />);
    for (const t of ["Обзор", "Капитаны", "Диалоги", "ZIP", "Академия", "ChatGPT"]) {
      expect(screen.getByRole("button", { name: t })).toBeTruthy();
    }
    expect(await screen.findByText(/Капитаны: 300 из 300/)).toBeTruthy();
  });

  it("на вкладке ChatGPT копирует промпт выбранной партии и показывает инструкцию", async () => {
    const f = route({ "GET /ai/dashboard": dash, "GET /ai/prompt": { prompt: "ПРОМПТ AI_Civilization_Pack_v1" } });
    const write = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText: write }, configurable: true });
    Object.defineProperty(window, "isSecureContext", { value: true, configurable: true });
    render(<Studio username="admin" onBack={() => {}} onLoggedOut={() => {}} />);
    await userEvent.click(screen.getByRole("button", { name: "ChatGPT" }));
    expect(await screen.findByText(/Как обучить 500 капитанов/)).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Скопировать промпт обучения ChatGPT" }));
    await waitFor(() => expect(write).toHaveBeenCalledWith("ПРОМПТ AI_Civilization_Pack_v1"));
    expect(f.mock.calls.some((c) => String(c[0]).includes("/ai/prompt?batch=25"))).toBe(true);
  });
});

describe("ImportPanel", () => {
  const bad = {
    id: 5, created_at: "2026-11-08T10:00:00Z", admin: "a", filename: "x.zip", sha256: "ab".repeat(32), status: "rejected", pack_id: "p", kind: "partial",
    gate_ok: false, version_id: null, gate: [], plan: { captains: [], sections: [], added: 0, changed: 0, unchanged: 0, conflicts: 0 },
    report: { ok: false, errors: [{ level: "error", code: "integrity", path: "captains/cpt-0001.json", message: "контрольная сумма не совпадает" }], warnings: [], stats: { files: 3 }, pack_id: "p", kind: "partial", integrity: "failed" },
  };
  it("показывает ошибки и блокирует публикацию отклонённого пакета", async () => {
    route({ "POST /ai/zip/import": bad });
    render(<ImportPanel />);
    const input = screen.getByLabelText("ZIP-файл") as HTMLInputElement;
    fireEvent.change(input, { target: { files: [new File(["x"], "x.zip", { type: "application/zip" })] } });
    await userEvent.click(screen.getByRole("button", { name: "Проверить" }));
    expect(await screen.findByText(/контрольная сумма не совпадает/)).toBeTruthy();
    expect(screen.getByText(/НАРУШЕНА/)).toBeTruthy();
    expect((screen.getByRole("button", { name: "Подтвердить публикацию" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("для проверенного пакета требует принять предупреждения и публикует", async () => {
    const ok = { ...bad, status: "validated", gate_ok: true,
      plan: { captains: [{ id: "cpt-0001", name: "Имя", kind: "changed", fields: ["bio"] }], sections: [], added: 0, changed: 1, unchanged: 0, conflicts: 0 },
      report: { ...bad.report, ok: true, errors: [], warnings: [{ level: "warning", code: "integrity", path: "r", message: "предупреждение" }], integrity: "partial" } };
    const f = route({ "POST /ai/zip/import": ok, "POST /ai/imports/5/publish": { version: 7 } });
    render(<ImportPanel />);
    fireEvent.change(screen.getByLabelText("ZIP-файл"), { target: { files: [new File(["x"], "x.zip", { type: "application/zip" })] } });
    await userEvent.click(screen.getByRole("button", { name: "Проверить" }));
    expect(await screen.findByText(/изменится 1/)).toBeTruthy();
    const publish = screen.getByRole("button", { name: "Подтвердить публикацию" }) as HTMLButtonElement;
    expect(publish.disabled).toBe(true);
    await userEvent.click(screen.getByLabelText("принимаю предупреждения"));
    expect(publish.disabled).toBe(false);
    await userEvent.click(publish);
    expect(await screen.findByText(/новая версия 7/)).toBeTruthy();
    expect(f.mock.calls.some((c) => String(c[0]).includes("/ai/imports/5/publish"))).toBe(true);
  });
});
