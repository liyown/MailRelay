import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export function HttpRequestConfig({
  config,
  setConfig,
}: {
  config: Record<string, unknown>;
  setConfig: (c: Record<string, unknown>) => void;
}) {
  const update = (patch: Record<string, unknown>) => setConfig({ ...config, ...patch });
  return (
    <div className="grid gap-4">
      <p className="text-xs text-muted-foreground">
        邮件正文写完整 HTTP 请求报文；base_url 用于解析 <code>GET /path HTTP/1.1</code> 这种相对请求。
        最终目标仍受 security.http_hosts 限制。
      </p>
      <div className="grid gap-1.5">
        <Label htmlFor="http-request-base-url">Base URL</Label>
        <Input
          id="http-request-base-url"
          value={String(config.base_url ?? "")}
          onChange={(event) => update({ base_url: event.target.value })}
          placeholder="https://api.example.com"
          className="font-mono"
        />
      </div>
    </div>
  );
}
