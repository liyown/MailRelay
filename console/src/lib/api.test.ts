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
});
