import { useState } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { KeyValueRows, recordToKv, kvToRecord, type KVRow } from "../KeyValueRows";
import { ParamPicker } from "../ParamPicker";

const METHODS = ["POST", "GET", "PUT", "PATCH", "DELETE"];

function getStr(config: Record<string, unknown>, key: string): string {
  const v = config[key];
  return typeof v === "string" ? v : "";
}

export function HttpConfig({
  config,
  setConfig,
  paramNames,
  httpHosts,
}: {
  config: Record<string, unknown>;
  setConfig: (c: Record<string, unknown>) => void;
  paramNames: string[];
  httpHosts: string[];
}) {
  const set = (key: string, value: unknown) => setConfig({ ...config, [key]: value });

  const [headerRows, setHeaderRows] = useState<KVRow[]>(
    () => recordToKv((config.headers as Record<string, unknown>) ?? {}),
  );

  const updateHeaders = (rows: KVRow[]) => {
    setHeaderRows(rows);
    set("headers", kvToRecord(rows));
  };

  const url = getStr(config, "url");
  const urlHost = (() => {
    try { return new URL(url).hostname; } catch { return ""; }
  })();
  const hostWarning = url && urlHost && !httpHosts.some((h) => h.toLowerCase() === urlHost.toLowerCase());

  return (
    <div className="grid gap-4">
      <div className="grid gap-1.5">
        <Label htmlFor="http-url">URL</Label>
        <Input
          id="http-url"
          value={url}
          onChange={(e) => set("url", e.target.value)}
          placeholder="https://api.example.com/endpoint"
        />
        {hostWarning && (
          <p className="text-xs text-amber-600 dark:text-amber-400">
            主机 <code>{urlHost}</code> 不在白名单（http_hosts）里，保存时会被校验拒绝。
            请在"安全与通知"面板里添加后再保存。
          </p>
        )}
        <p className="text-xs text-muted-foreground">
          URL 不支持 <code>{"{{参数}}"}</code> 插值（系统禁止）。如需动态内容，放在 Body 里。
        </p>
      </div>

      <div className="grid gap-1.5">
        <Label htmlFor="http-method">方法</Label>
        <Select value={getStr(config, "method") || "POST"} onValueChange={(v) => set("method", v)}>
          <SelectTrigger id="http-method">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {METHODS.map((m) => (
              <SelectItem key={m} value={m}>{m}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="grid gap-1.5">
        <Label>请求头 (Headers)</Label>
        <KeyValueRows
          rows={headerRows}
          onChange={updateHeaders}
          keyLabel="Header 名"
          valueLabel="值"
          keyPlaceholder="Authorization"
          valuePlaceholder="Bearer token"
          addLabel="添加 Header"
        />
        <p className="text-xs text-muted-foreground">Header 值为静态字符串，不支持 <code>{"{{参数}}"}</code>。</p>
      </div>

      <div className="grid gap-1.5">
        <div className="flex items-center justify-between">
          <Label htmlFor="http-body">Body</Label>
        </div>
        <Textarea
          id="http-body"
          className="min-h-28 resize-y font-mono text-xs"
          value={getStr(config, "body")}
          onChange={(e) => set("body", e.target.value)}
          placeholder={'{"message":"{{message}}","level":"info"}'}
        />
        <ParamPicker
          paramNames={paramNames}
          onInsert={(s) => set("body", getStr(config, "body") + s)}
        />
        <p className="text-xs text-muted-foreground">
          Body 支持 <code>{"{{参数名}}"}</code> 插值。点击上方参数芯片可插入。留空则发送空 Body。
        </p>
      </div>
    </div>
  );
}
