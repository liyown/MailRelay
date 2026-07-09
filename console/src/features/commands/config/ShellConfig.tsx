import { useState } from "react";
import { Plus, Trash } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { KeyValueRows, recordToKv, kvToRecord, type KVRow } from "../KeyValueRows";
import { ParamPicker } from "../ParamPicker";

function getStr(config: Record<string, unknown>, key: string): string {
  const v = config[key];
  return typeof v === "string" ? v : "";
}
function getArr(config: Record<string, unknown>, key: string): string[] {
  const v = config[key];
  return Array.isArray(v) ? (v as unknown[]).map(String) : [];
}

export function ShellConfig({
  config,
  setConfig,
  paramNames,
  handler,
}: {
  config: Record<string, unknown>;
  setConfig: (c: Record<string, unknown>) => void;
  paramNames: string[];
  handler: "shell" | "plugin";
}) {
  const set = (key: string, value: unknown) => setConfig({ ...config, [key]: value });

  const [argRows, setArgRows] = useState<string[]>(() => getArr(config, "args"));
  const [envRows, setEnvRows] = useState<KVRow[]>(
    () => recordToKv((config.env as Record<string, unknown>) ?? {}),
  );

  const updateArgs = (rows: string[]) => {
    setArgRows(rows);
    set("args", rows.filter((a) => a.trim()));
  };
  const updateEnv = (rows: KVRow[]) => {
    setEnvRows(rows);
    set("env", kvToRecord(rows));
  };

  return (
    <div className="grid gap-4">
      <p className="text-xs text-muted-foreground">
        {handler === "plugin"
          ? "Plugin 在 stdin 传入结构化 JSON（包含参数），并从 stdout 读取 JSON 格式的执行结果。"
          : "Shell 执行可执行文件，stdout+stderr 作为命令回复内容返回。"}
        &nbsp;<strong>实验性功能</strong>，需要在 runtime 配置里开启 enable_experimental。
        参数（args / env 值）支持 <code>{"{{参数名}}"}</code>。
      </p>

      <div className="grid gap-1.5">
        <Label htmlFor="sh-exe">可执行文件路径（绝对路径）</Label>
        <Input
          id="sh-exe"
          value={getStr(config, "executable")}
          onChange={(e) => set("executable", e.target.value)}
          placeholder="/usr/local/bin/my-script"
          className="font-mono text-xs"
        />
      </div>

      <div className="grid gap-1.5">
        <Label>命令行参数 (args)</Label>
        <div className="grid gap-2">
          {argRows.map((arg, i) => (
            <div key={i} className="flex items-center gap-1.5">
              <Input
                value={arg}
                placeholder={`参数 ${i + 1}，可用 {{参数名}}`}
                onChange={(e) => {
                  const next = argRows.map((a, j) => (j === i ? e.target.value : a));
                  updateArgs(next);
                }}
                className="font-mono text-xs"
              />
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={() => updateArgs(argRows.filter((_, j) => j !== i))}
                aria-label="删除"
              >
                <Trash className="size-3.5" />
              </Button>
            </div>
          ))}
          <Button variant="outline" size="sm" className="w-fit" onClick={() => updateArgs([...argRows, ""])}>
            <Plus className="size-3.5" />
            添加参数
          </Button>
        </div>
        <ParamPicker
          paramNames={paramNames}
          onInsert={(s) => updateArgs([...argRows, s])}
        />
      </div>

      <div className="grid gap-1.5">
        <Label htmlFor="sh-wd">工作目录</Label>
        <Input
          id="sh-wd"
          value={getStr(config, "working_dir")}
          onChange={(e) => set("working_dir", e.target.value)}
          placeholder="/ （默认）"
          className="font-mono text-xs"
        />
      </div>

      <div className="grid gap-1.5">
        <Label>环境变量 (env)</Label>
        <KeyValueRows
          rows={envRows}
          onChange={updateEnv}
          keyLabel="变量名"
          valueLabel="值（可用 {{参数名}}）"
          keyPlaceholder="MY_VAR"
          valuePlaceholder="{{message}} 或固定值"
          addLabel="添加环境变量"
        />
      </div>
    </div>
  );
}
