import { useEffect, useMemo, useState } from "react";
import { CaretLeft, CaretRight, Copy, Plus, PencilSimple, Trash } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { PageFrame } from "@/components/layout/PageFrame";
import { Panel, PanelTitle } from "@/components/common/Panel";
import { DataState } from "@/components/common/DataState";
import { MaturityBadge } from "@/components/common/StatusBadge";
import { ConfirmButton } from "@/components/common/ConfirmButton";
import { APIError, type CommandDetail, type ConfigDraft } from "@/lib/api";
import { useConfigDraft } from "@/hooks/queries";
import { useSaveConfig } from "@/hooks/useSaveConfig";
import { CommandEditor } from "./CommandEditor";

const STABLE_HANDLERS = ["http", "http_request", "webhook", "workflow", "queue"];
const HEADERS = ["命令", "处理器", "参数", ""];
const PAGE_SIZE = 8;

// Split on newlines, trim each line, drop blanks — used only on blur/save.
const linesToList = (text: string) =>
  text.split("\n").map((l) => l.trim()).filter(Boolean);

const listToLines = (list: string[]) => (list ?? []).join("\n");

const copyNameFor = (name: string, commands: CommandDetail[]) => {
  const names = new Set(commands.map((command) => command.name));
  const base = `${name}-copy`;
  if (!names.has(base)) return base;
  for (let i = 2; ; i += 1) {
    const candidate = `${base}-${i}`;
    if (!names.has(candidate)) return candidate;
  }
};

// A textarea that keeps its own raw string while the user is typing.
// It only commits (via linesToList) when focus leaves the field.
function ListTextarea({
  id,
  value,
  onChange,
  placeholder,
  className,
}: {
  id?: string;
  value: string[];
  onChange: (list: string[]) => void;
  placeholder?: string;
  className?: string;
}) {
  const [raw, setRaw] = useState(() => listToLines(value));

  // Sync from outside (e.g. discard / fresh load) only when the canonical
  // list actually changes, not on every render.
  const canonical = listToLines(value);
  useEffect(() => {
    setRaw(canonical);
  }, [canonical]);

  return (
    <Textarea
      id={id}
      className={className}
      value={raw}
      onChange={(e) => setRaw(e.target.value)}
      onBlur={() => onChange(linesToList(raw))}
      placeholder={placeholder}
    />
  );
}

export function CommandsPage({ csrf }: { csrf: string }) {
  const draftQuery = useConfigDraft();
  const save = useSaveConfig(csrf);
  const [draft, setDraft] = useState<ConfigDraft | null>(null);
  const [search, setSearch] = useState("");
  const [currentPage, setCurrentPage] = useState(1);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);

  useEffect(() => {
    if (draftQuery.data) setDraft(structuredClone(draftQuery.data));
  }, [draftQuery.data]);

  const dirty = useMemo(
    () => !!draft && JSON.stringify(draft) !== JSON.stringify(draftQuery.data),
    [draft, draftQuery.data],
  );

  const commands = draft?.commands ?? [];
  const filtered = commands
    .map((command, index) => ({ command, index }))
    .filter(({ command }) =>
      `${command.name} ${command.description ?? ""} ${command.handler}`
        .toLowerCase()
        .includes(search.toLowerCase()),
    );
  const pageCount = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const page = Math.min(currentPage, pageCount);
  const pageStart = (page - 1) * PAGE_SIZE;
  const pagedCommands = filtered.slice(pageStart, pageStart + PAGE_SIZE);
  const showingStart = filtered.length === 0 ? 0 : pageStart + 1;
  const showingEnd = Math.min(pageStart + PAGE_SIZE, filtered.length);

  useEffect(() => {
    setCurrentPage(1);
  }, [search]);

  useEffect(() => {
    setCurrentPage((prev) => Math.min(prev, pageCount));
  }, [pageCount]);

  const openNew = () => { setEditingIndex(null); setEditorOpen(true); };
  const openEdit = (index: number) => { setEditingIndex(index); setEditorOpen(true); };

  const applyCommand = (command: CommandDetail) => {
    setDraft((prev) => {
      if (!prev) return prev;
      const next = structuredClone(prev);
      if (editingIndex === null) next.commands.push(command);
      else next.commands[editingIndex] = command;
      return next;
    });
    setEditorOpen(false);
  };

  const removeCommand = (index: number) => {
    setDraft((prev) => {
      if (!prev) return prev;
      const next = structuredClone(prev);
      next.commands.splice(index, 1);
      return next;
    });
  };

  const copyCommand = (index: number) => {
    setDraft((prev) => {
      if (!prev) return prev;
      const next = structuredClone(prev);
      const source = next.commands[index];
      if (!source) return prev;
      const copy = structuredClone(source);
      copy.name = copyNameFor(source.name, next.commands);
      next.commands.splice(index + 1, 0, copy);
      return next;
    });
  };

  const setField = <K extends keyof ConfigDraft>(key: K, value: ConfigDraft[K]) =>
    setDraft((prev) => (prev ? { ...prev, [key]: value } : prev));

  const discard = () => draftQuery.data && setDraft(structuredClone(draftQuery.data));
  const saveError =
    save.error instanceof APIError ? save.error.message
    : save.error ? "保存失败，请稍后重试"
    : undefined;

  return (
    <PageFrame
      title="Command"
      description="预先声明、可审计的远程命令目录 — 保存后经完整校验写入配置并热重载"
      action={
        <div className="flex items-center gap-2">
          {dirty && (
            <Button variant="outline" onClick={discard} disabled={save.isPending}>
              放弃更改
            </Button>
          )}
          <Button onClick={() => draft && save.mutate(draft)} disabled={!dirty || save.isPending}>
            {save.isPending ? "正在保存..." : "保存更改"}
          </Button>
        </div>
      }
    >
      {saveError && (
        <p role="alert" className="mb-4 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {saveError}
        </p>
      )}

      <Tabs defaultValue="commands">
        <TabsList className="mb-5 grid !h-10 w-full max-w-md grid-cols-2 overflow-hidden rounded-lg border border-border bg-secondary/70 p-1 text-muted-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.55)] group-data-horizontal/tabs:!h-10">
          <TabsTrigger
            value="commands"
            className="!h-full w-full rounded-md px-3 !py-0 text-sm data-active:bg-card data-active:text-foreground data-active:shadow-sm"
          >
            <span>命令目录</span>
            {commands.length > 0 && (
              <span className="ml-1 rounded-full bg-primary/10 px-1.5 py-0.5 text-[11px] font-semibold leading-none text-primary">
                {commands.length}
              </span>
            )}
          </TabsTrigger>
          <TabsTrigger
            value="security"
            className="!h-full w-full rounded-md px-3 !py-0 text-sm data-active:bg-card data-active:text-foreground data-active:shadow-sm"
          >
            安全与配置
          </TabsTrigger>
        </TabsList>

        {/* ── Tab 1: 命令目录 ───────────────────────────────────────────── */}
        <TabsContent value="commands" className="min-h-0">
          <Panel className="flex h-[calc(100dvh-17rem)] min-h-0 flex-col overflow-hidden">
            <PanelTitle
              action={
                <div className="flex items-center gap-2">
                  <Input
                    className="w-[220px] bg-background/70"
                    placeholder="筛选命令..."
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                  />
                  <Button size="sm" onClick={openNew}>
                    <Plus />
                    新建命令
                  </Button>
                </div>
              }
            >
              命令目录
            </PanelTitle>
            <div className="flex min-h-0 flex-1 flex-col [&>*]:flex-1">
              <DataState
                isLoading={draftQuery.isPending}
                isError={draftQuery.isError}
                isEmpty={filtered.length === 0}
                emptyText={search ? "没有匹配的 Command" : "还没有命令，点击右上角新建命令开始"}
              >
                <div className="flex min-h-0 flex-1 flex-col">
                  <div className="min-h-0 flex-1 overflow-auto">
                    <Table className="min-w-[760px]">
                      <TableHeader>
                        <TableRow>
                          {HEADERS.map((h, i) => (
                            <TableHead
                              key={i}
                              className={
                                i === 0 ? "w-[48%] px-5 text-xs uppercase tracking-wide text-muted-foreground"
                                : i === 3 ? "px-5 text-right text-xs uppercase tracking-wide text-muted-foreground"
                                : "px-4 text-xs uppercase tracking-wide text-muted-foreground"
                              }
                            >
                              {h}
                            </TableHead>
                          ))}
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {pagedCommands.map(({ command, index }) => (
                          <TableRow key={command.name || index} className="group border-border/70 hover:bg-accent/35">
                            <TableCell className="px-5 py-4">
                              <div className="flex min-w-0 flex-col gap-1">
                                <div className="flex items-center gap-2">
                                  <code className="rounded-md border border-border bg-secondary/60 px-2 py-1 font-mono text-xs font-semibold text-foreground">
                                    {command.name}
                                  </code>
                                  <MaturityBadge maturity={STABLE_HANDLERS.includes(command.handler) ? "Stable" : "Experimental"} />
                                </div>
                                <p className="max-w-[34rem] truncate text-sm text-muted-foreground">
                                  {command.description || "未填写说明"}
                                </p>
                              </div>
                            </TableCell>
                            <TableCell className="px-4 py-4">
                              <Badge variant="outline" className="border-primary/20 bg-primary/5 font-mono text-[11px] text-primary">
                                {command.handler}
                              </Badge>
                            </TableCell>
                            <TableCell className="px-4 py-4">
                              <span className="inline-flex items-center gap-1 rounded-md bg-muted px-2 py-1 text-xs text-muted-foreground">
                                <span className="font-semibold text-foreground">{Object.keys(command.parameters ?? {}).length}</span>
                                个参数
                              </span>
                            </TableCell>
                            <TableCell className="px-5 py-4 text-right">
                              <div className="flex justify-end gap-1">
                                <Button
                                  variant="ghost"
                                  size="icon-sm"
                                  onClick={() => openEdit(index)}
                                  aria-label={`编辑命令 ${command.name}`}
                                  title="编辑"
                                >
                                  <PencilSimple className="size-3.5" />
                                </Button>
                                <Button
                                  variant="ghost"
                                  size="icon-sm"
                                  onClick={() => copyCommand(index)}
                                  aria-label={`复制命令 ${command.name}`}
                                  title="复制"
                                >
                                  <Copy className="size-3.5" />
                                </Button>
                                <ConfirmButton
                                  trigger={
                                    <Button
                                      variant="ghost"
                                      size="icon-sm"
                                      className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                                      aria-label={`删除命令 ${command.name}`}
                                      title="删除"
                                    >
                                      <Trash className="size-3.5" />
                                    </Button>
                                  }
                                  title={`删除命令 ${command.name}`}
                                  description="从草稿中移除该命令。点击保存更改后才会写入配置并热重载。"
                                  confirmText="删除"
                                  onConfirm={() => removeCommand(index)}
                                />
                              </div>
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                  {filtered.length > PAGE_SIZE && (
                    <div className="shrink-0 border-t border-border bg-card px-5 py-3">
                      <div className="flex flex-wrap items-center justify-between gap-3">
                        <p className="text-xs text-muted-foreground">
                          显示 {showingStart}-{showingEnd} / {filtered.length} 条
                        </p>
                        <div className="flex items-center gap-2">
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => setCurrentPage((prev) => Math.max(1, prev - 1))}
                            disabled={page <= 1}
                          >
                            <CaretLeft className="size-3.5" />
                            上一页
                          </Button>
                          <span className="min-w-16 text-center text-xs text-muted-foreground">
                            {page} / {pageCount}
                          </span>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => setCurrentPage((prev) => Math.min(pageCount, prev + 1))}
                            disabled={page >= pageCount}
                          >
                            下一页
                            <CaretRight className="size-3.5" />
                          </Button>
                        </div>
                      </div>
                    </div>
                  )}
                </div>
            </DataState>
            </div>
          </Panel>
        </TabsContent>

        {/* ── Tab 2: 安全与配置 ─────────────────────────────────────────── */}
        <TabsContent value="security" className="space-y-4">
          {/* 认证 */}
          <Panel>
            <PanelTitle>认证</PanelTitle>
            <div className="grid gap-5 p-5 md:grid-cols-2">
              <div className="grid gap-1.5">
                <Label htmlFor="security-token">Token（security.token）</Label>
                <Input
                  id="security-token"
                  className="font-mono text-xs"
                  value={draft?.token ?? ""}
                  onChange={(e) => setField("token", e.target.value)}
                  placeholder="${MAILRELAY_TOKEN} 或直接填值"
                />
                <p className="text-xs text-muted-foreground">
                  所有邮件触发请求都必须携带此 Token。建议用{" "}
                  <code className="rounded bg-muted px-1">${"{"}ENV_VAR{"}"}</code>{" "}
                  引用环境变量，避免明文存储。
                </p>
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="security-allow">发件人白名单（security.allow）</Label>
                <ListTextarea
                  id="security-allow"
                  className="min-h-28 font-mono text-xs"
                  value={draft?.allow ?? []}
                  onChange={(list) => setField("allow", list)}
                  placeholder={"ops@example.com\ndev@example.com"}
                />
                <p className="text-xs text-muted-foreground">
                  每行一个邮箱地址，只有白名单内的发件人才能触发命令。
                </p>
              </div>
            </div>
          </Panel>

          {/* 网络与通知 */}
          <Panel>
            <PanelTitle>网络与通知</PanelTitle>
            <div className="grid gap-5 p-5 md:grid-cols-2">
              <div className="grid gap-1.5">
                <Label htmlFor="http-hosts">外发主机白名单（http_hosts）</Label>
                <ListTextarea
                  id="http-hosts"
                  className="min-h-28 font-mono text-xs"
                  value={draft?.http_hosts ?? []}
                  onChange={(list) => setField("http_hosts", list)}
                  placeholder={"api.example.com\nhooks.example.com"}
                />
                <p className="text-xs text-muted-foreground">
                  每行一个域名，http/webhook 处理器只能向这些主机发送请求。
                </p>
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="catalog-notify">命令变更通知（catalog_notify）</Label>
                <ListTextarea
                  id="catalog-notify"
                  className="min-h-28 font-mono text-xs"
                  value={draft?.catalog_notify ?? []}
                  onChange={(list) => setField("catalog_notify", list)}
                  placeholder={"ops@example.com\ndev@example.com"}
                />
                <p className="text-xs text-muted-foreground">
                  命令目录变更时自动发通知邮件到这些地址，每行一个。
                </p>
              </div>
            </div>
          </Panel>
        </TabsContent>
      </Tabs>

      <CommandEditor
        open={editorOpen}
        initial={editingIndex === null ? null : commands[editingIndex] ?? null}
        existingCommands={commands}
        httpHosts={draft?.http_hosts ?? []}
        token={draft?.token ?? ""}
        onClose={() => setEditorOpen(false)}
        onSave={applyCommand}
      />
    </PageFrame>
  );
}
