import { Envelope, Queue } from "@phosphor-icons/react";
import { CardContent } from "@/components/ui/card";
import { Panel, PanelTitle } from "@/components/common/Panel";
import type { Dashboard, WorkCounts } from "@/lib/api";

function Column({ icon, title, counts }: { icon: React.ReactNode; title: string; counts?: WorkCounts }) {
  return (
    <div className="p-4">
      <div className="mb-3 flex items-center gap-2 text-sm font-medium">
        {icon}
        {title}
      </div>
      <div className="grid grid-cols-3 gap-2 text-center text-xs">
        <div>
          <div className="text-xl font-semibold">{counts?.pending ?? 0}</div>
          <div className="mt-1 text-muted-foreground">待处理</div>
        </div>
        <div>
          <div className="text-xl font-semibold">{counts?.running ?? 0}</div>
          <div className="mt-1 text-muted-foreground">运行中</div>
        </div>
        <div>
          <div className="text-xl font-semibold text-primary">{counts?.dead ?? 0}</div>
          <div className="mt-1 text-muted-foreground">死信</div>
        </div>
      </div>
    </div>
  );
}

export function QueuePanel({ data }: { data?: Dashboard }) {
  return (
    <Panel>
      <PanelTitle>队列与回复状态</PanelTitle>
      <CardContent className="grid grid-cols-2 divide-x divide-border p-0">
        <Column icon={<Queue />} title="命令队列" counts={data?.queue} />
        <Column icon={<Envelope />} title="回复处理" counts={data?.replies} />
      </CardContent>
    </Panel>
  );
}
