import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

const renderApp = () => render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><App /></QueryClientProvider>);

afterEach(() => vi.unstubAllGlobals());

describe("App", () => {
  it("renders the authenticated MailRelay operations identity and primary navigation", async () => {
    vi.stubGlobal("fetch", vi.fn().mockImplementation((input: string) => {
      const body = input.includes("/session")
        ? { user: { id: "admin", name: "平台管理员" }, csrf: "token" }
        : input.includes("/dashboard")
          ? { range: "24h", execution_count: 42, success_count: 41, success_rate: 97.62, p95_duration_ms: 2380, active_handlers: 3, queue: { pending: 2, running: 1, dead: 0 }, replies: { pending: 1, running: 0, dead: 0 }, recent_executions: [], recent_events: [] }
          : { items: [] };
      return Promise.resolve(new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } }));
    }));
    renderApp();
    expect(screen.getAllByLabelText("MailRelay").length).toBeGreaterThan(0);
    expect(await screen.findByRole("heading", { name: "仪表盘" })).toBeInTheDocument();
    expect(await screen.findByText("42")).toBeInTheDocument();
  });

  it("shows the login form when no session exists", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { code: "unauthorized", message: "Authentication required" } }), { status: 401, headers: { "Content-Type": "application/json" } })));
    renderApp();
    expect(await screen.findByRole("heading", { name: "登录运行控制台" })).toBeInTheDocument();
    expect(screen.getByLabelText("管理员密码")).toBeInTheDocument();
  });
});
