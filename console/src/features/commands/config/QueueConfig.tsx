import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { CommandDetail } from "@/lib/api";

export function QueueConfig({
  config,
  setConfig,
  existingCommands,
}: {
  config: Record<string, unknown>;
  setConfig: (c: Record<string, unknown>) => void;
  existingCommands: CommandDetail[];
}) {
  const set = (key: string, value: unknown) => setConfig({ ...config, [key]: value });
  const target = typeof config.command === "string" ? config.command : "";
  const maxAttempts = config.max_attempts != null ? String(config.max_attempts) : "";

  return (
    <div className="grid gap-4">
      <p className="text-xs text-muted-foreground">
        Queue 把目标命令排入持久队列异步执行，失败后按退避策略自动重试。
        <strong>注意：sensitive（脱敏）参数不能用于 Queue 命令</strong>，因为参数需要落盘。
      </p>

      <div className="grid gap-1.5">
        <Label>目标命令</Label>
        <Select value={target} onValueChange={(v) => set("command", v)}>
          <SelectTrigger>
            <SelectValue placeholder="选择要异步执行的命令..." />
          </SelectTrigger>
          <SelectContent>
            {existingCommands.map((cmd) => (
              <SelectItem key={cmd.name} value={cmd.name}>
                {cmd.name}
                {cmd.description ? ` — ${cmd.description}` : ""}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <p className="text-xs text-muted-foreground">
          目标命令的参数声明决定了 Queue 命令能传哪些参数，类型必须一致。
        </p>
      </div>

      <div className="grid gap-1.5">
        <Label htmlFor="q-max">最大重试次数</Label>
        <Input
          id="q-max"
          type="number"
          min={1}
          max={100}
          value={maxAttempts}
          onChange={(e) => set("max_attempts", e.target.value ? parseInt(e.target.value, 10) : undefined)}
          placeholder="3（默认）"
          className="w-32"
        />
      </div>
    </div>
  );
}
