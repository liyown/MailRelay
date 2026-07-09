import { Badge } from "@/components/ui/badge";

// ParamPicker renders clickable chips for each declared parameter name.
// Clicking a chip calls onInsert with "{{name}}" so the parent can append it
// to whatever field is focused (body, args entry, env value, step param value).
export function ParamPicker({
  paramNames,
  onInsert,
}: {
  paramNames: string[];
  onInsert: (snippet: string) => void;
}) {
  if (paramNames.length === 0) return null;
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <span className="text-xs text-muted-foreground">插入参数：</span>
      {paramNames.map((name) => (
        <button
          key={name}
          type="button"
          onClick={() => onInsert(`{{${name}}}`)}
          className="cursor-pointer"
        >
          <Badge variant="outline" className="font-mono text-xs hover:bg-accent">
            {`{{${name}}}`}
          </Badge>
        </button>
      ))}
    </div>
  );
}
