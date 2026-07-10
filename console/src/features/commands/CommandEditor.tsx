import { useEffect, useState } from "react";
import { MagicWand } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectSeparator, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { ParameterEditor, type ParameterRow, rowsToParams, paramsToRows } from "./ParameterEditor";
import { EmailPreview } from "./EmailPreview";
import { HttpConfig } from "./config/HttpConfig";
import { HTTP_TEMPLATES, type HTTPTemplate } from "./config/httpTemplates";
import { HttpRequestConfig } from "./config/HttpRequestConfig";
import { WebhookConfig } from "./config/WebhookConfig";
import { WorkflowConfig } from "./config/WorkflowConfig";
import { QueueConfig } from "./config/QueueConfig";
import { ShellConfig } from "./config/ShellConfig";
import { GenericConfig } from "./config/GenericConfig";
import type { CommandDetail } from "@/lib/api";

const STABLE_HANDLERS = ["http", "http_request", "webhook", "workflow", "queue"];
const EXPERIMENTAL_HANDLERS = ["plugin", "shell", "agent", "mcp"];
const NAME_PATTERN = /^[a-z0-9][a-z0-9_-]*$/;

const HANDLER_DESC: Record<string, string> = {
  http:    "向外部 HTTP 接口发送请求，支持自定义方法、Header 和 Body 模板。",
  http_request: "邮件正文就是 HTTP 请求报文，按原始请求转发到允许的目标。",
  webhook: "推送带 HMAC 签名的 JSON 信封到外部 Webhook，参数以结构化对象传入。",
  workflow:"按顺序调用其他已声明命令，任意步骤失败则中止。",
  queue:   "将目标命令排入持久队列异步执行，失败自动重试。",
  plugin:  "【实验性】通过 stdin/stdout JSON 协议调用本地插件可执行文件。",
  shell:   "【实验性】执行本地命令，stdout+stderr 作为回复返回。",
  agent:   "【实验性】调用 Agent，配置随实现而定。",
  mcp:     "【实验性】调用 MCP 工具，配置随实现而定。",
};

export function CommandEditor({
  open,
  initial,
  existingCommands,
  httpHosts,
  token,
  onAddHTTPHost,
  onClose,
  onSave,
}: {
  open: boolean;
  initial: CommandDetail | null;
  existingCommands: CommandDetail[];
  httpHosts: string[];
  token: string;
  onAddHTTPHost: (host: string) => void;
  onClose: () => void;
  onSave: (command: CommandDetail) => void;
}) {
  const isNew = initial === null;
  const [tab, setTab] = useState("basic");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [handler, setHandler] = useState("http");
  const [paramRows, setParamRows] = useState<ParameterRow[]>([]);
  const [config, setConfig] = useState<Record<string, unknown>>({});
  const [configRevision, setConfigRevision] = useState(0);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setTab("basic");
    setError(null);
    setName(initial?.name ?? "");
    setDescription(initial?.description ?? "");
    setHandler(initial?.handler ?? "http");
    setParamRows(paramsToRows(initial?.parameters));
    setConfig(initial?.config ? { ...(initial.config as Record<string, unknown>) } : {});
    setConfigRevision(0);
  }, [open, initial]);

  const existingNames = existingCommands.map((c) => c.name);
  const paramNames = paramRows.filter((r) => r.name.trim()).map((r) => r.name.trim());

  const applyHTTPTemplate = (template: HTTPTemplate, options?: { fillBasics?: boolean }) => {
    setHandler("http");
    setConfig({ ...template.config });
    setConfigRevision((revision) => revision + 1);
    if (options?.fillBasics) {
      setName((current) => current || template.commandName);
      setDescription((current) => current || template.commandDescription);
    }
    setParamRows((rows) => {
      const existing = new Set(rows.map((row) => row.name.trim()).filter(Boolean));
      const missing = template.params.filter((row) => !existing.has(row.name));
      return [...rows, ...missing];
    });
  };

  const submit = () => {
    if (!NAME_PATTERN.test(name) || name === "help") {
      setError("命令名需匹配 ^[a-z0-9][a-z0-9_-]*$，且不能为 help");
      setTab("basic");
      return;
    }
    if (isNew && existingNames.includes(name)) {
      setError("已存在同名命令");
      setTab("basic");
      return;
    }
    const parameters = rowsToParams(paramRows);
    const cleanConfig = Object.fromEntries(
      Object.entries(config).filter(([, v]) => v !== "" && v !== undefined && v !== null),
    );
    onSave({
      name,
      description: description || undefined,
      handler,
      parameters,
      config: Object.keys(cleanConfig).length > 0 ? cleanConfig : undefined,
    });
  };

  // configKey unmounts and remounts the handler-specific form each time the
  // editor opens or switches to a different command, resetting any local row
  // state (headers, args, env) that lives inside the form component.
  const configKey = `${handler}:${isNew ? "new" : (initial?.name ?? "edit")}:${open ? "open" : "closed"}:${configRevision}`;

  const configForm = () => {
    switch (handler) {
      case "http":
        return (
          <HttpConfig
            key={configKey}
            config={config}
            setConfig={setConfig}
            paramNames={paramNames}
            paramRows={paramRows}
            httpHosts={httpHosts}
            onApplyTemplate={applyHTTPTemplate}
            onAddHTTPHost={onAddHTTPHost}
          />
        );
      case "http_request":
        return <HttpRequestConfig key={configKey} config={config} setConfig={setConfig} />;
      case "webhook":
        return <WebhookConfig key={configKey} config={config} setConfig={setConfig} httpHosts={httpHosts} />;
      case "workflow":
        return <WorkflowConfig key={configKey} config={config} setConfig={setConfig} existingCommands={existingCommands} paramNames={paramNames} />;
      case "queue":
        return <QueueConfig key={configKey} config={config} setConfig={setConfig} existingCommands={existingCommands} />;
      case "shell":
      case "plugin":
        return <ShellConfig key={configKey} config={config} setConfig={setConfig} paramNames={paramNames} handler={handler} />;
      default:
        return <GenericConfig key={configKey} config={config} setConfig={setConfig} handler={handler} />;
    }
  };

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="flex max-h-[90vh] flex-col gap-3 sm:max-w-3xl">
        <DialogHeader className="shrink-0">
          <DialogTitle>{isNew ? "新建命令" : `编辑命令 ${initial?.name}`}</DialogTitle>
          <DialogDescription>
            保存后经完整校验写入配置并热重载。密钥请用 <code>{"${ENV_VAR}"}</code> 引用。
          </DialogDescription>
        </DialogHeader>

        <Tabs value={tab} onValueChange={setTab} className="flex min-h-0 flex-1 flex-col">
          <TabsList className="shrink-0 w-full">
            <TabsTrigger value="basic" className="flex-1">基本信息</TabsTrigger>
            <TabsTrigger value="params" className="flex-1">
              参数声明{paramRows.length > 0 ? ` (${paramRows.length})` : ""}
            </TabsTrigger>
            <TabsTrigger value="config" className="flex-1">处理器配置</TabsTrigger>
            <TabsTrigger value="preview" className="flex-1">邮件示例</TabsTrigger>
          </TabsList>

          {/* Tab 1 — 基本信息 */}
          <TabsContent value="basic" className="min-h-0 overflow-y-auto pr-1">
            <div className="grid gap-4 py-2">
              {isNew && (
                <div className="grid gap-2 rounded-lg border border-border bg-muted/20 p-3">
                  <div>
                    <Label className="text-sm">从 API Action 模板开始</Label>
                    <p className="mt-0.5 text-xs text-muted-foreground">选中后填入 HTTP 命令、参数和配置，接着核对请求预览。</p>
                  </div>
                  <div className="grid gap-2 sm:grid-cols-3">
                    {HTTP_TEMPLATES.map((template) => (
                      <button
                        key={template.id}
                        type="button"
                        className="rounded-lg border border-border bg-background p-3 text-left transition hover:border-primary/40 hover:bg-primary/5"
                        onClick={() => {
                          applyHTTPTemplate(template, { fillBasics: true });
                          setTab("config");
                        }}
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
              )}
              <div className="grid gap-1.5">
                <Label htmlFor="cmd-name">命令名称</Label>
                <Input
                  id="cmd-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="deploy"
                  disabled={!isNew}
                  className="font-mono"
                />
                {!isNew && <p className="text-xs text-muted-foreground">命令名在创建后不可修改。</p>}
                {isNew && (
                  <p className="text-xs text-muted-foreground">
                    邮件触发标识。只能小写字母、数字、下划线或连字符，例如 deploy、send-report。
                  </p>
                )}
              </div>

              <div className="grid gap-1.5">
                <Label htmlFor="cmd-desc">说明</Label>
                <Input
                  id="cmd-desc"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="触发一次部署"
                />
                <p className="text-xs text-muted-foreground">
                  显示在 help 目录，让发件人知道该命令做什么。
                </p>
              </div>

              <div className="grid gap-1.5">
                <Label>处理器类型</Label>
                <Select value={handler} onValueChange={(v) => { setHandler(v); setConfig({}); }}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {STABLE_HANDLERS.map((h) => (
                      <SelectItem key={h} value={h}>{h}</SelectItem>
                    ))}
                    <SelectSeparator />
                    {EXPERIMENTAL_HANDLERS.map((h) => (
                      <SelectItem key={h} value={h}>{h} (experimental)</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">{HANDLER_DESC[handler]}</p>
                {EXPERIMENTAL_HANDLERS.includes(handler) && (
                  <p className="text-xs text-amber-600 dark:text-amber-400">
                    实验性处理器需要在 runtime 配置里开启 enable_experimental。
                  </p>
                )}
              </div>
            </div>
          </TabsContent>

          {/* Tab 2 — 参数声明 */}
          <TabsContent value="params" className="min-h-0 overflow-y-auto pr-1">
            <div className="py-2">
              <p className="mb-3 text-xs text-muted-foreground">
                声明邮件可以传入的参数。类型控制校验，"必填"控制是否必须提供，
                "脱敏"会从审计日志和队列任务里隐去该参数的值（脱敏参数不能用于 queue 命令）。
              </p>
              <ParameterEditor rows={paramRows} onChange={setParamRows} />
            </div>
          </TabsContent>

          {/* Tab 3 — 处理器配置 */}
          <TabsContent value="config" className="min-h-0 overflow-y-auto pr-1">
            <div className="py-2">{configForm()}</div>
          </TabsContent>

          {/* Tab 4 — 邮件示例 */}
          <TabsContent value="preview" className="min-h-0 overflow-y-auto pr-1">
            <EmailPreview name={name} paramRows={paramRows} token={token} />
          </TabsContent>
        </Tabs>

        {error && (
          <p role="alert" className="shrink-0 text-sm text-destructive">
            {error}
          </p>
        )}

        <DialogFooter className="shrink-0">
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button onClick={submit}>{isNew ? "添加命令" : "保存更改"}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
