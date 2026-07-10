import { useState } from "react";
import { MagicWand, Plus } from "@phosphor-icons/react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { KeyValueRows, recordToKv, kvToRecord, type KVRow } from "../KeyValueRows";
import { ParamPicker } from "../ParamPicker";
import type { ParameterRow } from "../ParameterEditor";
import { HTTP_TEMPLATES, type HTTPTemplate } from "./httpTemplates";

const METHODS = ["POST", "GET", "PUT", "PATCH", "DELETE"];
const METHODS_WITH_BODY = new Set(["POST", "PUT", "PATCH"]);

function getStr(config: Record<string, unknown>, key: string): string {
  const v = config[key];
  return typeof v === "string" ? v : "";
}

function fallbackExample(row: ParameterRow): string {
  if (row.example) return row.example;
  switch (row.type) {
    case "integer": return "42";
    case "number": return "3.14";
    case "boolean": return "true";
    default: return `sample-${row.name}`;
  }
}

function paramExamples(paramRows: ParameterRow[], paramNames: string[]): Record<string, string> {
  const examples = new Map<string, string>();
  for (const row of paramRows) {
    const name = row.name.trim();
    if (name) examples.set(name, fallbackExample(row));
  }
  for (const name of paramNames) {
    if (!examples.has(name)) examples.set(name, `sample-${name}`);
  }
  return Object.fromEntries(examples);
}

function expandWithExamples(value: string, examples: Record<string, string>): string {
  return value.replace(/\{\{([^{}]+)\}\}/g, (_, key: string) => examples[key] ?? `sample-${key}`);
}

function requestPreview(config: Record<string, unknown>, paramNames: string[], paramRows: ParameterRow[]) {
  const examples = paramExamples(paramRows, paramNames);
  const method = getStr(config, "method") || "POST";
  const expandedUrl = expandWithExamples(getStr(config, "url"), examples);
  let url: URL;
  try {
    url = new URL(expandedUrl);
  } catch {
    return "URL 填写完整后会显示请求预览。";
  }

  if (config.query && typeof config.query === "object" && !Array.isArray(config.query)) {
    for (const [key, value] of Object.entries(config.query as Record<string, unknown>)) {
      if (!key) continue;
      url.searchParams.set(key, expandWithExamples(String(value), examples));
    }
  }

  const headers: Record<string, string> = {};
  if (config.headers && typeof config.headers === "object" && !Array.isArray(config.headers)) {
    for (const [key, value] of Object.entries(config.headers as Record<string, unknown>)) {
      if (key) headers[key] = String(value);
    }
  }

  const body = METHODS_WITH_BODY.has(method) ? expandWithExamples(getStr(config, "body"), examples) : "";
  const hasContentType = Object.keys(headers).some((key) => key.toLowerCase() === "content-type");
  if (body && !hasContentType) headers["Content-Type"] = "application/json";

  const path = `${url.pathname}${url.search}`;
  const lines = [`${method} ${path || "/"} HTTP/1.1`, `Host: ${url.host}`];
  for (const [key, value] of Object.entries(headers)) lines.push(`${key}: ${value}`);
  if (body) lines.push("", body);
  return lines.join("\n");
}

export function HttpConfig({
  config,
  setConfig,
  paramNames,
  paramRows,
  httpHosts,
  onApplyTemplate,
  onAddHTTPHost,
}: {
  config: Record<string, unknown>;
  setConfig: (c: Record<string, unknown>) => void;
  paramNames: string[];
  paramRows: ParameterRow[];
  httpHosts: string[];
  onApplyTemplate?: (template: HTTPTemplate) => void;
  onAddHTTPHost?: (host: string) => void;
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
  const applyTemplate = (template: HTTPTemplate) => {
    setHeaderRows(recordToKv(template.config.headers as Record<string, unknown> | undefined));
    setQueryRows(recordToKv(template.config.query as Record<string, unknown> | undefined));
    onApplyTemplate?.(template);
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
  const preview = requestPreview(config, paramNames, paramRows);

  return (
    <div className="grid gap-4">
      <div className="grid gap-2 rounded-lg border border-border bg-muted/20 p-3">
        <div className="flex items-center justify-between gap-2">
          <div>
            <Label className="text-sm">API Action 模板</Label>
            <p className="mt-0.5 text-xs text-muted-foreground">选择后会覆盖 HTTP 配置，并补齐缺失的参数声明。</p>
          </div>
          <Badge variant="outline" className="shrink-0">HTTP</Badge>
        </div>
        <div className="grid gap-2 sm:grid-cols-3">
          {HTTP_TEMPLATES.map((template) => (
            <button
              key={template.id}
              type="button"
              className="rounded-lg border border-border bg-background p-3 text-left transition hover:border-primary/40 hover:bg-primary/5"
              onClick={() => applyTemplate(template)}
            >
              <span className="mb-1 flex items-center gap-1.5 text-sm font-medium">
                <MagicWand className="size-3.5 text-primary" />
                {template.title}
              </span>
              <span className="block text-xs leading-relaxed text-muted-foreground">{template.description}</span>
            </button>
          ))}
        </div>
      </div>

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
          <div className="flex flex-wrap items-center gap-2 text-xs text-amber-600 dark:text-amber-400">
            <span>主机 <code>{urlHost}</code> 不在白名单（http_hosts）里，保存时会被校验拒绝。</span>
            {onAddHTTPHost && (
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-7 border-amber-500/40 px-2 text-xs text-amber-700 hover:bg-amber-500/10 hover:text-amber-800 dark:text-amber-300 dark:hover:text-amber-200"
                onClick={() => onAddHTTPHost(urlHost)}
              >
                <Plus className="size-3.5" />
                添加到白名单
              </Button>
            )}
          </div>
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

      <div className="grid gap-1.5">
        <div className="flex items-center justify-between">
          <Label>请求预览</Label>
          <Badge variant="outline" className="font-normal">示例值</Badge>
        </div>
        <pre className="overflow-x-auto rounded-lg border border-border bg-muted/50 p-4 font-mono text-xs leading-6 whitespace-pre">
          {preview}
        </pre>
        <p className="text-xs text-muted-foreground">
          预览使用参数示例值渲染，保存后真实请求仍以收到的邮件参数为准。
        </p>
      </div>
    </div>
  );
}
