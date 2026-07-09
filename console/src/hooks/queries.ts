import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { api, type ListParams } from "@/lib/api";

const REFRESH = { dashboard: 15_000, queue: 10_000, logs: 10_000 } as const;

export function useSession() {
  return useQuery({ queryKey: ["session"], queryFn: api.session, retry: false });
}

export function useDashboard(range: string) {
  return useQuery({ queryKey: ["dashboard", range], queryFn: () => api.dashboard(range), refetchInterval: REFRESH.dashboard });
}

export function useCommands(enabled = true) {
  return useQuery({ queryKey: ["commands"], queryFn: api.commands, enabled });
}

export function useConfigDraft(enabled = true) {
  return useQuery({ queryKey: ["config-draft"], queryFn: api.configDraft, enabled });
}

export function useSystem() {
  return useQuery({ queryKey: ["system"], queryFn: api.system });
}

type Fetcher<T> = (params: ListParams) => Promise<{ items: T[]; next_cursor?: string }>;

function makeInfinite<T>(key: unknown[], fetcher: Fetcher<T>, filters: ListParams, refetchInterval?: number) {
  return useInfiniteQuery({
    queryKey: [...key, filters],
    queryFn: ({ pageParam }) => fetcher({ ...filters, limit: 50, cursor: pageParam || undefined }),
    initialPageParam: "",
    getNextPageParam: (last) => last.next_cursor || undefined,
    refetchInterval,
  });
}

export function useExecutions(filters: { status?: string; command?: string }) {
  return makeInfinite(["executions"], api.executions, filters);
}

export function useJobs() {
  return makeInfinite(["jobs"], api.jobs, {}, REFRESH.queue);
}

export function useReplies() {
  return makeInfinite(["replies"], api.replies, {}, REFRESH.queue);
}

export function useEvents(severity?: string) {
  return makeInfinite(["events"], api.events, { severity }, REFRESH.logs);
}

export function flatten<T>(pages?: { items: T[] }[]): T[] {
  return (pages ?? []).flatMap((page) => page.items);
}
