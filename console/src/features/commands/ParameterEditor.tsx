import { Plus, Trash } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import type { Parameter } from "@/lib/api";

export type ParameterRow = {
  name: string;
  description: string;
  type: string;
  required: boolean;
  sensitive: boolean;
  example: string;
};

const TYPES = [
  { value: "string", label: "string（文本）" },
  { value: "integer", label: "integer（整数）" },
  { value: "number", label: "number（小数）" },
  { value: "boolean", label: "boolean（布尔）" },
];

export const emptyRow = (): ParameterRow => ({
  name: "",
  description: "",
  type: "string",
  required: false,
  sensitive: false,
  example: "",
});

export function rowsToParams(rows: ParameterRow[]): Record<string, Parameter> | undefined {
  const valid = rows.filter((r) => r.name.trim());
  if (valid.length === 0) return undefined;
  return Object.fromEntries(
    valid.map((r) => [
      r.name.trim(),
      {
        ...(r.description ? { description: r.description } : {}),
        ...(r.type && r.type !== "string" ? { type: r.type } : {}),
        ...(r.required ? { required: true } : {}),
        ...(r.sensitive ? { sensitive: true } : {}),
        ...(r.example ? { example: r.example } : {}),
      } satisfies Parameter,
    ]),
  );
}

export function paramsToRows(params: Record<string, Parameter> | undefined): ParameterRow[] {
  if (!params) return [];
  return Object.entries(params).map(([name, p]) => ({
    name,
    description: p.description ?? "",
    type: p.type ?? "string",
    required: p.required ?? false,
    sensitive: p.sensitive ?? false,
    example: p.example != null ? String(p.example) : "",
  }));
}

// ParameterEditor replaces the raw-JSON parameter textarea with structured rows.
// Each row maps to one entry in command.Parameters.
export function ParameterEditor({
  rows,
  onChange,
}: {
  rows: ParameterRow[];
  onChange: (rows: ParameterRow[]) => void;
}) {
  const update = <K extends keyof ParameterRow>(index: number, field: K, value: ParameterRow[K]) => {
    onChange(rows.map((r, i) => (i === index ? { ...r, [field]: value } : r)));
  };
  const remove = (index: number) => onChange(rows.filter((_, i) => i !== index));
  const add = () => onChange([...rows, emptyRow()]);

  return (
    <div className="grid gap-3">
      {rows.length > 0 && (
        <div className="grid grid-cols-[1.4fr_1fr_1.5fr_auto_auto_auto] items-center gap-1.5 px-1">
          {["名称", "类型", "说明"].map((h) => (
            <Label key={h} className="text-xs text-muted-foreground">{h}</Label>
          ))}
          <Label className="text-xs text-muted-foreground text-center">必填</Label>
          <Label className="text-xs text-muted-foreground text-center">脱敏</Label>
          <span />
        </div>
      )}
      {rows.map((row, i) => (
        <div key={i} className="grid grid-cols-[1.4fr_1fr_1.5fr_auto_auto_auto] items-center gap-1.5">
          <Input
            value={row.name}
            placeholder="参数名"
            onChange={(e) => update(i, "name", e.target.value)}
            className="font-mono text-xs"
          />
          <Select value={row.type} onValueChange={(v) => update(i, "type", v)}>
            <SelectTrigger className="text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {TYPES.map((t) => (
                <SelectItem key={t.value} value={t.value} className="text-xs">
                  {t.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Input
            value={row.description}
            placeholder="用途说明（给发件人看）"
            onChange={(e) => update(i, "description", e.target.value)}
            className="text-xs"
          />
          <Checkbox
            checked={row.required}
            onCheckedChange={(v) => update(i, "required", v === true)}
          />
          <Checkbox
            checked={row.sensitive}
            onCheckedChange={(v) => update(i, "sensitive", v === true)}
            title="脱敏：该参数值在审计日志和队列任务中会被隐去"
          />
          <Button variant="ghost" size="icon-sm" onClick={() => remove(i)} aria-label="删除参数">
            <Trash className="size-3.5" />
          </Button>
        </div>
      ))}
      <Button variant="outline" size="sm" className="w-fit" onClick={add}>
        <Plus className="size-3.5" />
        添加参数
      </Button>
      {rows.length > 0 && (
        <p className="text-xs text-muted-foreground">
          脱敏：该参数值会从审计日志和队列任务里隐去；脱敏参数不能用于 queue 命令。
        </p>
      )}
    </div>
  );
}
