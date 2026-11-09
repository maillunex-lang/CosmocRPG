import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DesignAdmin } from "./DesignAdmin";

vi.mock("./ModelPreview", () => ({ ModelPreview: ({ url }: { url: string }) => <div data-testid="model3d">{url}</div> }));

const rep = { ok: true, errors: [], warnings: ["много треугольников"], info: { triangles: 1200 } };
const asset = { id: 7, element_id: "model:COR-001:a", version: 1, file_name: "bars.glb", mime: "model/gltf-binary", sha256: "a".repeat(64), size: 123456, status: "draft", report: rep, notes: "первая", uploaded_by: "owner", created_at: "2026-11-09T10:00:00Z" };
const published = { ...asset, id: 5, version: 3, status: "published", mime: "image/svg+xml", file_name: "g.svg", element_id: "icon:gauge" };
const base = { tags: ["корабль"], builtin: "res://assets/x", bytes: 1000, drafts: 0, versions: 0 };
const items = [
  { ...base, id: "icon:gauge", category: "icon", name: "gauge", group: "Иконки интерфейса", state: "replaced", published, versions: 1, preview_svg: "<svg/>" },
  { ...base, id: "model:COR-001:a", category: "model", name: "Барс — Временная окраска", group: "3D-модели кораблей", state: "builtin", placeholder: true, ship: "COR-001", skin: "a", model_length: 1.9, nose_axis: "-x" },
];
const facets = { categories: { icon: 28, model: 14 }, groups: { "Иконки интерфейса": 28, "3D-модели кораблей": 14 }, states: { builtin: 56, replaced: 1 }, tags: { корабль: 14, заглушка: 12 } };
const status = { can_publish: true, manifest_version: 4, published: 1, elements: 57, public_key: "KEY" };

type Reply = { ok?: boolean; status?: number; body: unknown };
function route(map: Record<string, Reply | ((url: string, init?: RequestInit) => Reply)>) {
  const f = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
    const key = `${init?.method ?? "GET"} ${url.replace(/^.*\/api/, "")}`;
    const hit = Object.keys(map).find((k) => key.startsWith(k));
    const r = hit ? (typeof map[hit] === "function" ? (map[hit] as (u: string, i?: RequestInit) => Reply)(url, init) : (map[hit] as Reply)) : { body: {} };
    return Promise.resolve({ ok: r.ok ?? true, status: r.status ?? 200, json: async () => r.body, clone() { return this; }, headers: new Headers() });
  });
  vi.stubGlobal("fetch", f);
  return f;
}
const common = (extra: Record<string, Reply | ((u: string, i?: RequestInit) => Reply)> = {}) => ({
  "GET /design/status": { body: status },
  "GET /design/elements/": { body: { element: items[1], assets: [asset] } },
  "GET /design/elements": { body: { items, facets } },
  ...extra,
});
afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

describe("DesignAdmin", () => {
  it("показывает состояние, каталог, счётчики фильтров и метку заглушки", async () => {
    route(common());
    render(<DesignAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    expect(await screen.findByText("Барс — Временная окраска")).toBeTruthy();
    expect(screen.getByText("заглушка")).toBeTruthy();
    expect(await screen.findByText(/Публикация включена/)).toBeTruthy();
    expect(screen.getByText(/Версия набора ресурсов: 4/)).toBeTruthy();
    expect(screen.getByText("Найдено: 2")).toBeTruthy();
    expect(screen.getByRole("option", { name: "3D-модели (14)" })).toBeTruthy();
    expect(screen.getByRole("option", { name: "Заменён (1)" })).toBeTruthy();
  });

  it("фильтры и поиск уходят в запрос списка", async () => {
    const f = route(common());
    render(<DesignAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    await screen.findByText("Барс — Временная окраска");
    await userEvent.selectOptions(screen.getByLabelText("Категория"), "model");
    await userEvent.type(screen.getByLabelText("Поиск"), "барс");
    await waitFor(() => expect(f.mock.calls.some((c) => String(c[0]).includes("/design/elements?") && String(c[0]).includes("category=model") && String(c[0]).includes("q=%D0%B1%D0%B0%D1%80%D1%81"))).toBe(true));
  });

  it("у КАЖДОГО элемента есть кнопка «Скопировать промпт»; текст копируется и показывается", async () => {
    const write = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText: write } });
    const f = route(common({ "GET /design/prompt": (u) => ({ body: { prompt: `ПРОМПТ для ${decodeURIComponent(u.split("element=")[1])}` } }) }));
    render(<DesignAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    await screen.findByText("Барс — Временная окраска");
    expect(screen.getAllByRole("button", { name: /Скопировать промпт:/ })).toHaveLength(items.length);
    await userEvent.click(screen.getByRole("button", { name: "Скопировать промпт: Барс — Временная окраска" }));
    await waitFor(() => expect(write).toHaveBeenCalledWith("ПРОМПТ для model:COR-001:a"));
    expect((screen.getByLabelText("Текст промпта") as HTMLTextAreaElement).value).toContain("model:COR-001:a");
    expect(screen.getByText("Промпт скопирован в буфер обмена")).toBeTruthy();
    expect(f.mock.calls.some((c) => String(c[0]).includes("/design/prompt?element=model%3ACOR-001%3Aa"))).toBe(true);
  });

  it("если буфер обмена недоступен, промпт остаётся в поле для ручного копирования", async () => {
    vi.stubGlobal("navigator", { clipboard: { writeText: vi.fn().mockRejectedValue(new Error("нет")) } });
    route(common({ "GET /design/prompt": { body: { prompt: "ТЕКСТ" } } }));
    render(<DesignAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    await userEvent.click(await screen.findByRole("button", { name: "Скопировать промпт: gauge" }));
    expect(await screen.findByText(/Не удалось скопировать автоматически/)).toBeTruthy();
    expect((screen.getByLabelText("Текст промпта") as HTMLTextAreaElement).value).toBe("ТЕКСТ");
  });

  it("загрузка отправляет файл с именем и заметкой и показывает отчёт проверки", async () => {
    const f = route(common({ "POST /design/elements/": { ok: true, status: 201, body: asset } }));
    render(<DesignAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    await userEvent.click(await screen.findByRole("button", { name: "Открыть: Барс — Временная окраска" }));
    await screen.findByText("Загрузить замену");
    await userEvent.type(screen.getByLabelText("Заметка к версии"), "тест");
    const file = new File([new Uint8Array([1, 2, 3])], "bars.glb", { type: "model/gltf-binary" });
    fireEvent.change(screen.getByLabelText("Файл замены"), { target: { files: [file] } });
    await waitFor(() => expect(screen.getByLabelText("Отчёт проверки файла")).toBeTruthy());
    const post = f.mock.calls.find((c) => (c[1] as RequestInit | undefined)?.method === "POST");
    expect(String(post?.[0])).toContain("/design/elements/model%3ACOR-001%3Aa/upload?name=bars.glb&notes=%D1%82%D0%B5%D1%81%D1%82");
    expect((post?.[1] as RequestInit).body).toBe(file);
    expect(screen.getByText("Проверка пройдена")).toBeTruthy();
    expect(screen.getAllByText("много треугольников").length).toBeGreaterThan(0);
  });

  it("плохой файл: сообщение сервера и список ошибок из отчёта", async () => {
    route(common({ "POST /design/elements/": { ok: false, status: 422, body: { error: "файл не прошёл проверку: внешний буфер", report: { ok: false, errors: ["буфер 0 ссылается на внешний файл"], warnings: [], info: {} } } } }));
    render(<DesignAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    await userEvent.click(await screen.findByRole("button", { name: "Открыть: Барс — Временная окраска" }));
    await screen.findByText("Загрузить замену");
    fireEvent.change(screen.getByLabelText("Файл замены"), { target: { files: [new File(["x"], "bad.glb")] } });
    expect(await screen.findByText("Файл не прошёл проверку")).toBeTruthy();
    expect(screen.getByText("буфер 0 ссылается на внешний файл")).toBeTruthy();
    expect(screen.getByRole("alert").textContent).toContain("внешний буфер");
  });

  it("история: публикация, 3D-предпросмотр, удаление; без ключа публикация недоступна", async () => {
    const f = route(common({ "POST /design/assets/": { body: { manifest_version: 5 } }, "DELETE /design/assets/": { body: { ok: true } } }));
    render(<DesignAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    await userEvent.click(await screen.findByRole("button", { name: "Открыть: Барс — Временная окраска" }));
    const table = await screen.findByRole("table", { name: "История замен" });
    await userEvent.click(within(table).getByText("Показать 3D"));
    expect((await screen.findByTestId("model3d")).textContent).toContain("/design/assets/7/file");
    await userEvent.click(within(table).getByText("Опубликовать"));
    await waitFor(() => expect(screen.getByText(/Версия 1 опубликована/)).toBeTruthy());
    expect(f.mock.calls.some((c) => String(c[0]).endsWith("/design/assets/7/publish") && (c[1] as RequestInit).method === "POST")).toBe(true);
    await userEvent.click(within(table).getByText("Удалить"));
    await waitFor(() => expect(f.mock.calls.some((c) => (c[1] as RequestInit | undefined)?.method === "DELETE")).toBe(true));
  });

  it("без ключа подписи кнопка «Опубликовать» отключена и показано предупреждение", async () => {
    route(common({ "GET /design/status": { body: { ...status, can_publish: false } } }));
    render(<DesignAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    expect(await screen.findByText(/Публикация выключена/)).toBeTruthy();
    await userEvent.click(await screen.findByRole("button", { name: "Открыть: Барс — Временная окраска" }));
    const table = await screen.findByRole("table", { name: "История замен" });
    expect((within(table).getByText("Опубликовать") as HTMLButtonElement).disabled).toBe(true);
  });

  it("у заменённого элемента есть «Снять с публикации»", async () => {
    const f = route(common({
      "GET /design/elements/": { body: { element: items[0], assets: [published] } },
      "POST /design/elements/": { body: { manifest_version: 6 } },
    }));
    render(<DesignAdmin username="a" onBack={() => {}} onLoggedOut={() => {}} />);
    await userEvent.click(await screen.findByRole("button", { name: "Открыть: gauge" }));
    await userEvent.click(await screen.findByText("Снять с публикации"));
    await waitFor(() => expect(screen.getByText(/снова встроенный элемент/)).toBeTruthy());
    expect(f.mock.calls.some((c) => String(c[0]).includes("/unpublish"))).toBe(true);
  });
});
