import type { ComponentType } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

type IconComponent = ComponentType<{ className?: string }>;

export function MetricCard({
  icon: Icon,
  label,
  value,
  delta,
  good = false,
}: {
  icon: IconComponent;
  label: string;
  value: string;
  delta: string;
  good?: boolean;
}) {
  return (
    <Card className="shadow-none">
      <CardContent className="flex items-center gap-4 p-5">
        <div className="grid size-12 shrink-0 place-items-center rounded-full border border-border">
          <Icon className="size-6" />
        </div>
        <div>
          <div className="text-sm text-muted-foreground">{label}</div>
          <div className="mt-0.5 text-2xl font-semibold tracking-tight">{value}</div>
          <div className={cn("mt-1 text-xs", good ? "text-emerald-700" : "text-primary")}>{delta}</div>
        </div>
      </CardContent>
    </Card>
  );
}
