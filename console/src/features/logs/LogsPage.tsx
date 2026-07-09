import { useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ArrowUp, Pulse } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { PageFrame } from "@/components/layout/PageFrame";
import { DataState } from "@/components/common/DataState";
import { useEvents, flatten } from "@/hooks/queries";
import { formatClock } from "@/lib/format";
import { cn } from "@/lib/utils";

const FILTERS = [
  { label: "全部", value: undefined },
  { label: "仅错误", value: "error" },
] as const;

type Severity = (typeof FILTERS)[number]["value"];

export function LogsPage() {
  const [severity, setSeverity] = useState<Severity>(undefined);
  const events = useEvents(severity);
  const items = flatten(events.data?.pages);
  const scrollRef = useRef<HTMLDivElement>(null);
  const queryClient = useQueryClient();

  function handleRefresh() {
    // Reset to first page (clears accumulated pages) then refetch
    queryClient.resetQueries({ queryKey: ["events"] });
    scrollRef.current?.scrollTo({ top: 0, behavior: "smooth" });
  }

  function scrollToTop() {
    scrollRef.current?.scrollTo({ top: 0, behavior: "smooth" });
  }

  return (
    <PageFrame
      title="运行日志"
      description="经过脱敏的运行事件与关联上下文"
      action={
        <Button variant="outline" size="sm" onClick={handleRefresh}>
          <Pulse />
          刷新至最新
        </Button>
      }
    >
      <Card className="overflow-hidden shadow-none">
        {/* Toolbar */}
        <div className="flex items-center gap-3 border-b border-border px-4 py-2.5">
          <span className="font-mono text-xs text-muted-foreground">EVENT STREAM</span>
          {items.length > 0 && (
            <span className="font-mono text-xs text-muted-foreground/60">
              已加载 {items.length} 条{events.hasNextPage ? "，还有更多" : "，已全部加载"}
            </span>
          )}
          <div className="ml-auto flex gap-1 rounded-md border border-border bg-muted p-0.5">
            {FILTERS.map((f) => (
              <button
                key={String(f.value)}
                type="button"
                onClick={() => setSeverity(f.value)}
                className={cn(
                  "rounded px-2.5 py-1 font-mono text-xs transition-colors",
                  severity === f.value
                    ? "bg-card text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                {f.label}
              </button>
            ))}
          </div>
        </div>

        {/* Scrollable log area — capped height so page doesn't grow forever */}
        <div ref={scrollRef} className="max-h-[640px] overflow-y-auto font-mono text-xs leading-6">
          <DataState
            isLoading={events.isPending}
            isError={events.isError}
            isEmpty={items.length === 0}
            emptyText="暂无运行事件"
            errorText="无法读取运行事件"
            rows={6}
          >
            {items.map((item) => (
              <div
                key={item.id}
                className={cn(
                  "border-b border-border/50 px-4 py-2 last:border-0",
                  item.severity === "error" && "bg-destructive/[0.03]",
                )}
              >
                {/* Main row */}
                <div className="flex items-baseline gap-3 min-w-0">
                  <span className="shrink-0 w-[68px] text-muted-foreground/60 tabular-nums">
                    {formatClock(item.at)}
                  </span>
                  <span
                    className={cn(
                      "shrink-0 w-10 font-semibold",
                      item.severity === "error" ? "text-destructive" : "text-emerald-600",
                    )}
                  >
                    {item.severity === "error" ? "ERR" : "INFO"}
                  </span>
                  <span className="shrink-0 w-[72px] text-primary/70 truncate">
                    {item.phase}
                  </span>
                  <span className="min-w-0 flex-1 truncate text-foreground">
                    {item.summary}
                  </span>
                </div>

                {/* Secondary row — only when there's extra context */}
                {(item.command || item.handler || item.error_kind) && (
                  <div className="mt-0.5 ml-[156px] flex flex-wrap items-center gap-x-3 gap-y-0.5 text-muted-foreground">
                    {item.command && (
                      <span>
                        <span className="text-muted-foreground/50">cmd</span>{" "}
                        {item.command}
                      </span>
                    )}
                    {item.handler && (
                      <span>
                        <span className="text-muted-foreground/50">via</span>{" "}
                        {item.handler}
                      </span>
                    )}
                    {item.error_kind && (
                      <Badge
                        variant="outline"
                        className="h-4 border-destructive/30 bg-destructive/5 px-1.5 py-0 text-[10px] text-destructive"
                      >
                        {item.error_kind}
                      </Badge>
                    )}
                  </div>
                )}
              </div>
            ))}
          </DataState>
        </div>

        {/* Footer: load more + scroll to top */}
        {(events.hasNextPage || items.length > 10) && (
          <div className="flex items-center justify-between border-t border-border px-4 py-2.5">
            <span className="font-mono text-xs text-muted-foreground/60">
              {events.hasNextPage ? `已显示 ${items.length} 条` : `共 ${items.length} 条`}
            </span>
            <div className="flex gap-2">
              <Button variant="ghost" size="sm" onClick={scrollToTop} className="h-7 gap-1 text-xs">
                <ArrowUp className="size-3" />
                回到顶部
              </Button>
              {events.hasNextPage && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => events.fetchNextPage()}
                  disabled={events.isFetchingNextPage}
                  className="h-7 text-xs"
                >
                  {events.isFetchingNextPage ? "加载中..." : "加载更多（50 条）"}
                </Button>
              )}
            </div>
          </div>
        )}
      </Card>
    </PageFrame>
  );
}
