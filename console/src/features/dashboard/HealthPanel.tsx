import { CheckCircle, Warning } from "@phosphor-icons/react";
import { CardContent } from "@/components/ui/card";
import { Panel, PanelTitle } from "@/components/common/Panel";
import { cn } from "@/lib/utils";
import type { Dashboard } from "@/lib/api";

export function HealthPanel({ data }: { data?: Dashboard }) {
  const deadTotal = (data?.queue.dead ?? 0) + (data?.replies.dead ?? 0);
  const items: [string, string, string, boolean][] = [
    ["处理器", `${data?.active_handlers ?? 0} 活跃`, "", true],
    ["命令执行", `${data?.execution_count ?? 0} 次`, `${data?.success_count ?? 0} 成功`, true],
    ["待处理任务", `${(data?.queue.pending ?? 0) + (data?.replies.pending ?? 0)} 个`, "", true],
    ["死信队列", `${deadTotal} 待处理`, "", deadTotal === 0],
  ];
  return (
    <Panel>
      <PanelTitle>运行健康状态</PanelTitle>
      <CardContent className="grid grid-cols-2 p-0 md:grid-cols-4">
        {items.map(([label, first, second, ok]) => (
          <div key={label} className="border-r border-border p-4 last:border-0">
            <div className="flex items-center gap-2 text-sm font-medium">
              {ok ? (
                <CheckCircle weight="fill" className="size-5 text-emerald-600" />
              ) : (
                <Warning weight="fill" className="size-5 text-amber-500" />
              )}
              {label}
            </div>
            <div className="mt-3 text-sm">{first}</div>
            <div className={cn("mt-1 text-xs", ok ? "text-muted-foreground" : "text-primary")}>{second}</div>
          </div>
        ))}
      </CardContent>
    </Panel>
  );
}
