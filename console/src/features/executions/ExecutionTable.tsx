import { useState } from "react";
import { Export } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Panel, PanelTitle } from "@/components/common/Panel";
import { StatusBadge } from "@/components/common/StatusBadge";
import { DataState } from "@/components/common/DataState";
import { LoadMore } from "@/components/common/LoadMore";
import { formatDateTime, formatSeconds } from "@/lib/format";
import type { Execution } from "@/lib/api";
import { ExecutionDetail } from "./ExecutionDetail";

function downloadExecutions(items: Execution[]) {
  const rows = [
    ["id", "command", "sender", "status", "handler", "duration_ms", "started_at"],
    ...items.map((item) => [item.id, item.command, item.sender ?? "", item.status, item.handler, item.duration_ms, item.started_at]),
  ];
  const csv = rows.map((row) => row.map((value) => `"${String(value).replaceAll('"', '""')}"`).join(",")).join("\n");
  const url = URL.createObjectURL(new Blob([csv], { type: "text/csv;charset=utf-8" }));
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = "mailrelay-executions.csv";
  anchor.click();
  URL.revokeObjectURL(url);
}

const HEADERS = ["执行 ID", "命令", "发送者", "状态", "处理器", "耗时", "执行时间"];

export function ExecutionTable({
  items = [],
  compact = false,
  onViewAll,
  isLoading = false,
  isError = false,
  hasNextPage = false,
  isFetchingNextPage = false,
  onLoadMore,
}: {
  items?: Execution[];
  compact?: boolean;
  onViewAll?: () => void;
  isLoading?: boolean;
  isError?: boolean;
  hasNextPage?: boolean;
  isFetchingNextPage?: boolean;
  onLoadMore?: () => void;
}) {
  const [selected, setSelected] = useState<Execution | null>(null);
  return (
    <Panel className="mt-4 overflow-hidden">
      <PanelTitle
        action={
          <Button variant="outline" onClick={() => downloadExecutions(items)} disabled={items.length === 0}>
            <Export />
            导出
          </Button>
        }
      >
        {compact ? "最近执行记录" : "执行记录"}
      </PanelTitle>
      <div className="overflow-x-auto">
        <DataState isLoading={isLoading} isError={isError} isEmpty={items.length === 0} emptyText="暂无执行记录">
          <Table>
            <TableHeader>
              <TableRow>
                {HEADERS.map((header) => (
                  <TableHead key={header}>{header}</TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((item) => (
                <TableRow key={item.id} className="cursor-pointer" onClick={() => setSelected(item)}>
                  <TableCell className="font-mono text-xs">EXE-{item.id}</TableCell>
                  <TableCell>{item.command}</TableCell>
                  <TableCell>{item.sender || "—"}</TableCell>
                  <TableCell>
                    <StatusBadge status={item.status} />
                  </TableCell>
                  <TableCell>{item.handler}</TableCell>
                  <TableCell>{formatSeconds(item.duration_ms)}</TableCell>
                  <TableCell>{formatDateTime(item.started_at)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </DataState>
      </div>
      {onLoadMore && (
        <LoadMore hasNextPage={hasNextPage} isFetchingNextPage={isFetchingNextPage} onLoadMore={onLoadMore} />
      )}
      {compact && onViewAll && (
        <button onClick={onViewAll} className="w-full border-t border-border py-3 text-sm font-medium text-primary hover:bg-muted">
          查看全部执行记录　→
        </button>
      )}
      <ExecutionDetail execution={selected} onClose={() => setSelected(null)} />
    </Panel>
  );
}
