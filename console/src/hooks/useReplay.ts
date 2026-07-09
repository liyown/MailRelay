import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";

type ReplayKind = "job" | "reply";

// useReplay re-queues a dead-letter job or reply. It sends the CSRF token and,
// on success, invalidates the affected lists plus the dashboard counters.
export function useReplay(kind: ReplayKind, csrf: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => (kind === "job" ? api.replayJob(id, csrf) : api.replayReply(id, csrf)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["jobs"] });
      queryClient.invalidateQueries({ queryKey: ["replies"] });
      queryClient.invalidateQueries({ queryKey: ["dashboard"] });
    },
  });
}
