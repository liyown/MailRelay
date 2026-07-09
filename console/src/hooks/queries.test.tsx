import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { flatten, useJobs } from "./queries";

afterEach(() => vi.unstubAllGlobals());

function wrapper() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

describe("useJobs pagination", () => {
  it("appends the next page when fetchNextPage is called", async () => {
    const page = (body: unknown) => new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(page({ items: [{ id: 1, command: "a", status: "dead", attempts: 3, max_attempts: 3, available_at: "2026-07-08T08:00:00Z" }], next_cursor: "c1" }))
      .mockResolvedValueOnce(page({ items: [{ id: 2, command: "b", status: "pending", attempts: 0, max_attempts: 3, available_at: "2026-07-08T08:01:00Z" }] }));
    vi.stubGlobal("fetch", fetchMock);

    const { result } = renderHook(() => useJobs(), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(flatten(result.current.data?.pages)).toHaveLength(1);
    expect(result.current.hasNextPage).toBe(true);

    result.current.fetchNextPage();
    await waitFor(() => expect(flatten(result.current.data?.pages)).toHaveLength(2));
    expect(flatten(result.current.data?.pages).map((job) => job.id)).toEqual([1, 2]);
    expect(result.current.hasNextPage).toBe(false);
  });
});
