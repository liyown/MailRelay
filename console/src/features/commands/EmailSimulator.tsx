import { useEffect, useState } from "react";
import { CheckCircle, Play, WarningCircle } from "@phosphor-icons/react";
import { useMutation } from "@tanstack/react-query";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { api, type CommandDetail, type MailPreview } from "@/lib/api";

const ERROR_HINTS: Record<string, string> = {
  parse: "检查 From、Subject 和正文格式；纯文本正文每行都应为 key=value。",
  sender: "把 From 地址加入“安全与配置”的发件人白名单。",
  token: "检查正文中的 _token，必须与当前 security.token 一致。",
  self_message: "发件人不能与 MailRelay 的 SMTP 发件地址相同。",
  unknown_command: "检查邮件主题的第一个词，它必须是已声明的命令名。",
  invalid_parameters: "检查参数名、必填参数和声明的类型。",
  authentication: "检查发件人白名单和 Token。",
};

function initialMail(commands: CommandDetail[], token: string, allow: string[]) {
  const command = commands[0];
  const params = Object.entries(command?.parameters ?? {})
    .filter(([, parameter]) => parameter.required)
    .map(([name, parameter]) => `${name}=${parameter.example ?? `your-${name}`}`);
  return [
    `From: ${allow[0] ?? "you@example.com"}`,
    `Subject: ${command?.name ?? "command-name"}`,
    "",
    `_token=${token || "your-token"}`,
    ...params,
  ].join("\n");
}

function Result({ value }: { value: MailPreview }) {
  if (value.accepted) {
    return (
      <div className="rounded-lg border border-emerald-500/30 bg-emerald-500/5 p-3 text-sm">
        <div className="flex items-center gap-2 font-medium text-emerald-700 dark:text-emerald-300">
          <CheckCircle className="size-4" weight="fill" />
          会进入命令处理阶段
        </div>
        <p className="mt-1 text-xs text-muted-foreground">
          命令 <code>{value.command}</code> 将由 <code>{value.handler}</code> 处理；已解析参数：{value.parameters?.join("、") || "无"}。
        </p>
      </div>
    );
  }
  const hint = ERROR_HINTS[value.error_kind ?? ""] ?? "检查运行日志或执行记录中的错误分类。";
  return (
    <div className="rounded-lg border border-destructive/35 bg-destructive/5 p-3 text-sm">
      <div className="flex items-center gap-2 font-medium text-destructive">
        <WarningCircle className="size-4" weight="fill" />
        会在 {value.stage || "检查"} 阶段被拒绝
        {value.error_kind && <Badge variant="outline" className="border-destructive/30 text-destructive">{value.error_kind}</Badge>}
      </div>
      <p className="mt-1 text-xs text-muted-foreground">{hint}</p>
    </div>
  );
}

export function EmailSimulator({
  open,
  commands,
  token,
  allow,
  onClose,
}: {
  open: boolean;
  commands: CommandDetail[];
  token: string;
  allow: string[];
  onClose: () => void;
}) {
  const [raw, setRaw] = useState("");
  const preview = useMutation({ mutationFn: api.previewMail });

  useEffect(() => {
    if (!open) return;
    setRaw(initialMail(commands, token, allow));
    preview.reset();
  }, [open, commands, token, allow]);

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="flex max-h-[90vh] flex-col sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>模拟邮件</DialogTitle>
          <DialogDescription>使用当前运行配置解析、认证和校验邮件；不会执行命令、请求 API 或写入日志。</DialogDescription>
        </DialogHeader>
        <Textarea
          value={raw}
          onChange={(event) => setRaw(event.target.value)}
          spellCheck={false}
          className="min-h-72 resize-y font-mono text-xs leading-6"
          aria-label="原始邮件内容"
        />
        {preview.data && <Result value={preview.data} />}
        {preview.isError && <p role="alert" className="text-sm text-destructive">模拟失败，请稍后重试。</p>}
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>关闭</Button>
          <Button onClick={() => preview.mutate(raw)} disabled={!raw.trim() || preview.isPending}>
            <Play className="size-3.5" weight="fill" />
            {preview.isPending ? "正在检查..." : "检查邮件"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
