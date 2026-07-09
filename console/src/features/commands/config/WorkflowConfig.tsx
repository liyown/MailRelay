import { Plus, Trash } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { kvToRecord, recordToKv, type KVRow } from "../KeyValueRows";
import type { CommandDetail, Parameter } from "@/lib/api";

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
  paramNames,
}: {
  config: Record<string, unknown>;
  setConfig: (c: Record<string, unknown>) => void;
  existingCommands: CommandDetail[];
  paramNames: string[];
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
  const commandByName = new Map(existingCommands.map((command) => [command.name, command]));
  const updateStepParam = (stepIndex: number, key: string, value: string) => {
    const step = steps[stepIndex];
    if (!step) return;
    const existing = new Map(step.params.map((row) => [row.key, row.value]));
    existing.set(key, value);
    const target = commandByName.get(step.command);
    const targetParamNames = Object.keys(target?.parameters ?? {});
    const params = targetParamNames
      .map((name) => ({ key: name, value: existing.get(name) ?? "" }))
      .filter((row) => row.value.trim());
    updateStep(stepIndex, { params });
  };

  return (
    <div className="grid gap-4">
      <p className="text-xs text-muted-foreground">
        Workflow 按顺序调用已声明的其他命令。任意步骤失败则整个 Workflow 中止并回复失败。
        选择目标命令后，把 Workflow 收到的邮件参数映射到目标命令已声明的参数。
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

          <StepParamMapping
            step={step}
            target={commandByName.get(step.command)}
            workflowParamNames={paramNames}
            onChange={(key, value) => updateStepParam(i, key, value)}
          />
        </div>
      ))}

      <Button variant="outline" size="sm" className="w-fit" onClick={addStep}>
        <Plus className="size-3.5" />
        添加步骤
      </Button>
    </div>
  );
}

function StepParamMapping({
  step,
  target,
  workflowParamNames,
  onChange,
}: {
  step: Step;
  target?: CommandDetail;
  workflowParamNames: string[];
  onChange: (key: string, value: string) => void;
}) {
  if (!step.command) {
    return <p className="text-xs text-muted-foreground">先选择目标命令，再配置参数映射。</p>;
  }
  const targetParams = Object.entries(target?.parameters ?? {}) as Array<[string, Parameter]>;
  if (targetParams.length === 0) {
    return <p className="text-xs text-muted-foreground">目标命令没有声明参数，此步骤无需映射。</p>;
  }
  const values = new Map(step.params.map((row) => [row.key, row.value]));
  return (
    <div className="grid gap-2">
      <div className="grid grid-cols-[minmax(140px,0.8fr)_1fr] gap-2">
        <Label className="text-xs text-muted-foreground">目标命令参数</Label>
        <Label className="text-xs text-muted-foreground">映射来源或固定值</Label>
      </div>
      {targetParams.map(([name, param]) => {
        const value = values.get(name) ?? "";
        return (
          <div key={name} className="grid grid-cols-[minmax(140px,0.8fr)_1fr] items-center gap-2">
            <div className="min-w-0 rounded-md border border-border bg-muted/40 px-3 py-2">
              <div className="flex min-w-0 items-center gap-2">
                <span className="truncate font-mono text-xs">{name}</span>
                {param.required && <Badge variant="outline" className="h-5 px-1.5 text-[10px]">必填</Badge>}
              </div>
              <div className="mt-0.5 truncate text-[11px] text-muted-foreground">
                {param.type || "string"}{param.description ? ` · ${param.description}` : ""}
              </div>
            </div>
            <div className="flex min-w-0 gap-1.5">
              <Select value="" onValueChange={(source) => onChange(name, `{{${source}}}`)}>
                <SelectTrigger className="w-[150px] shrink-0 text-xs">
                  <SelectValue placeholder="选择入参" />
                </SelectTrigger>
                <SelectContent>
                  {workflowParamNames.map((source) => (
                    <SelectItem key={source} value={source} className="font-mono text-xs">
                      {`{{${source}}}`}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Input
                className="min-w-0 font-mono text-xs"
                value={value}
                onChange={(event) => onChange(name, event.target.value)}
                placeholder={workflowParamNames.length > 0 ? "{{参数名}} 或固定值" : "固定值"}
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}
