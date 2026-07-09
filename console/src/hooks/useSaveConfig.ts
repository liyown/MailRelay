import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api, type ConfigDraft } from "@/lib/api";

// useSaveConfig persists the whole editable config draft (commands, outbound host
// allowlist, catalog-notify list). It sends the CSRF token and, on success,
// invalidates every view derived from the command catalog.
export function useSaveConfig(csrf: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (draft: ConfigDraft) => api.saveConfig(draft, csrf),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["config-draft"] });
      queryClient.invalidateQueries({ queryKey: ["commands"] });
      queryClient.invalidateQueries({ queryKey: ["system"] });
    },
  });
}
