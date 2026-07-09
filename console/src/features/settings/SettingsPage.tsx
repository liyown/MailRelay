import { CheckCircle, GitBranch, HardDrive, Lightning, ShieldCheck, Timer } from "@phosphor-icons/react";
import { CardContent } from "@/components/ui/card";
import { PageFrame } from "@/components/layout/PageFrame";
import { Panel, PanelTitle } from "@/components/common/Panel";
import { useSystem, useDashboard } from "@/hooks/queries";
import { formatDateTime, formatUptime, formatSeconds } from "@/lib/format";

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 border-b border-border/60 py-3 text-sm last:border-0">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-mono text-xs text-right">{value ?? "—"}</span>
    </div>
  );
}

const SECURITY_STATUS = [
  "签名会话已启用",
  "CSRF 防护已启用",
  "敏感字段默认脱敏",
  "API 响应禁止缓存",
  "邮箱凭据不经控制台 API 返回",
  "Command Token 保持服务端隔离",
];

export function SettingsPage() {
  const system = useSystem();
  const dashboard = useDashboard("24h");
  const s = system.data;
  const d = dashboard.data;

  return (
    <PageFrame title="系统设置" description="运行环境、构建信息与安全状态（只读）">
      <div className="grid gap-4 xl:grid-cols-[1fr_320px]">
        <div className="space-y-4">
          {/* Runtime */}
          <Panel>
            <PanelTitle><Timer className="size-4 text-muted-foreground" />运行状态</PanelTitle>
            <CardContent className="px-5 pb-5 pt-0">
              <Row label="启动时间" value={formatDateTime(s?.started_at)} />
              <Row label="运行时长" value={s ? formatUptime(s.uptime_seconds) : undefined} />
              <Row label="已声明 Command" value={s?.command_count ?? "—"} />
              <Row label="活跃处理器" value={d?.active_handlers ?? "—"} />
            </CardContent>
          </Panel>

          {/* Execution stats */}
          <Panel>
            <PanelTitle><Lightning className="size-4 text-muted-foreground" />执行统计（近 24h）</PanelTitle>
            <CardContent className="px-5 pb-5 pt-0">
              <Row label="总执行次数" value={d?.execution_count ?? "—"} />
              <Row label="成功次数" value={d?.success_count ?? "—"} />
              <Row
                label="成功率"
                value={
                  d ? (
                    <span className={d.success_rate >= 0.9 ? "text-emerald-600" : "text-destructive"}>
                      {(d.success_rate * 100).toFixed(1)}%
                    </span>
                  ) : undefined
                }
              />
              <Row label="P95 响应时长" value={d ? formatSeconds(d.p95_duration_ms) : undefined} />
              <Row
                label="Queue 待处理"
                value={d ? `${d.queue.pending} 个 · 死信 ${d.queue.dead}` : undefined}
              />
              <Row
                label="Reply 待处理"
                value={d ? `${d.replies.pending} 个 · 死信 ${d.replies.dead}` : undefined}
              />
            </CardContent>
          </Panel>

          {/* Build info */}
          <Panel>
            <PanelTitle><GitBranch className="size-4 text-muted-foreground" />构建信息</PanelTitle>
            <CardContent className="px-5 pb-5 pt-0">
              <Row label="版本" value={s?.version} />
              <Row label="Commit" value={s?.commit} />
              <Row label="构建时间" value={s?.build_time} />
              <Row label="Go 版本" value={s?.go_version} />
            </CardContent>
          </Panel>
        </div>

        {/* Security status sidebar */}
        <div className="space-y-4">
          <Panel>
            <PanelTitle><ShieldCheck className="size-4 text-muted-foreground" />安全状态</PanelTitle>
            <CardContent className="space-y-3 px-5 pb-5 pt-0">
              {SECURITY_STATUS.map((status) => (
                <div key={status} className="flex items-start gap-2.5 text-sm">
                  <CheckCircle weight="fill" className="mt-0.5 size-4 shrink-0 text-emerald-600" />
                  <span className="leading-snug">{status}</span>
                </div>
              ))}
            </CardContent>
          </Panel>
          <Panel>
            <PanelTitle><HardDrive className="size-4 text-muted-foreground" />存储</PanelTitle>
            <CardContent className="px-5 pb-5 pt-0">
              <Row label="数据库" value="SQLite (modernc)" />
              <Row label="消息队列" value="内置 SQLite" />
              <Row label="会话存储" value="内存（重启清空）" />
            </CardContent>
          </Panel>
        </div>
      </div>
    </PageFrame>
  );
}
