import { FormEvent, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Pulse,
  Bell,
  CaretDown,
  ChartDonut,
  CheckCircle,
  Clock,
  Command,
  Database,
  Envelope,
  Export,
  Gear,
  ListChecks,
  MagnifyingGlass,
  PaperPlaneTilt,
  Queue,
  Question,
  ShieldCheck,
  SidebarSimple,
  SignOut,
  SlidersHorizontal,
  Warning,
} from "@phosphor-icons/react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  Line,
  Pie,
  PieChart,
  Cell,
  ResponsiveContainer,
  Tooltip as ChartTooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { api, APIError, type Dashboard as DashboardData, type Session } from "@/lib/api";

type Page = "dashboard" | "commands" | "executions" | "queue" | "logs" | "security" | "settings";

const navItems = [
  { id: "dashboard", label: "仪表盘", icon: ChartDonut },
  { id: "commands", label: "Command", icon: PaperPlaneTilt },
  { id: "executions", label: "执行记录", icon: ListChecks },
  { id: "queue", label: "Queue 与死信", icon: Envelope },
  { id: "logs", label: "运行日志", icon: Pulse },
  { id: "security", label: "邮箱与安全", icon: ShieldCheck },
  { id: "settings", label: "系统设置", icon: Gear },
] as const;

function Logo() {
  return <div aria-label="MailRelay" className="flex items-center gap-2.5 px-7 py-5"><PaperPlaneTilt weight="fill" className="size-7 text-primary" /><span className="text-[22px] font-semibold tracking-tight">Mail<span className="text-primary">Relay</span></span></div>;
}

function SideNav({ page, setPage, compact = false }: { page: Page; setPage: (page: Page) => void; compact?: boolean }) {
  return <div className={cn("flex h-full flex-col bg-sidebar", compact ? "w-full" : "w-[224px] border-r border-border") }>
    <Logo />
    <div className="mx-4 mb-5 flex h-10 items-center justify-between rounded-lg border border-border bg-card px-3 text-sm">
      <span className="flex items-center gap-2"><Envelope className="size-4" />生产环境</span><CaretDown className="size-3.5" />
    </div>
    <nav className="space-y-1 px-3" aria-label="主导航">
      {navItems.map(({ id, label, icon: Icon }) => <button key={id} type="button" onClick={() => setPage(id)} className={cn("relative flex h-11 w-full items-center gap-3 rounded-lg px-3 text-sm font-medium transition-colors", page === id ? "bg-accent text-accent-foreground" : "text-foreground/80 hover:bg-muted") }>
        {page === id && <span className="absolute -left-3 h-7 w-0.5 rounded-r bg-primary" />}<Icon className="size-[19px]" />{label}
      </button>)}
    </nav>
    <div className="mt-auto border-t border-border p-4"><button className="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground"><SidebarSimple className="size-4" />收起</button></div>
  </div>;
}

function Header({ onMobileNav, session }: { onMobileNav: React.ReactNode; session: Session }) {
  const queryClient = useQueryClient();
  const logout = useMutation({ mutationFn: () => api.logout(session.csrf), onSuccess: () => { queryClient.removeQueries(); } });
  return <header className="flex h-[72px] items-center gap-4 border-b border-border bg-card/90 px-5 backdrop-blur lg:px-8">
    {onMobileNav}
    <button className="hidden h-10 min-w-0 max-w-[480px] flex-1 items-center gap-3 rounded-lg border border-border bg-background px-4 text-left text-sm text-muted-foreground md:flex">
      <MagnifyingGlass className="size-4" /><span className="truncate">搜索命令、ID、收件人、主题或内容...</span><kbd className="ml-auto text-xs">⌘ K</kbd>
    </button>
    <div className="ml-auto flex items-center gap-2 md:gap-4">
      <div className="hidden items-center gap-2 rounded-lg border border-border bg-card px-3 py-2 text-xs font-medium sm:flex"><span className="size-2 rounded-full bg-emerald-600" />系统健康<br />正常</div>
      <IconButton label="通知"><Bell /></IconButton><IconButton label="帮助"><Question /></IconButton>
      <DropdownMenu><DropdownMenuTrigger asChild><button className="flex items-center gap-2 rounded-lg p-1.5 outline-none hover:bg-muted"><Avatar className="size-9"><AvatarFallback className="bg-muted text-xs font-semibold">管</AvatarFallback></Avatar><div className="hidden text-left leading-tight xl:block"><div className="text-sm font-medium">{session.user.name}</div><div className="text-xs text-muted-foreground">平台管理员</div></div><CaretDown className="hidden size-3.5 xl:block" /></button></DropdownMenuTrigger><DropdownMenuContent align="end" className="w-52"><DropdownMenuLabel>已登录为 {session.user.id}</DropdownMenuLabel><DropdownMenuSeparator /><DropdownMenuItem variant="destructive" onSelect={() => logout.mutate()}><SignOut />退出登录</DropdownMenuItem></DropdownMenuContent></DropdownMenu>
    </div>
  </header>;
}

function IconButton({ label, children }: { label: string; children: React.ReactNode }) {
  return <Tooltip><TooltipTrigger asChild><Button aria-label={label} variant="ghost" size="icon">{children}</Button></TooltipTrigger><TooltipContent>{label}</TooltipContent></Tooltip>;
}

function MetricCard({ icon: Icon, label, value, delta, good = false }: { icon: typeof Command; label: string; value: string; delta: string; good?: boolean }) {
  return <Card className="shadow-none"><CardContent className="flex items-center gap-4 p-5"><div className="grid size-12 shrink-0 place-items-center rounded-full border border-border"><Icon className="size-6" /></div><div><div className="text-sm text-muted-foreground">{label}</div><div className="mt-0.5 text-2xl font-semibold tracking-tight">{value}</div><div className={cn("mt-1 text-xs", good ? "text-emerald-700" : "text-primary")}>{delta}</div></div></CardContent></Card>;
}

function PanelTitle({ children, action }: { children: React.ReactNode; action?: React.ReactNode }) {
  return <CardHeader className="flex-row items-center justify-between border-b border-border px-5 py-4"><CardTitle className="text-base font-semibold">{children}</CardTitle>{action}</CardHeader>;
}

function Dashboard() {
  const [range, setRange] = useState("24h");
  const dashboard = useQuery({ queryKey: ["dashboard", range], queryFn: () => api.dashboard(range) });
  const data = dashboard.data;
  const chartData = (data?.recent_executions ?? []).slice().reverse().map((item) => ({ time: new Date(item.started_at).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" }), count: 1, success: item.status === "success" ? 100 : 0 }));
  return <>
    <div className="mb-4 flex flex-wrap items-start justify-between gap-3"><div><h1 className="text-2xl font-semibold tracking-tight">仪表盘</h1><p className="mt-1 text-sm text-muted-foreground">系统概览与运行状态 · {dashboard.isFetching ? "正在更新" : "数据已同步"}</p></div><div className="flex gap-2"><Select value={range} onValueChange={setRange}><SelectTrigger className="w-[142px]"><Clock /><SelectValue /></SelectTrigger><SelectContent><SelectItem value="24h">过去 24 小时</SelectItem><SelectItem value="7d">过去 7 天</SelectItem><SelectItem value="30d">过去 30 天</SelectItem></SelectContent></Select><Button variant="outline" onClick={() => dashboard.refetch()} disabled={dashboard.isFetching}><Pulse />刷新</Button></div></div>
    {dashboard.error && <div role="alert" className="mb-4 rounded-lg border border-destructive/20 bg-red-50 p-3 text-sm text-destructive">无法读取运行数据，请稍后重试。</div>}
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4"><MetricCard icon={Database} label="执行命令数" value={data ? data.execution_count.toLocaleString() : "—"} delta="当前窗口" /><MetricCard icon={PaperPlaneTilt} label="执行成功率" value={data ? `${data.success_rate.toFixed(2)}%` : "—"} delta={data ? `${data.success_count} 次成功` : "等待数据"} good /><MetricCard icon={Clock} label="P95 执行耗时" value={data ? `${(data.p95_duration_ms / 1000).toFixed(2)}s` : "—"} delta="当前窗口" good /><MetricCard icon={ShieldCheck} label="活跃处理器" value={data ? String(data.active_handlers) : "—"} delta="当前窗口" /></div>
    <div className="mt-4 grid gap-4 xl:grid-cols-[1.15fr_1fr]">
      <div className="space-y-4"><Card className="shadow-none"><PanelTitle action={<span className="text-xs text-muted-foreground">最近 {chartData.length} 次执行</span>}>命令活动趋势</PanelTitle><CardContent className="relative h-[280px] p-4">{chartData.length === 0 && <div className="absolute inset-0 z-10 grid place-items-center text-sm text-muted-foreground">当前窗口暂无执行记录</div>}<ResponsiveContainer width="100%" height="100%"><AreaChart data={chartData}><defs><linearGradient id="warmArea" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stopColor="#d84a1b" stopOpacity={0.22}/><stop offset="100%" stopColor="#d84a1b" stopOpacity={0}/></linearGradient></defs><CartesianGrid vertical={false} stroke="#e8e1d9" /><XAxis dataKey="time" tick={{ fontSize: 11, fill: "#817870" }} axisLine={false} tickLine={false} /><YAxis tick={{ fontSize: 11, fill: "#817870" }} axisLine={false} tickLine={false} /><ChartTooltip /><Area type="monotone" dataKey="count" stroke="#d84a1b" strokeWidth={2} fill="url(#warmArea)" /><Line type="monotone" dataKey="success" stroke="#c9ad7b" strokeDasharray="4 4" dot={false} /></AreaChart></ResponsiveContainer></CardContent></Card>
        <EventCallout event={data?.recent_events[0]} />
      </div>
      <div className="space-y-4"><HealthPanel data={data} /><QueuePanel data={data} /><DistributionPanel data={data} /></div>
    </div>
    <ExecutionTable compact items={data?.recent_executions} />
  </>;
}

function EventCallout({ event }: { event?: DashboardData["recent_events"][number] }) {
  if (!event) return <Card className="shadow-none"><CardContent className="flex items-center gap-3 p-4 text-sm text-muted-foreground"><CheckCircle weight="fill" className="size-6 text-emerald-600" />当前没有需要关注的运行事件</CardContent></Card>;
  return <Card className="border-primary/40 bg-[#fffaf7] shadow-none"><CardContent className="flex gap-3 p-4"><div className="grid size-8 shrink-0 place-items-center rounded-full bg-primary text-white"><Warning weight="fill" /></div><div className="min-w-0 flex-1"><div className="flex flex-wrap items-center gap-2 text-sm font-medium">运行事件需要关注 <Badge variant="outline" className="border-primary/20 bg-primary/5 text-primary">{event.severity}</Badge></div><p className="mt-2 text-sm text-muted-foreground">{event.summary}</p><div className="mt-3 text-xs text-muted-foreground">{new Date(event.at).toLocaleString("zh-CN")} · {event.phase}{event.command ? ` · ${event.command}` : ""}</div></div></CardContent></Card>;
}

function HealthPanel({ data }: { data?: DashboardData }) {
  const items = [["处理器", `${data?.active_handlers ?? 0} 活跃`, "", true], ["命令执行", `${data?.execution_count ?? 0} 次`, `${data?.success_count ?? 0} 成功`, true], ["待处理任务", `${(data?.queue.pending ?? 0) + (data?.replies.pending ?? 0)} 个`, "", true], ["死信队列", `${(data?.queue.dead ?? 0) + (data?.replies.dead ?? 0)} 待处理`, "", (data?.queue.dead ?? 0) + (data?.replies.dead ?? 0) === 0]];
  return <Card className="shadow-none"><PanelTitle action={<button className="text-sm font-medium text-primary">查看详情</button>}>运行健康状态</PanelTitle><CardContent className="grid grid-cols-2 p-0 md:grid-cols-4">{items.map(([label, first, second, ok]) => <div key={String(label)} className="border-r border-border p-4 last:border-0"><div className="flex items-center gap-2 text-sm font-medium">{ok ? <CheckCircle weight="fill" className="size-5 text-emerald-600" /> : <Warning weight="fill" className="size-5 text-amber-500" />}{label}</div><div className="mt-3 text-sm">{first}</div><div className={cn("mt-1 text-xs", ok ? "text-muted-foreground" : "text-primary")}>{second}</div></div>)}</CardContent></Card>;
}

function QueuePanel({ data }: { data?: DashboardData }) {
  return <Card className="shadow-none"><PanelTitle>队列与回复状态</PanelTitle><CardContent className="grid grid-cols-2 p-0"><div className="border-r border-border p-4"><div className="mb-3 flex items-center gap-2 text-sm font-medium"><Queue />命令队列</div><div className="grid grid-cols-2 gap-y-2 text-xs"><span className="text-muted-foreground">待处理</span><span>运行中　<strong>{data?.queue.running ?? 0}</strong></span><strong className="text-xl">{data?.queue.pending ?? 0}</strong><span>死信　<strong>{data?.queue.dead ?? 0}</strong></span></div></div><div className="p-4"><div className="mb-3 flex items-center gap-2 text-sm font-medium"><Envelope />回复处理</div><div className="grid grid-cols-2 gap-y-2 text-xs"><span className="text-muted-foreground">待处理</span><span>运行中　<strong>{data?.replies.running ?? 0}</strong></span><strong className="text-xl">{data?.replies.pending ?? 0}</strong><span>死信　<strong>{data?.replies.dead ?? 0}</strong></span></div></div></CardContent></Card>;
}

function DistributionPanel({ data }: { data?: DashboardData }) {
  const values = [data?.queue.pending ?? 0, data?.queue.running ?? 0, data?.queue.dead ?? 0, data?.replies.pending ?? 0, data?.replies.dead ?? 0];
  const total = values.reduce((sum, value) => sum + value, 0);
  const percentages = values.map((value) => total ? Math.round(value / total * 100) : 0);
  const colors = ["#d84a1b", "#c69a56", "#d9c9ad", "#9f9a92", "#e6e1d8"];
  const chart = values.map((value, index) => ({ value: value || (total ? 0 : index === 0 ? 1 : 0) }));
  return <Card className="shadow-none"><PanelTitle>工作负载分布</PanelTitle><CardContent className="flex items-center gap-6 p-5"><div className="relative size-28 shrink-0"><ResponsiveContainer width="100%" height="100%"><PieChart><Pie data={chart} dataKey="value" innerRadius={36} outerRadius={54} stroke="none">{chart.map((_, index) => <Cell key={colors[index]} fill={colors[index]} />)}</Pie></PieChart></ResponsiveContainer><div className="pointer-events-none absolute inset-0 grid place-items-center text-center"><span className="text-xl font-semibold leading-none">{total}<small className="mt-1 block text-[10px] font-normal text-muted-foreground">总计</small></span></div></div><div className="grid flex-1 gap-2 text-xs">{[["队列待处理", values[0]], ["队列运行中", values[1]], ["队列死信", values[2]], ["回复待处理", values[3]], ["回复死信", values[4]]].map(([label, value], index) => <div key={String(label)} className="flex justify-between"><span><i className="mr-2 inline-block size-1.5 rounded-full" style={{ background: colors[index] }} />{label}</span><span>{value}　{percentages[index]}%</span></div>)}</div></CardContent></Card>;
}

function ExecutionTable({ compact = false, items }: { compact?: boolean; items?: DashboardData["recent_executions"] }) {
  const records = items ?? [];
  return <Card className="mt-4 overflow-hidden shadow-none"><PanelTitle action={<div className="hidden gap-2 md:flex"><Button variant="outline"><Export />导出</Button></div>}>{compact ? "最近执行记录" : "执行记录"}</PanelTitle><div className="overflow-x-auto"><Table><TableHeader><TableRow>{["执行 ID", "命令", "发送者", "状态", "处理器", "耗时", "执行时间"].map(x => <TableHead key={x}>{x}</TableHead>)}</TableRow></TableHeader><TableBody>{records.map(item => <TableRow key={item.id}><TableCell className="font-mono text-xs">EXE-{item.id}</TableCell><TableCell>{item.command}</TableCell><TableCell>{item.sender || "—"}</TableCell><TableCell><Badge className={item.status === "success" ? "border-emerald-200 bg-emerald-50 text-emerald-700" : "border-red-200 bg-red-50 text-red-700"} variant="outline">{item.status === "success" ? "成功" : "失败"}</Badge></TableCell><TableCell>{item.handler}</TableCell><TableCell>{(item.duration_ms / 1000).toFixed(2)}s</TableCell><TableCell>{new Date(item.started_at).toLocaleString("zh-CN")}</TableCell></TableRow>)}{records.length === 0 && <TableRow><TableCell colSpan={7} className="h-24 text-center text-muted-foreground">暂无执行记录</TableCell></TableRow>}</TableBody></Table></div>{compact && <button className="w-full border-t border-border py-3 text-sm font-medium text-primary">查看全部执行记录　→</button>}</Card>;
}

function CommandsPage() {
  const result = useQuery({ queryKey: ["commands"], queryFn: api.commands });
  const [search, setSearch] = useState("");
  const items = (result.data?.items ?? []).filter((item) => `${item.name} ${item.description} ${item.handler}`.toLowerCase().includes(search.toLowerCase()));
  return <PageFrame title="Command" description="预先声明、可审计的远程命令目录"><Card className="shadow-none"><PanelTitle action={<Input className="w-[220px]" placeholder="筛选命令..." value={search} onChange={(event) => setSearch(event.target.value)} />}>命令目录</PanelTitle><Table><TableHeader><TableRow>{["命令标识", "说明", "处理器", "成熟度", "参数数", "操作"].map(x => <TableHead key={x}>{x}</TableHead>)}</TableRow></TableHeader><TableBody>{items.map(item => <TableRow key={item.name}><TableCell className="font-mono text-xs">{item.name}</TableCell><TableCell>{item.description || "—"}</TableCell><TableCell>{item.handler}</TableCell><TableCell><Badge variant="outline" className={item.maturity === "Stable" ? "border-emerald-200 bg-emerald-50 text-emerald-700" : "border-amber-200 bg-amber-50 text-amber-700"}>{item.maturity}</Badge></TableCell><TableCell>{item.parameter_count}</TableCell><TableCell><Button size="sm" variant="ghost">查看</Button></TableCell></TableRow>)}{!result.isPending && items.length === 0 && <TableRow><TableCell colSpan={6} className="h-24 text-center text-muted-foreground">没有匹配的 Command</TableCell></TableRow>}</TableBody></Table></Card></PageFrame>;
}

function ExecutionsPage() {
  const result = useQuery({ queryKey: ["executions"], queryFn: () => api.executions({ limit: 50 }) });
  return <PageFrame title="执行记录" description="所有命令的不可变审计轨迹"><ExecutionTable items={result.data?.items} /></PageFrame>;
}

function QueuePage() {
  const jobs = useQuery({ queryKey: ["jobs"], queryFn: () => api.jobs({ limit: 50 }) });
  const replies = useQuery({ queryKey: ["replies"], queryFn: () => api.replies({ limit: 50 }) });
  const all = [...(jobs.data?.items ?? []).map(item => ({ id: `job-${item.id}`, kind: "命令任务", name: item.command, status: item.status, attempts: `${item.attempts}/${item.max_attempts}`, at: item.available_at })), ...(replies.data?.items ?? []).map(item => ({ id: `reply-${item.id}`, kind: "邮件回复", name: item.recipient, status: item.status, attempts: `${item.attempts}/${item.max_attempts}`, at: item.available_at }))];
  const pending = all.filter(item => item.status === "pending").length;
  const running = all.filter(item => item.status === "running").length;
  const dead = all.filter(item => item.status === "dead").length;
  return <PageFrame title="Queue 与死信" description="观察待处理任务、重试与死信状态"><div className="grid gap-4 md:grid-cols-3"><MetricCard icon={Queue} label="待处理" value={String(pending)} delta="当前列表" /><MetricCard icon={Pulse} label="运行中" value={String(running)} delta="当前列表" good /><MetricCard icon={Warning} label="死信" value={String(dead)} delta="当前列表" /></div><Card className="mt-4 shadow-none"><PanelTitle>任务与回复</PanelTitle><Table><TableHeader><TableRow>{["类型", "目标", "状态", "尝试", "可执行时间"].map(x => <TableHead key={x}>{x}</TableHead>)}</TableRow></TableHeader><TableBody>{all.map(item => <TableRow key={item.id}><TableCell>{item.kind}</TableCell><TableCell>{item.name}</TableCell><TableCell><Badge variant="outline">{item.status}</Badge></TableCell><TableCell>{item.attempts}</TableCell><TableCell>{new Date(item.at).toLocaleString("zh-CN")}</TableCell></TableRow>)}{all.length === 0 && <TableRow><TableCell colSpan={5} className="h-24 text-center text-muted-foreground">当前没有队列任务或待发送回复</TableCell></TableRow>}</TableBody></Table></Card></PageFrame>;
}

function LogsPage() {
  const events = useQuery({ queryKey: ["events"], queryFn: () => api.events({ limit: 100 }) });
  return <PageFrame title="运行日志" description="经过脱敏的运行事件与关联上下文" action={<Button variant="outline" onClick={() => events.refetch()}><Pulse />刷新</Button>}><Card className="overflow-hidden bg-[#24211f] text-[#eee8df] shadow-none"><div className="border-b border-white/10 px-4 py-3 font-mono text-xs text-[#a89e94]">EVENT STREAM · safe projection</div><div className="space-y-3 p-5 font-mono text-xs leading-6">{(events.data?.items ?? []).map(item => <div key={item.id} className="grid grid-cols-[72px_56px_70px_1fr] gap-3"><span className="text-[#827970]">{new Date(item.at).toLocaleTimeString("zh-CN", { hour12: false })}</span><span className={item.severity === "error" ? "text-[#ff8a69]" : "text-[#7bc693]"}>{item.severity.toUpperCase()}</span><span className="text-[#d2ad74]">{item.phase}</span><span>{item.summary}{item.command ? ` · ${item.command}` : ""}</span></div>)}{!events.isPending && events.data?.items.length === 0 && <div className="py-8 text-center text-[#a89e94]">暂无运行事件</div>}</div></Card></PageFrame>;
}

function SettingsPage({ security = false }: { security?: boolean }) {
  const system = useQuery({ queryKey: ["system"], queryFn: api.system });
  const uptime = system.data ? `${Math.floor(system.data.uptime_seconds / 3600)} 小时 ${Math.floor(system.data.uptime_seconds % 3600 / 60)} 分钟` : "—";
  return <PageFrame title={security ? "邮箱与安全" : "系统设置"} description={security ? "连接状态、信任边界与敏感配置" : "控制台与运行环境信息（只读）"}><div className="grid gap-4 xl:grid-cols-[1fr_340px]"><Card className="shadow-none"><PanelTitle>{security ? "安全边界" : "运行信息"}</PanelTitle><CardContent className="space-y-5 p-5">{security ? <div className="space-y-4">{["邮箱凭据与授权码不会通过控制台 API 返回", "Command Token 与处理器密钥保持服务端隔离", "邮件正文和原始依赖错误不会进入可见日志", "第一阶段控制台仅提供只读运维能力"].map(x => <div key={x} className="flex items-start gap-3 rounded-lg border border-border p-4 text-sm"><ShieldCheck className="mt-0.5 size-5 shrink-0 text-emerald-600" />{x}</div>)}</div> : <><Field label="启动时间" value={system.data ? new Date(system.data.started_at).toLocaleString("zh-CN") : "—"} /><Field label="运行时长" value={uptime} /><Field label="已声明 Command" value={String(system.data?.command_count ?? "—")} /></>}</CardContent></Card><Card className="h-fit shadow-none"><PanelTitle>安全状态</PanelTitle><CardContent className="space-y-4 p-5">{["签名会话已启用", "CSRF 防护已启用", "敏感字段默认脱敏", "API 响应禁止缓存"].map(x => <div key={x} className="flex items-center gap-2 text-sm"><CheckCircle weight="fill" className="text-emerald-600" />{x}</div>)}</CardContent></Card></div></PageFrame>;
}

function Field({ label, value }: { label: string; value: string }) { return <label className="grid gap-2 text-sm font-medium"><span>{label}</span><Input value={value} readOnly className="bg-muted/40" /></label>; }

function PageFrame({ title, description, action, children }: { title: string; description: string; action?: React.ReactNode; children: React.ReactNode }) {
  return <><div className="mb-5 flex items-start justify-between gap-3"><div><h1 className="text-2xl font-semibold tracking-tight">{title}</h1><p className="mt-1 text-sm text-muted-foreground">{description}</p></div>{action}</div>{children}</>;
}

function ConsoleShell({ session }: { session: Session }) {
  const [page, setPage] = useState<Page>("dashboard");
  const content = useMemo(() => ({ dashboard: <Dashboard />, commands: <CommandsPage />, executions: <ExecutionsPage />, queue: <QueuePage />, logs: <LogsPage />, security: <SettingsPage security />, settings: <SettingsPage /> })[page], [page]);
  return <TooltipProvider><div className="min-h-screen bg-background text-foreground" data-user={session.user.id}><aside className="fixed inset-y-0 left-0 z-30 hidden lg:block"><SideNav page={page} setPage={setPage} /></aside><div className="lg:pl-[224px]"><Header session={session} onMobileNav={<Sheet><SheetTrigger asChild><Button className="lg:hidden" variant="outline" size="icon"><SidebarSimple /></Button></SheetTrigger><SheetContent side="left" className="w-[270px] p-0"><SideNav page={page} setPage={setPage} compact /></SheetContent></Sheet>} /><main className="mx-auto max-w-[1500px] p-4 lg:p-6">{content}</main></div></div></TooltipProvider>;
}

function LoginPage() {
  const queryClient = useQueryClient();
  const [password, setPassword] = useState("");
  const login = useMutation({
    mutationFn: () => api.login(password),
    onSuccess: (session) => queryClient.setQueryData(["session"], session),
  });
  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (password) login.mutate();
  };
  const message = login.error instanceof APIError ? login.error.message : login.error ? "暂时无法登录，请稍后重试" : "";
  return <main className="grid min-h-screen grid-cols-1 bg-background lg:grid-cols-[minmax(420px,0.8fr)_1.2fr]">
    <section className="flex min-h-screen flex-col border-r border-border bg-card px-6 py-5 sm:px-12 lg:px-16">
      <div className="-ml-7"><Logo /></div>
      <div className="my-auto w-full max-w-[420px] py-16">
        <div className="mb-8 grid size-12 place-items-center rounded-xl bg-primary text-white shadow-[0_12px_28px_rgba(216,74,27,0.2)]"><ShieldCheck className="size-6" /></div>
        <h1 className="text-3xl font-semibold tracking-tight">登录运行控制台</h1>
        <p className="mt-3 text-sm leading-6 text-muted-foreground">查看命令、执行记录、队列与系统健康状态。控制台默认仅监听本机地址。</p>
        <form className="mt-8 space-y-5" onSubmit={submit}>
          <div className="space-y-2"><Label htmlFor="admin-password">管理员密码</Label><Input id="admin-password" type="password" autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} placeholder="输入控制台管理员密码" aria-invalid={Boolean(message)} autoFocus /></div>
          {message && <p role="alert" className="text-sm text-destructive">{message}</p>}
          <Button className="h-10 w-full" type="submit" disabled={!password || login.isPending}>{login.isPending ? "正在验证..." : "登录"}</Button>
        </form>
        <p className="mt-6 flex items-center gap-2 text-xs text-muted-foreground"><ShieldCheck className="size-4 text-emerald-600" />会话使用 HttpOnly Cookie 与 CSRF 防护</p>
      </div>
      <p className="text-xs text-muted-foreground">MailRelay · 少量、预先声明且可审计的远程命令</p>
    </section>
    <section className="relative hidden overflow-hidden bg-[#f1ece4] lg:block"><div className="absolute inset-0 grid place-items-center p-16"><div className="w-full max-w-[640px] rounded-2xl border border-white/80 bg-card/85 p-7 shadow-[0_30px_80px_rgba(70,49,32,0.12)] backdrop-blur"><div className="flex items-center justify-between border-b border-border pb-5"><div><div className="text-sm font-semibold">运行状态</div><div className="mt-1 text-xs text-muted-foreground">安全边界内的实时可见性</div></div><Badge variant="outline" className="border-emerald-200 bg-emerald-50 text-emerald-700"><span className="mr-1 size-1.5 rounded-full bg-emerald-600" />系统健康</Badge></div><div className="grid grid-cols-3 gap-3 py-6">{[["命令", "预先声明"], ["执行", "全程审计"], ["敏感信息", "默认脱敏"]].map(([name, value]) => <div key={name} className="rounded-xl border border-border bg-card p-4"><div className="text-xs text-muted-foreground">{name}</div><div className="mt-2 text-sm font-semibold">{value}</div></div>)}</div><div className="rounded-xl bg-[#28211f] p-5 font-mono text-xs leading-6 text-[#eee6dd]"><span className="text-[#d48d65]">11:58:32</span>　command accepted<br/><span className="text-[#7bc693]">11:58:33</span>　policy verified<br/><span className="text-[#7bc693]">11:58:34</span>　audit record persisted</div></div></div></section>
  </main>;
}

export function App() {
  const session = useQuery({ queryKey: ["session"], queryFn: api.session, retry: false });
  if (session.isPending) return <div className="grid min-h-screen place-items-center"><div className="text-center"><Logo /><p className="text-sm text-muted-foreground">正在连接运行控制台...</p></div></div>;
  if (session.error) return <LoginPage />;
  return <ConsoleShell session={session.data} />;
}
