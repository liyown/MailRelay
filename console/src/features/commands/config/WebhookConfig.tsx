import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

function getStr(config: Record<string, unknown>, key: string): string {
  const v = config[key];
  return typeof v === "string" ? v : "";
}

export function WebhookConfig({
  config,
  setConfig,
  httpHosts,
}: {
  config: Record<string, unknown>;
  setConfig: (c: Record<string, unknown>) => void;
  httpHosts: string[];
}) {
  const set = (key: string, value: unknown) => setConfig({ ...config, [key]: value });

  const url = getStr(config, "url");
  const urlHost = (() => {
    try { return new URL(url).hostname; } catch { return ""; }
  })();
  const hostWarning = url && urlHost && !httpHosts.some((h) => h.toLowerCase() === urlHost.toLowerCase());

  return (
    <div className="grid gap-4">
      <div className="grid gap-1.5">
        <Label htmlFor="wh-url">URL</Label>
        <Input
          id="wh-url"
          value={url}
          onChange={(e) => set("url", e.target.value)}
          placeholder="https://hooks.example.com/webhook"
        />
        {hostWarning && (
          <p className="text-xs text-amber-600 dark:text-amber-400">
            主机 <code>{urlHost}</code> 不在白名单（http_hosts）里，保存时会被校验拒绝。
          </p>
        )}
      </div>

      <div className="grid gap-1.5">
        <Label htmlFor="wh-secret">签名密钥 (Secret)</Label>
        <Input
          id="wh-secret"
          value={getStr(config, "secret")}
          onChange={(e) => set("secret", e.target.value)}
          placeholder="${WEBHOOK_SECRET}"
        />
        <p className="text-xs text-muted-foreground">
          设置后会在请求头 <code>X-MailRelay-Signature: sha256=…</code> 里加 HMAC 签名，供接收方验证。
          建议用 <code>{"${ENV_VAR}"}</code> 引用环境变量，不要明文写入。
        </p>
      </div>

      <div className="rounded-lg border bg-muted/40 px-3 py-2.5">
        <p className="mb-2 text-xs font-medium">发送的 JSON 信封（自动）</p>
        <pre className="text-xs text-muted-foreground">{`{
  "version": "1",
  "command": "<命令名>",
  "request_id": "<邮件 Message-ID>",
  "timestamp": "<ISO 8601>",
  "params": { /* 邮件传入的参数 */ }
}`}</pre>
        <p className="mt-2 text-xs text-muted-foreground">
          所有参数通过 <code>params</code> 对象结构化传递，不支持 <code>{"{{参数}}"}</code> 模板语法。
        </p>
      </div>
    </div>
  );
}
