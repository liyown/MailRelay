import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "./api";

afterEach(() => vi.unstubAllGlobals());

describe("api", () => {
  it("uses same-origin credentials and parses JSON", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ status: "healthy" }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(api.health()).resolves.toEqual({ status: "healthy" });
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/health", expect.objectContaining({ credentials: "same-origin" }));
  });

  it("surfaces safe API errors", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { code: "unauthorized", message: "Authentication required" } }), { status: 401, headers: { "Content-Type": "application/json" } })));
    await expect(api.session()).rejects.toMatchObject({ status: 401, code: "unauthorized" });
  });

  it("replays a dead job with a POST carrying the CSRF token", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ status: "ok" }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(api.replayJob(7, "csrf-token")).resolves.toEqual({ status: "ok" });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/jobs/7/replay",
      expect.objectContaining({ method: "POST", headers: expect.objectContaining({ "X-CSRF-Token": "csrf-token" }) }),
    );
  });

  it("saves the config draft with a PUT carrying JSON and the CSRF token", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ status: "ok" }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);
    const draft = { commands: [{ name: "deploy", handler: "http" }], http_hosts: ["api.example.com"], catalog_notify: [], token: "test-token", allow: ["user@example.com"] };
    await expect(api.saveConfig(draft, "csrf-token")).resolves.toEqual({ status: "ok" });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/config/draft");
    expect(init).toMatchObject({ method: "PUT", headers: expect.objectContaining({ "X-CSRF-Token": "csrf-token", "Content-Type": "application/json" }) });
    expect(JSON.parse(init.body)).toEqual(draft);
  });

  it("carries field-level validation errors on APIError", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { code: "invalid_config", message: "bad", fields: { name: "required" } } }), { status: 422, headers: { "Content-Type": "application/json" } })));
    await expect(api.saveConfig({ commands: [], http_hosts: [], catalog_notify: [], token: "", allow: [] }, "t")).rejects.toMatchObject({ status: 422, code: "invalid_config", fields: { name: "required" } });
  });

  it("encodes pagination cursors and filters into the query string", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ items: [] }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);
    await api.executions({ limit: 50, status: "error", cursor: "abc123" });
    const url = fetchMock.mock.calls[0][0] as string;
    expect(url).toContain("/api/v1/executions?");
    expect(url).toContain("limit=50");
    expect(url).toContain("status=error");
    expect(url).toContain("cursor=abc123");
  });
});
