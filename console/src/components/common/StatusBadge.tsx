import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

const STATUS: Record<string, { label: string; className: string }> = {
  success: { label: "成功", className: "border-emerald-200 bg-emerald-50 text-emerald-700" },
  error: { label: "失败", className: "border-red-200 bg-red-50 text-red-700" },
  pending: { label: "待处理", className: "border-amber-200 bg-amber-50 text-amber-700" },
  running: { label: "运行中", className: "border-sky-200 bg-sky-50 text-sky-700" },
  dead: { label: "死信", className: "border-red-200 bg-red-50 text-red-700" },
  done: { label: "已完成", className: "border-emerald-200 bg-emerald-50 text-emerald-700" },
};

export function StatusBadge({ status }: { status: string }) {
  const entry = STATUS[status] ?? { label: status, className: "" };
  return (
    <Badge variant="outline" className={cn(entry.className)}>
      {entry.label}
    </Badge>
  );
}

export function MaturityBadge({ maturity }: { maturity: string }) {
  const stable = maturity === "Stable";
  return (
    <Badge
      variant="outline"
      className={stable ? "border-emerald-200 bg-emerald-50 text-emerald-700" : "border-amber-200 bg-amber-50 text-amber-700"}
    >
      {maturity}
    </Badge>
  );
}
