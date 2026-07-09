import { Cell, Pie, PieChart, ResponsiveContainer } from "recharts";
import { CardContent } from "@/components/ui/card";
import { Panel, PanelTitle } from "@/components/common/Panel";
import type { Dashboard } from "@/lib/api";

const COLORS = ["#d84a1b", "#c69a56", "#d9c9ad", "#9f9a92", "#e6e1d8"];
const LABELS = ["队列待处理", "队列运行中", "队列死信", "回复待处理", "回复死信"];

export function DistributionPanel({ data }: { data?: Dashboard }) {
  const values = [
    data?.queue.pending ?? 0,
    data?.queue.running ?? 0,
    data?.queue.dead ?? 0,
    data?.replies.pending ?? 0,
    data?.replies.dead ?? 0,
  ];
  const total = values.reduce((sum, value) => sum + value, 0);
  const percentages = values.map((value) => (total ? Math.round((value / total) * 100) : 0));
  const chart = values.map((value, index) => ({ value: value || (total ? 0 : index === 0 ? 1 : 0) }));
  return (
    <Panel>
      <PanelTitle>工作负载分布</PanelTitle>
      <CardContent className="flex items-center gap-6 p-5">
        <div className="relative size-28 shrink-0">
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie data={chart} dataKey="value" innerRadius={36} outerRadius={54} stroke="none">
                {chart.map((_, index) => (
                  <Cell key={COLORS[index]} fill={COLORS[index]} />
                ))}
              </Pie>
            </PieChart>
          </ResponsiveContainer>
          <div className="pointer-events-none absolute inset-0 grid place-items-center text-center">
            <span className="text-xl font-semibold leading-none">
              {total}
              <small className="mt-1 block text-[10px] font-normal text-muted-foreground">总计</small>
            </span>
          </div>
        </div>
        <div className="grid flex-1 gap-2 text-xs">
          {LABELS.map((label, index) => (
            <div key={label} className="flex justify-between">
              <span>
                <i className="mr-2 inline-block size-1.5 rounded-full" style={{ background: COLORS[index] }} />
                {label}
              </span>
              <span>
                {values[index]}　{percentages[index]}%
              </span>
            </div>
          ))}
        </div>
      </CardContent>
    </Panel>
  );
}
