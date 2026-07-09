import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { StatusBadge } from "@/components/common/StatusBadge";
import { formatDateTime, formatSeconds } from "@/lib/format";
import type { Execution } from "@/lib/api";

function Row({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="grid grid-cols-[110px_1fr] gap-3 border-b border-border py-3 text-sm last:border-0">
      <span className="text-muted-foreground">{label}</span>
      <span className={mono ? "break-all font-mono text-xs" : "break-words"}>{value || "—"}</span>
    </div>
  );
}

export function ExecutionDetail({ execution, onClose }: { execution: Execution | null; onClose: () => void }) {
  return (
    <Sheet open={Boolean(execution)} onOpenChange={(open) => !open && onClose()}>
      <SheetContent side="right" className="w-full gap-0 overflow-y-auto sm:max-w-md">
        <SheetHeader>
          <SheetTitle>执行详情</SheetTitle>
          <SheetDescription>单次命令执行的审计记录（已脱敏）。</SheetDescription>
        </SheetHeader>
        {execution && (
          <div className="px-4 pb-6">
            <div className="mb-3 flex items-center gap-3">
              <span className="font-mono text-sm">EXE-{execution.id}</span>
              <StatusBadge status={execution.status} />
            </div>
            <Row label="命令" value={execution.command} />
            <Row label="处理器" value={execution.handler} />
            <Row label="发送者" value={execution.sender ?? ""} />
            <Row label="Message-ID" value={execution.message_id} mono />
            <Row label="摘要" value={execution.summary} />
            <Row label="错误类型" value={execution.error_kind ?? ""} />
            <Row label="耗时" value={formatSeconds(execution.duration_ms)} />
            <Row label="执行时间" value={formatDateTime(execution.started_at)} />
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}
