import { CheckCircle, ShieldCheck } from "@phosphor-icons/react";
import { CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { PageFrame } from "@/components/layout/PageFrame";
import { Panel, PanelTitle } from "@/components/common/Panel";
import { useSystem } from "@/hooks/queries";
import { formatDateTime, formatUptime } from "@/lib/format";

function Field({ label, value }: { label: string; value: string }) {
  return (
    <label className="grid gap-2 text-sm font-medium">
      <span>{label}</span>
      <Input value={value} readOnly className="bg-muted/40" />
    </label>
  );
}

const SECURITY_NOTES = [
  "邮箱凭据与授权码不会通过控制台 API 返回",
  "Command Token 与处理器密钥保持服务端隔离",
  "邮件正文和原始依赖错误不会进入可见日志",
  "运维写操作（重放死信）需要有效会话与 CSRF 令牌",
];

const SECURITY_STATUS = ["签名会话已启用", "CSRF 防护已启用", "敏感字段默认脱敏", "API 响应禁止缓存"];

export function SettingsPage({ security = false }: { security?: boolean }) {
  const system = useSystem();
  return (
    <PageFrame
      title={security ? "邮箱与安全" : "系统设置"}
      description={security ? "连接状态、信任边界与敏感配置" : "控制台与运行环境信息（只读）"}
    >
      <div className="grid gap-4 xl:grid-cols-[1fr_340px]">
        <Panel>
          <PanelTitle>{security ? "安全边界" : "运行信息"}</PanelTitle>
          <CardContent className="space-y-5 p-5">
            {security ? (
              <div className="space-y-4">
                {SECURITY_NOTES.map((note) => (
                  <div key={note} className="flex items-start gap-3 rounded-lg border border-border p-4 text-sm">
                    <ShieldCheck className="mt-0.5 size-5 shrink-0 text-emerald-600" />
                    {note}
                  </div>
                ))}
              </div>
            ) : (
              <>
                <Field label="启动时间" value={system.data ? formatDateTime(system.data.started_at) : "—"} />
                <Field label="运行时长" value={system.data ? formatUptime(system.data.uptime_seconds) : "—"} />
                <Field label="已声明 Command" value={String(system.data?.command_count ?? "—")} />
              </>
            )}
          </CardContent>
        </Panel>
        <Panel className="h-fit">
          <PanelTitle>安全状态</PanelTitle>
          <CardContent className="space-y-4 p-5">
            {SECURITY_STATUS.map((status) => (
              <div key={status} className="flex items-center gap-2 text-sm">
                <CheckCircle weight="fill" className="text-emerald-600" />
                {status}
              </div>
            ))}
          </CardContent>
        </Panel>
      </div>
    </PageFrame>
  );
}
