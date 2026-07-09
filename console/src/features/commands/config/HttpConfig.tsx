import { useState } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { KeyValueRows, recordToKv, kvToRecord, type KVRow } from "../KeyValueRows";
import { ParamPicker } from "../ParamPicker";

const METHODS = ["POST", "GET", "PUT", "PATCH", "DELETE"];
const METHODS_WITH_BODY = new Set(["POST", "PUT", "PATCH"]);

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
  const [queryRows, setQueryRows] = useState<KVRow[]>(
    () => recordToKv((config.query as Record<string, unknown>) ?? {}),
  );

  const updateHeaders = (rows: KVRow[]) => {
    setHeaderRows(rows);
    set("headers", kvToRecord(rows));
  };
  const updateQuery = (rows: KVRow[]) => {
    setQueryRows(rows);
    set("query", kvToRecord(rows));
  };

  const appendToLastQueryValue = (value: string) => {
    const rows = queryRows.length > 0 ? queryRows : [{ key: "", value: "" }];
    const next = rows.map((row, index) =>
      index === rows.length - 1 ? { ...row, value: row.value + value } : row,
    );
    updateQuery(next);
  };

  const url = getStr(config, "url");
  const method = getStr(config, "method") || "POST";
  const showBody = METHODS_WITH_BODY.has(method);
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
          placeholder="https://api.example.com/users/{{user_id}}/notify"
        />
        <ParamPicker
          paramNames={paramNames}
          onInsert={(s) => set("url", url + s)}
        />
        {hostWarning && (
          <p className="text-xs text-amber-600 dark:text-amber-400">
            主机 <code>{urlHost}</code> 不在白名单（http_hosts）里，保存时会被校验拒绝。
            请在"安全与通知"面板里添加后再保存。
          </p>
        )}
        <p className="text-xs text-muted-foreground">
          Scheme、账号信息和 Host 必须保持静态；Path 和下方 Query 值支持 <code>{"{{参数名}}"}</code>。
        </p>
      </div>

      <div className="grid gap-1.5">
        <Label>URL Query 参数</Label>
        <KeyValueRows
          rows={queryRows}
          onChange={updateQuery}
          keyLabel="参数名"
          valueLabel="值"
          keyPlaceholder="q"
          valuePlaceholder="{{keyword}}"
          addLabel="添加 Query 参数"
        />
        <ParamPicker
          paramNames={paramNames}
          onInsert={appendToLastQueryValue}
        />
        <p className="text-xs text-muted-foreground">
          Query 值支持 <code>{"{{参数名}}"}</code> 插值，发送时会自动 URL encode 并追加到 URL。
        </p>
      </div>

      <div className="grid gap-1.5">
        <Label htmlFor="http-method">方法</Label>
        <Select
          value={method}
          onValueChange={(v) => {
            const next: Record<string, unknown> = { ...config, method: v };
            if (!METHODS_WITH_BODY.has(v)) delete next.body;
            setConfig(next);
          }}
        >
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

      {showBody && (
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
      )}
    </div>
  );
}
