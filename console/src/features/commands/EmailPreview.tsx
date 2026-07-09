import { useEffect, useState } from "react";
import { Check, Copy, Envelope } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useSystem } from "@/hooks/queries";
import type { ParameterRow } from "./ParameterEditor";

function exampleValue(row: ParameterRow): string {
  if (row.example) return row.example;
  switch (row.type) {
    case "integer": return "42";
    case "number":  return "3.14";
    case "boolean": return "true";
    default:        return row.name ? `your-${row.name}` : "value";
  }
}

function buildPlainBody(rows: ParameterRow[], token: string): string {
  const lines = [`_token=${token}`];
  for (const row of rows.filter((r) => r.name.trim())) {
    lines.push(`${row.name}=${exampleValue(row)}`);
  }
  return lines.join("\n");
}

function buildJsonBody(rows: ParameterRow[], token: string): string {
  const params = rows.filter((r) => r.name.trim());
  const body: Record<string, unknown> = { _token: token };
  for (const row of params) {
    const v = row.example || exampleValue(row);
    body[row.name] =
      row.type === "integer" ? Number(v) || 0
      : row.type === "number"  ? parseFloat(v) || 0
      : row.type === "boolean" ? v === "true"
      : v;
  }
  return JSON.stringify(body, null, 2);
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  const copy = () => {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  };
  return (
    <Button variant="ghost" size="sm" className="h-7 gap-1 text-xs" onClick={copy}>
      {copied ? <Check className="size-3 text-emerald-600" /> : <Copy className="size-3" />}
      {copied ? "已复制" : "复制"}
    </Button>
  );
}

function Block({ label, content }: { label: string; content: string }) {
  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-muted-foreground">{label}</span>
        <CopyButton text={content} />
      </div>
      <pre className="overflow-x-auto rounded-lg border border-border bg-muted/50 p-4 font-mono text-xs leading-6 whitespace-pre">
        {content}
      </pre>
    </div>
  );
}

export function EmailPreview({
  name,
  paramRows,
  token: configToken,
}: {
  name: string;
  paramRows: ParameterRow[];
  token: string;
}) {
  const [token, setToken] = useState(configToken);
  const system = useSystem();
  const inboxAddress = system.data?.inbox_address ?? "";

  useEffect(() => {
    setToken(configToken);
  }, [configToken]);

  const cmdName = name || "command-name";
  const plainBody = buildPlainBody(paramRows, token);
  const jsonBody = buildJsonBody(paramRows, token);

  const mailtoPlain =
    `mailto:${inboxAddress}` +
    `?subject=${encodeURIComponent(cmdName)}` +
    `&body=${encodeURIComponent(plainBody)}`;

  const required = paramRows.filter((r) => r.name.trim() && r.required);
  const optional = paramRows.filter((r) => r.name.trim() && !r.required);

  return (
    <div className="space-y-5 py-2">
      {/* Explanation */}
      <div className="rounded-lg border border-border bg-muted/30 px-4 py-3 text-xs text-muted-foreground leading-relaxed space-y-1">
        <p>
          发送邮件到 MailRelay 收件邮箱，<strong className="text-foreground">主题首词</strong>即为命令名。
        </p>
        <p>
          Token 在正文里以 <code className="rounded bg-muted px-1 text-foreground">_token=…</code> 传入（解析后自动移除，不会传给 handler）。
        </p>
      </div>

      {/* Inbox + Token inputs */}
      <div className="grid gap-2">
        <div className="flex items-center gap-3">
          <span className="w-16 shrink-0 text-xs text-muted-foreground">收件地址</span>
          <span className="flex-1 rounded-md border border-border bg-muted/50 px-3 py-1.5 font-mono text-xs text-foreground">
            {inboxAddress || <span className="text-muted-foreground/60">（从系统配置加载中）</span>}
          </span>
        </div>
        <div className="flex items-center gap-3">
          <span className="w-16 shrink-0 text-xs text-muted-foreground">Token</span>
          <input
            value={token}
            onChange={(e) => setToken(e.target.value)}
            className="h-8 flex-1 rounded-md border border-border bg-background px-3 font-mono text-xs outline-none focus:ring-1 focus:ring-ring"
            placeholder="从 security.token 读取"
          />
        </div>
      </div>

      {/* Parameter legend */}
      {paramRows.some((r) => r.name.trim()) && (
        <div className="flex flex-wrap gap-2">
          {required.map((r) => (
            <span key={r.name} className="inline-flex items-center gap-1 rounded-full border border-primary/30 bg-primary/5 px-2 py-0.5 font-mono text-[11px] text-primary">
              {r.name} <span className="text-primary/60">必填</span>
            </span>
          ))}
          {optional.map((r) => (
            <span key={r.name} className="inline-flex items-center gap-1 rounded-full border border-border bg-muted px-2 py-0.5 font-mono text-[11px] text-muted-foreground">
              {r.name} <span className="opacity-60">可选</span>
            </span>
          ))}
        </div>
      )}

      {/* Plain text format (primary) */}
      <div className="space-y-1.5">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium text-muted-foreground">纯文本格式（推荐）</span>
          <div className="flex gap-1">
            <CopyButton text={plainBody} />
            {inboxAddress && (
              <Button variant="outline" size="sm" className="h-7 gap-1 text-xs" asChild>
                <a href={mailtoPlain} target="_blank" rel="noreferrer">
                  <Envelope className="size-3" />
                  一键发送
                </a>
              </Button>
            )}
          </div>
        </div>
        <pre className="overflow-x-auto rounded-lg border border-border bg-muted/50 p-4 font-mono text-xs leading-6 whitespace-pre">
          <span className="text-muted-foreground">To:      </span>{inboxAddress || "relay@your-domain.com"}{"\n"}
          <span className="text-muted-foreground">Subject: </span>{cmdName}{"\n"}
          {"\n"}
          {plainBody}
        </pre>
      </div>

      {/* JSON format (secondary) */}
      <Block
        label="JSON 格式（Content-Type: application/json）"
        content={
          `To:      ${inboxAddress || "relay@your-domain.com"}\n` +
          `Subject: ${cmdName}\n\n` +
          jsonBody
        }
      />
    </div>
  );
}
