import { CheckCircle, Warning } from "@phosphor-icons/react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { formatDateTime } from "@/lib/format";
import type { EventItem } from "@/lib/api";

export function EventCallout({ event }: { event?: EventItem }) {
  if (!event) {
    return (
      <Card className="shadow-none">
        <CardContent className="flex items-center gap-3 p-4 text-sm text-muted-foreground">
          <CheckCircle weight="fill" className="size-6 text-emerald-600" />
          当前没有需要关注的运行事件
        </CardContent>
      </Card>
    );
  }
  return (
    <Card className="border-primary/40 bg-[#fffaf7] shadow-none">
      <CardContent className="flex gap-3 p-4">
        <div className="grid size-8 shrink-0 place-items-center rounded-full bg-primary text-white">
          <Warning weight="fill" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2 text-sm font-medium">
            运行事件需要关注
            <Badge variant="outline" className="border-primary/20 bg-primary/5 text-primary">
              {event.severity}
            </Badge>
          </div>
          <p className="mt-2 text-sm text-muted-foreground">{event.summary}</p>
          <div className="mt-3 text-xs text-muted-foreground">
            {formatDateTime(event.at)} · {event.phase}
            {event.command ? ` · ${event.command}` : ""}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
