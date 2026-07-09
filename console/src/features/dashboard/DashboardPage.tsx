import { useState } from "react";
import { Clock, Database, PaperPlaneTilt, Pulse, ShieldCheck } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { MetricCard } from "@/components/common/MetricCard";
import { useDashboard } from "@/hooks/queries";
import { formatCount, formatSeconds } from "@/lib/format";
import { ExecutionTable } from "@/features/executions/ExecutionTable";
import { TrendChart } from "./TrendChart";
import { EventCallout } from "./EventCallout";
import { HealthPanel } from "./HealthPanel";
import { QueuePanel } from "./QueuePanel";
import { DistributionPanel } from "./DistributionPanel";

export function DashboardPage({ onOpenExecutions }: { onOpenExecutions: () => void }) {
  const [range, setRange] = useState("24h");
  const dashboard = useDashboard(range);
  const data = dashboard.data;
  return (
    <>
      <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">仪表盘</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            系统概览与运行状态 · {dashboard.isFetching ? "正在更新" : "数据已同步"}
          </p>
        </div>
        <div className="flex gap-2">
          <Select value={range} onValueChange={setRange}>
            <SelectTrigger className="w-[142px]">
              <Clock />
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="24h">过去 24 小时</SelectItem>
              <SelectItem value="7d">过去 7 天</SelectItem>
              <SelectItem value="30d">过去 30 天</SelectItem>
            </SelectContent>
          </Select>
          <Button variant="outline" onClick={() => dashboard.refetch()} disabled={dashboard.isFetching}>
            <Pulse />
            刷新
          </Button>
        </div>
      </div>
      {dashboard.error && (
        <div role="alert" className="mb-4 rounded-lg border border-destructive/20 bg-red-50 p-3 text-sm text-destructive">
          无法读取运行数据，请稍后重试。
        </div>
      )}
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard icon={Database} label="执行命令数" value={formatCount(data?.execution_count)} delta="当前窗口" />
        <MetricCard
          icon={PaperPlaneTilt}
          label="执行成功率"
          value={data ? `${data.success_rate.toFixed(2)}%` : "—"}
          delta={data ? `${data.success_count} 次成功` : "等待数据"}
          good
        />
        <MetricCard icon={Clock} label="P95 执行耗时" value={data ? formatSeconds(data.p95_duration_ms) : "—"} delta="当前窗口" good />
        <MetricCard icon={ShieldCheck} label="活跃处理器" value={data ? String(data.active_handlers) : "—"} delta="当前窗口" />
      </div>
      <div className="mt-4 grid gap-4 xl:grid-cols-[1.15fr_1fr]">
        <div className="space-y-4">
          <TrendChart series={data?.series} />
          <EventCallout event={data?.recent_events[0]} />
        </div>
        <div className="space-y-4">
          <HealthPanel data={data} />
          <QueuePanel data={data} />
          <DistributionPanel data={data} />
        </div>
      </div>
      <ExecutionTable
        compact
        items={data?.recent_executions}
        isLoading={dashboard.isPending}
        isError={dashboard.isError}
        onViewAll={onOpenExecutions}
      />
    </>
  );
}
