import { Pulse } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { PageFrame } from "@/components/layout/PageFrame";
import { DataState } from "@/components/common/DataState";
import { LoadMore } from "@/components/common/LoadMore";
import { useEvents, flatten } from "@/hooks/queries";
import { formatClock } from "@/lib/format";

export function LogsPage() {
  const events = useEvents();
  const items = flatten(events.data?.pages);
  return (
    <PageFrame
      title="运行日志"
      description="经过脱敏的运行事件与关联上下文"
      action={
        <Button variant="outline" onClick={() => events.refetch()}>
          <Pulse />
          刷新
        </Button>
      }
    >
      <Card className="overflow-hidden bg-[#24211f] text-[#eee8df] shadow-none">
        <div className="border-b border-white/10 px-4 py-3 font-mono text-xs text-[#a89e94]">EVENT STREAM · safe projection</div>
        <div className="space-y-3 p-5 font-mono text-xs leading-6">
          <DataState
            isLoading={events.isPending}
            isError={events.isError}
            isEmpty={items.length === 0}
            emptyText="暂无运行事件"
            errorText="无法读取运行事件"
            rows={6}
          >
            {items.map((item) => (
              <div key={item.id} className="grid grid-cols-[72px_56px_70px_1fr] gap-3">
                <span className="text-[#827970]">{formatClock(item.at)}</span>
                <span className={item.severity === "error" ? "text-[#ff8a69]" : "text-[#7bc693]"}>{item.severity.toUpperCase()}</span>
                <span className="text-[#d2ad74]">{item.phase}</span>
                <span>
                  {item.summary}
                  {item.command ? ` · ${item.command}` : ""}
                </span>
              </div>
            ))}
          </DataState>
        </div>
        <LoadMore hasNextPage={events.hasNextPage} isFetchingNextPage={events.isFetchingNextPage} onLoadMore={() => events.fetchNextPage()} />
      </Card>
    </PageFrame>
  );
}
