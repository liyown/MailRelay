import { Textarea } from "@/components/ui/textarea";

// GenericConfig is the JSON-textarea fallback used for agent, mcp, and any
// unknown handler types whose config schema is not fixed.
export function GenericConfig({
  config,
  setConfig,
  handler,
}: {
  config: Record<string, unknown>;
  setConfig: (c: Record<string, unknown>) => void;
  handler: string;
}) {
  const text = JSON.stringify(config, null, 2);
  const handleChange = (raw: string) => {
    try {
      const parsed = JSON.parse(raw);
      if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
        setConfig(parsed as Record<string, unknown>);
      }
    } catch {
      // keep the last valid config while user is typing
    }
  };

  return (
    <div className="grid gap-2">
      <p className="text-xs text-muted-foreground">
        <strong>{handler}</strong> 是实验性处理器，配置随实现而定。
        直接编辑下方 JSON，保存时经服务端完整校验。
      </p>
      <Textarea
        className="min-h-48 resize-y font-mono text-xs"
        defaultValue={text}
        onChange={(e) => handleChange(e.target.value)}
      />
    </div>
  );
}
