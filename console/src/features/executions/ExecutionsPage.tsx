import { useState } from "react";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { PageFrame } from "@/components/layout/PageFrame";
import { useExecutions, flatten } from "@/hooks/queries";
import { ExecutionTable } from "./ExecutionTable";

export function ExecutionsPage() {
  const [status, setStatus] = useState("all");
  const [commandName, setCommandName] = useState("");
  const result = useExecutions({ status: status === "all" ? undefined : status, command: commandName || undefined });
  const items = flatten(result.data?.pages);
  return (
    <PageFrame
      title="执行记录"
      description="所有命令的不可变审计轨迹"
      action={
        <div className="flex gap-2">
          <Input className="w-40" value={commandName} onChange={(event) => setCommandName(event.target.value)} placeholder="Command 名称" />
          <Select value={status} onValueChange={setStatus}>
            <SelectTrigger className="w-32">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部状态</SelectItem>
              <SelectItem value="success">成功</SelectItem>
              <SelectItem value="error">失败</SelectItem>
            </SelectContent>
          </Select>
        </div>
      }
    >
      <ExecutionTable
        items={items}
        isLoading={result.isPending}
        isError={result.isError}
        hasNextPage={result.hasNextPage}
        isFetchingNextPage={result.isFetchingNextPage}
        onLoadMore={() => result.fetchNextPage()}
      />
    </PageFrame>
  );
}
