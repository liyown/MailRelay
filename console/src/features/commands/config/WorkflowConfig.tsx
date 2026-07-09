import { Plus, Trash } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { KeyValueRows, kvToRecord, recordToKv, type KVRow } from "../KeyValueRows";
import type { CommandDetail } from "@/lib/api";

type Step = { command: string; params: KVRow[] };

function stepsToConfig(steps: Step[]): unknown[] {
  return steps.map((s) => ({
    command: s.command,
    ...(s.params.some((r) => r.key.trim())
      ? { params: kvToRecord(s.params) }
      : {}),
  }));
}

function configToSteps(config: Record<string, unknown>): Step[] {
  const raw = config.steps;
  if (!Array.isArray(raw)) return [];
  return raw.map((item) => {
    const s = item as Record<string, unknown>;
    const params = s.params && typeof s.params === "object"
      ? recordToKv(s.params as Record<string, unknown>)
      : [];
    return { command: String(s.command ?? ""), params };
  });
}

export function WorkflowConfig({
  config,
  setConfig,
  existingCommands,
}: {
  config: Record<string, unknown>;
  setConfig: (c: Record<string, unknown>) => void;
  existingCommands: CommandDetail[];
}) {
  const steps = configToSteps(config);

  const updateStep = (index: number, update: Partial<Step>) => {
    const next = steps.map((s, i) => (i === index ? { ...s, ...update } : s));
    setConfig({ ...config, steps: stepsToConfig(next) });
  };
  const removeStep = (index: number) =>
    setConfig({ ...config, steps: stepsToConfig(steps.filter((_, i) => i !== index)) });
  const addStep = () =>
    setConfig({ ...config, steps: stepsToConfig([...steps, { command: "", params: [] }]) });

  return (
    <div className="grid gap-4">
      <p className="text-xs text-muted-foreground">
        Workflow 按顺序调用已声明的其他命令。任意步骤失败则整个 Workflow 中止并回复失败。
        步骤参数值支持 <code>{"{{参数名}}"}</code>，会用 Workflow 收到的邮件参数展开。
      </p>

      {steps.map((step, i) => (
        <div key={i} className="rounded-lg border p-3 grid gap-3">
          <div className="flex items-center justify-between">
            <Label className="text-sm font-medium">步骤 {i + 1}</Label>
            <Button variant="ghost" size="icon-sm" onClick={() => removeStep(i)} aria-label="删除步骤">
              <Trash className="size-3.5" />
            </Button>
          </div>

          <div className="grid gap-1.5">
            <Label className="text-xs">目标命令</Label>
            <Select value={step.command} onValueChange={(v) => updateStep(i, { command: v })}>
              <SelectTrigger className="text-xs">
                <SelectValue placeholder="选择命令..." />
              </SelectTrigger>
              <SelectContent>
                {existingCommands.map((cmd) => (
                  <SelectItem key={cmd.name} value={cmd.name} className="text-xs">
                    {cmd.name}
                    {cmd.description ? ` — ${cmd.description}` : ""}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="grid gap-1.5">
            <Label className="text-xs">传入参数</Label>
            <KeyValueRows
              rows={step.params}
              onChange={(rows) => updateStep(i, { params: rows })}
              keyLabel="参数名"
              valueLabel={`值（可用 {{参数名}}）`}
              keyPlaceholder="参数名"
              valuePlaceholder="{{message}} 或固定值"
              addLabel="添加参数"
            />
            <p className="text-xs text-muted-foreground">
              字符串值里可写 <code>{"{{参数名}}"}</code>，会用 Workflow 收到的邮件参数展开后再传给目标命令。
            </p>
          </div>
        </div>
      ))}

      <Button variant="outline" size="sm" className="w-fit" onClick={addStep}>
        <Plus className="size-3.5" />
        添加步骤
      </Button>
    </div>
  );
}
