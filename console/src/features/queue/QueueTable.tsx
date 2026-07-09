import { ArrowClockwise } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Panel, PanelTitle } from "@/components/common/Panel";
import { DataState } from "@/components/common/DataState";
import { LoadMore } from "@/components/common/LoadMore";
import { StatusBadge } from "@/components/common/StatusBadge";
import { ConfirmButton } from "@/components/common/ConfirmButton";
import { APIError } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { useReplay } from "@/hooks/useReplay";

export type QueueRow = { id: number; target: string; status: string; attempts: string; at: string };

const HEADERS = ["ID", "目标", "状态", "尝试", "可执行时间", ""];

export function QueueTable({
  title,
  kind,
  rows,
  csrf,
  isLoading,
  isError,
  hasNextPage,
  isFetchingNextPage,
  onLoadMore,
}: {
  title: string;
  kind: "job" | "reply";
  rows: QueueRow[];
  csrf: string;
  isLoading: boolean;
  isError: boolean;
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  onLoadMore: () => void;
}) {
  const replay = useReplay(kind, csrf);
  const label = kind === "job" ? "任务" : "回复";
  const errorText = replay.error instanceof APIError ? replay.error.message : replay.error ? "重放失败，请稍后重试" : undefined;
  return (
    <Panel className="overflow-hidden">
      <PanelTitle>{title}</PanelTitle>
      <div className="overflow-x-auto">
        <DataState isLoading={isLoading} isError={isError} isEmpty={rows.length === 0} emptyText={`当前没有${label}`}>
          <Table>
            <TableHeader>
              <TableRow>
                {HEADERS.map((header, index) => (
                  <TableHead key={index}>{header}</TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => (
                <TableRow key={row.id}>
                  <TableCell className="font-mono text-xs">{row.id}</TableCell>
                  <TableCell>{row.target}</TableCell>
                  <TableCell>
                    <StatusBadge status={row.status} />
                  </TableCell>
                  <TableCell>{row.attempts}</TableCell>
                  <TableCell>{formatDateTime(row.at)}</TableCell>
                  <TableCell className="text-right">
                    {row.status === "dead" && (
                      <ConfirmButton
                        trigger={
                          <Button variant="outline" size="sm">
                            <ArrowClockwise />
                            重放
                          </Button>
                        }
                        title={`重放死信${label}`}
                        description={`确认将 ${label} #${row.id} 重新排入队列？系统会重置尝试次数并由后台工作器再次处理。`}
                        confirmText="重放"
                        pending={replay.isPending && replay.variables === row.id}
                        error={errorText}
                        onConfirm={() => replay.mutate(row.id)}
                      />
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </DataState>
      </div>
      <LoadMore hasNextPage={hasNextPage} isFetchingNextPage={isFetchingNextPage} onLoadMore={onLoadMore} />
    </Panel>
  );
}
