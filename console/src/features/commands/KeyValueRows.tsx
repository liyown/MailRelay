import { Plus, Trash } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export type KVRow = { key: string; value: string };

// KeyValueRows renders an editable list of key→value pairs.
// valueLabel / keyLabel allow callers to customise the column headers.
// valueRenderer replaces the plain <Input> for the value column when provided
// (e.g. a Textarea for multi-line bodies).
export function KeyValueRows({
  rows,
  onChange,
  keyLabel = "键",
  valueLabel = "值",
  valuePlaceholder = "",
  keyPlaceholder = "",
  addLabel = "添加一行",
  valueRenderer,
}: {
  rows: KVRow[];
  onChange: (rows: KVRow[]) => void;
  keyLabel?: string;
  valueLabel?: string;
  keyPlaceholder?: string;
  valuePlaceholder?: string;
  addLabel?: string;
  valueRenderer?: (row: KVRow, index: number, onChange: (v: string) => void) => React.ReactNode;
}) {
  const update = (index: number, field: keyof KVRow, value: string) => {
    const next = rows.map((r, i) => (i === index ? { ...r, [field]: value } : r));
    onChange(next);
  };
  const remove = (index: number) => onChange(rows.filter((_, i) => i !== index));
  const add = () => onChange([...rows, { key: "", value: "" }]);

  return (
    <div className="grid gap-2">
      {rows.length > 0 && (
        <div className="grid grid-cols-[1fr_1fr_auto] gap-1.5">
          <Label className="text-xs text-muted-foreground">{keyLabel}</Label>
          <Label className="text-xs text-muted-foreground">{valueLabel}</Label>
          <span />
        </div>
      )}
      {rows.map((row, i) => (
        <div key={i} className="grid grid-cols-[1fr_1fr_auto] items-center gap-1.5">
          <Input
            value={row.key}
            placeholder={keyPlaceholder}
            onChange={(e) => update(i, "key", e.target.value)}
          />
          {valueRenderer ? (
            valueRenderer(row, i, (v) => update(i, "value", v))
          ) : (
            <Input
              value={row.value}
              placeholder={valuePlaceholder}
              onChange={(e) => update(i, "value", e.target.value)}
            />
          )}
          <Button variant="ghost" size="icon-sm" onClick={() => remove(i)} aria-label="删除">
            <Trash className="size-3.5" />
          </Button>
        </div>
      ))}
      <Button variant="outline" size="sm" className="w-fit" onClick={add}>
        <Plus className="size-3.5" />
        {addLabel}
      </Button>
    </div>
  );
}

/** Convert a KVRow array into a Record, skipping rows with empty keys. */
export function kvToRecord(rows: KVRow[]): Record<string, string> {
  return Object.fromEntries(rows.filter((r) => r.key.trim()).map((r) => [r.key, r.value]));
}

/** Convert a Record<string,unknown> into a KVRow array. */
export function recordToKv(obj: Record<string, unknown> | undefined): KVRow[] {
  if (!obj) return [];
  return Object.entries(obj).map(([key, value]) => ({ key, value: String(value ?? "") }));
}
