import { useEffect, useMemo, useState } from "react";
import { Plus, PencilSimple, Trash } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
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

const STABLE_HANDLERS = ["http", "webhook", "workflow", "queue"];
const HEADERS = ["命令标识", "说明", "处理器", "成熟度", "参数数", ""];

const linesToList = (text: string) =>
  text
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
const listToLines = (list: string[]) => (list ?? []).join("\n");

export function CommandsPage({ csrf }: { csrf: string }) {
  const draftQuery = useConfigDraft();
  const save = useSaveConfig(csrf);
  const [draft, setDraft] = useState<ConfigDraft | null>(null);
  const [search, setSearch] = useState("");
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);

  // Reset the local working copy whenever fresh server state arrives (initial
  // load and after a successful save invalidates the query).
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
      `${command.name} ${command.description ?? ""} ${command.handler}`.toLowerCase().includes(search.toLowerCase()),
    );

  const openNew = () => {
    setEditingIndex(null);
    setEditorOpen(true);
  };
  const openEdit = (index: number) => {
    setEditingIndex(index);
    setEditorOpen(true);
  };

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

  const setHosts = (text: string) => setDraft((prev) => (prev ? { ...prev, http_hosts: linesToList(text) } : prev));
  const setNotify = (text: string) => setDraft((prev) => (prev ? { ...prev, catalog_notify: linesToList(text) } : prev));
  const discard = () => draftQuery.data && setDraft(structuredClone(draftQuery.data));

  const saveError = save.error instanceof APIError ? save.error.message : save.error ? "保存失败，请稍后重试" : undefined;

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
      <Panel className="mb-6">
        <PanelTitle
          action={
            <div className="flex items-center gap-2">
              <Input className="w-[220px]" placeholder="筛选命令..." value={search} onChange={(event) => setSearch(event.target.value)} />
              <Button size="sm" onClick={openNew}>
                <Plus />
                新建命令
              </Button>
            </div>
          }
        >
          命令目录
        </PanelTitle>
        <DataState
          isLoading={draftQuery.isPending}
          isError={draftQuery.isError}
          isEmpty={filtered.length === 0}
          emptyText="没有匹配的 Command"
        >
          <Table>
            <TableHeader>
              <TableRow>
                {HEADERS.map((header, index) => (
                  <TableHead key={index}>{header}</TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map(({ command, index }) => (
                <TableRow key={command.name || index}>
                  <TableCell className="font-mono text-xs">{command.name}</TableCell>
                  <TableCell>{command.description || "—"}</TableCell>
                  <TableCell>{command.handler}</TableCell>
                  <TableCell>
                    <MaturityBadge maturity={STABLE_HANDLERS.includes(command.handler) ? "Stable" : "Experimental"} />
                  </TableCell>
                  <TableCell>{Object.keys(command.parameters ?? {}).length}</TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-1">
                      <Button variant="outline" size="sm" onClick={() => openEdit(index)}>
                        <PencilSimple />
                        编辑
                      </Button>
                      <ConfirmButton
                        trigger={
                          <Button variant="destructive" size="sm">
                            <Trash />
                            删除
                          </Button>
                        }
                        title={`删除命令 ${command.name}`}
                        description="从草稿中移除该命令。点击“保存更改”后才会写入配置并热重载。"
                        confirmText="删除"
                        onConfirm={() => removeCommand(index)}
                      />
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </DataState>
      </Panel>

      <Panel>
        <PanelTitle>安全与通知</PanelTitle>
        <div className="grid gap-4 p-5 md:grid-cols-2">
          <div className="grid gap-1.5">
            <Label htmlFor="http-hosts">外发主机白名单（http_hosts，每行一个）</Label>
            <Textarea
              id="http-hosts"
              className="min-h-28 font-mono text-xs"
              value={listToLines(draft?.http_hosts ?? [])}
              onChange={(event) => setHosts(event.target.value)}
              placeholder="api.example.com"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="catalog-notify">命令通知列表（catalog_notify，每行一个）</Label>
            <Textarea
              id="catalog-notify"
              className="min-h-28 font-mono text-xs"
              value={listToLines(draft?.catalog_notify ?? [])}
              onChange={(event) => setNotify(event.target.value)}
              placeholder="ops@example.com"
            />
          </div>
        </div>
      </Panel>

      <CommandEditor
        open={editorOpen}
        initial={editingIndex === null ? null : commands[editingIndex] ?? null}
        existingCommands={commands}
        httpHosts={draft?.http_hosts ?? []}
        onClose={() => setEditorOpen(false)}
        onSave={applyCommand}
      />
    </PageFrame>
  );
}
