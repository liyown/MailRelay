import { Area, AreaChart, CartesianGrid, Line, ResponsiveContainer, Tooltip as ChartTooltip, XAxis, YAxis } from "recharts";
import { Panel, PanelTitle } from "@/components/common/Panel";
import { CardContent } from "@/components/ui/card";
import { formatTime } from "@/lib/format";
import type { SeriesPoint } from "@/lib/api";

export function TrendChart({ series = [] }: { series?: SeriesPoint[] }) {
  const data = series.map((point) => ({
    time: formatTime(point.at),
    count: point.count,
    success: point.success,
  }));
  const hasData = series.some((point) => point.count > 0);
  return (
    <Panel>
      <PanelTitle action={<span className="text-xs text-muted-foreground">按时间分桶</span>}>命令活动趋势</PanelTitle>
      <CardContent className="relative h-[280px] p-4">
        {!hasData && (
          <div className="absolute inset-0 z-10 grid place-items-center text-sm text-muted-foreground">当前窗口暂无执行记录</div>
        )}
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={data}>
            <defs>
              <linearGradient id="warmArea" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="#d84a1b" stopOpacity={0.22} />
                <stop offset="100%" stopColor="#d84a1b" stopOpacity={0} />
              </linearGradient>
            </defs>
            <CartesianGrid vertical={false} stroke="#e8e1d9" />
            <XAxis dataKey="time" tick={{ fontSize: 11, fill: "#817870" }} axisLine={false} tickLine={false} minTickGap={24} />
            <YAxis tick={{ fontSize: 11, fill: "#817870" }} axisLine={false} tickLine={false} allowDecimals={false} />
            <ChartTooltip />
            <Area type="monotone" dataKey="count" name="执行" stroke="#d84a1b" strokeWidth={2} fill="url(#warmArea)" />
            <Line type="monotone" dataKey="success" name="成功" stroke="#c9ad7b" strokeWidth={2} dot={false} />
          </AreaChart>
        </ResponsiveContainer>
      </CardContent>
    </Panel>
  );
}
