import { useMemo, useState } from "react";
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
  SlidersHorizontal,
  Warning,
} from "@phosphor-icons/react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  Line,
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

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

const trend = [
  ["06-01 12:00", 460, 99.1], ["14:00", 590, 99.4], ["16:00", 760, 99.2],
  ["18:00", 720, 99.4], ["20:00", 1060, 99.3], ["22:00", 840, 99.2],
  ["06-02 00:00", 930, 99.5], ["02:00", 810, 99.5], ["04:00", 570, 99.1],
  ["06:00", 480, 99.2], ["08:00", 390, 99.4], ["10:00", 610, 99.3],
  ["12:00", 1040, 99.6], ["14:00", 820, 99.5],
].map(([time, count, success]) => ({ time, count, success }));

const executions = [
  ["EXE-20240602-00128", "重置密码", "user1@example.com", "成功", "node-03", "1.42s", "2024-06-02 11:58:32"],
  ["EXE-20240602-00127", "发送报告", "ops-team@example.com", "成功", "node-01", "2.11s", "2024-06-02 11:57:41"],
  ["EXE-20240602-00126", "通知发送", "user2@example.com", "失败", "node-07", "—", "2024-06-02 11:56:28"],
  ["EXE-20240602-00125", "同步数据", "system@example.com", "成功", "node-02", "3.05s", "2024-06-02 11:55:12"],
  ["EXE-20240602-00124", "清理缓存", "admin@example.com", "成功", "node-05", "0.98s", "2024-06-02 11:54:03"],
];

const commands = [
  ["reset-password", "重置密码", "HTTP", "已启用", "24 分钟前"],
  ["send-report", "发送报告", "Workflow", "已启用", "1 小时前"],
  ["notify", "通知发送", "HTTP", "已启用", "2 小时前"],
  ["sync-data", "同步数据", "Queue", "实验性", "昨天"],
];

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

function Header({ onMobileNav }: { onMobileNav: React.ReactNode }) {
  return <header className="flex h-[72px] items-center gap-4 border-b border-border bg-card/90 px-5 backdrop-blur lg:px-8">
    {onMobileNav}
    <button className="hidden h-10 min-w-0 max-w-[480px] flex-1 items-center gap-3 rounded-lg border border-border bg-background px-4 text-left text-sm text-muted-foreground md:flex">
      <MagnifyingGlass className="size-4" /><span className="truncate">搜索命令、ID、收件人、主题或内容...</span><kbd className="ml-auto text-xs">⌘ K</kbd>
    </button>
    <div className="ml-auto flex items-center gap-2 md:gap-4">
      <div className="hidden items-center gap-2 rounded-lg border border-border bg-card px-3 py-2 text-xs font-medium sm:flex"><span className="size-2 rounded-full bg-emerald-600" />系统健康<br />正常</div>
      <IconButton label="通知"><Bell /></IconButton><IconButton label="帮助"><Question /></IconButton>
      <Avatar className="size-9"><AvatarFallback className="bg-muted text-xs font-semibold">ZL</AvatarFallback></Avatar>
      <div className="hidden leading-tight xl:block"><div className="text-sm font-medium">张磊</div><div className="text-xs text-muted-foreground">平台管理员</div></div><CaretDown className="hidden size-3.5 xl:block" />
    </div>
  </header>;
}

function IconButton({ label, children }: { label: string; children: React.ReactNode }) {
  return <Tooltip><TooltipTrigger asChild><Button aria-label={label} variant="ghost" size="icon">{children}</Button></TooltipTrigger><TooltipContent>{label}</TooltipContent></Tooltip>;
}

function MetricCard({ icon: Icon, label, value, delta, good = false }: { icon: typeof Command; label: string; value: string; delta: string; good?: boolean }) {
  return <Card className="shadow-none"><CardContent className="flex items-center gap-4 p-5"><div className="grid size-12 shrink-0 place-items-center rounded-full border border-border"><Icon className="size-6" /></div><div><div className="text-sm text-muted-foreground">{label}</div><div className="mt-0.5 text-2xl font-semibold tracking-tight">{value}</div><div className="mt-1 text-xs text-muted-foreground">较昨日 <span className={good ? "text-emerald-700" : "text-primary"}>{delta}</span></div></div></CardContent></Card>;
}

function PanelTitle({ children, action }: { children: React.ReactNode; action?: React.ReactNode }) {
  return <CardHeader className="flex-row items-center justify-between border-b border-border px-5 py-4"><CardTitle className="text-base font-semibold">{children}</CardTitle>{action}</CardHeader>;
}

function Dashboard() {
  const [range, setRange] = useState("24h");
  const [refreshed, setRefreshed] = useState("刚刚");
  return <>
    <div className="mb-4 flex flex-wrap items-start justify-between gap-3"><div><h1 className="text-2xl font-semibold tracking-tight">仪表盘</h1><p className="mt-1 text-sm text-muted-foreground">系统概览与运行状态 · 更新于{refreshed}</p></div><div className="flex gap-2"><Select value={range} onValueChange={setRange}><SelectTrigger className="w-[142px]"><Clock /><SelectValue /></SelectTrigger><SelectContent><SelectItem value="24h">过去 24 小时</SelectItem><SelectItem value="7d">过去 7 天</SelectItem><SelectItem value="30d">过去 30 天</SelectItem></SelectContent></Select><Button variant="outline" onClick={() => setRefreshed(new Date().toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" }))}><Pulse />刷新</Button></div></div>
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4"><MetricCard icon={Database} label="执行命令数" value="1,284" delta="↑ 18.6%" /><MetricCard icon={PaperPlaneTilt} label="投递成功率" value="99.62%" delta="↑ 0.18%" /><MetricCard icon={Clock} label="P95 执行耗时" value="2.38s" delta="↓ 0.42s" good /><MetricCard icon={ShieldCheck} label="活跃处理器" value="23/30" delta="↑ 2" /></div>
    <div className="mt-4 grid gap-4 xl:grid-cols-[1.15fr_1fr]">
      <div className="space-y-4"><Card className="shadow-none"><PanelTitle action={<div className="rounded-md bg-muted p-1 text-xs"><span className="rounded bg-card px-3 py-1.5 text-primary shadow-sm">时间</span><span className="px-3">命令类型</span></div>}>命令活动趋势</PanelTitle><CardContent className="h-[280px] p-4"><ResponsiveContainer width="100%" height="100%"><AreaChart data={trend}><defs><linearGradient id="warmArea" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stopColor="#d84a1b" stopOpacity={0.22}/><stop offset="100%" stopColor="#d84a1b" stopOpacity={0}/></linearGradient></defs><CartesianGrid vertical={false} stroke="#e8e1d9" /><XAxis dataKey="time" tick={{ fontSize: 11, fill: "#817870" }} interval={2} axisLine={false} tickLine={false} /><YAxis tick={{ fontSize: 11, fill: "#817870" }} axisLine={false} tickLine={false} /><ChartTooltip /><Area type="monotone" dataKey="count" stroke="#d84a1b" strokeWidth={2} fill="url(#warmArea)" /><Line type="monotone" dataKey="success" stroke="#c9ad7b" strokeDasharray="4 4" dot={false} /></AreaChart></ResponsiveContainer></CardContent></Card>
        <Card className="border-primary/40 bg-[#fffaf7] shadow-none"><CardContent className="flex gap-3 p-4"><div className="grid size-8 shrink-0 place-items-center rounded-full bg-primary text-white"><Warning weight="fill" /></div><div className="min-w-0 flex-1"><div className="flex flex-wrap items-center gap-2 text-sm font-medium">1 起事件需要关注 <Badge variant="outline" className="border-primary/20 bg-primary/5 text-primary">处理中</Badge></div><p className="mt-2 text-sm text-muted-foreground">处理器 node-07 在 10:15 出现连续超时，已自动切换流量。</p><div className="mt-3 flex flex-wrap justify-between gap-2 text-xs text-muted-foreground"><span>开始时间：06-02 10:15　　影响范围：命令执行延迟</span><button className="font-medium text-primary">查看事件</button></div></div></CardContent></Card>
      </div>
      <div className="space-y-4"><HealthPanel /><QueuePanel /><DistributionPanel /></div>
    </div>
    <ExecutionTable compact />
  </>;
}

function HealthPanel() {
  const items = [["处理器", "23 正常", "2 异常", true], ["邮箱连接", "28 正常", "0 异常", true], ["存储", "使用率 42%", "", true], ["死信队列", "12 待处理", "较昨日 ↑ 5", false]];
  return <Card className="shadow-none"><PanelTitle action={<button className="text-sm font-medium text-primary">查看详情</button>}>运行健康状态</PanelTitle><CardContent className="grid grid-cols-2 p-0 md:grid-cols-4">{items.map(([label, first, second, ok]) => <div key={String(label)} className="border-r border-border p-4 last:border-0"><div className="flex items-center gap-2 text-sm font-medium">{ok ? <CheckCircle weight="fill" className="size-5 text-emerald-600" /> : <Warning weight="fill" className="size-5 text-amber-500" />}{label}</div><div className="mt-3 text-sm">{first}</div><div className={cn("mt-1 text-xs", ok ? "text-muted-foreground" : "text-primary")}>{second}</div></div>)}</CardContent></Card>;
}

function QueuePanel() {
  return <Card className="shadow-none"><PanelTitle action={<button className="text-sm font-medium text-primary">查看详情</button>}>队列与回复状态</PanelTitle><CardContent className="grid grid-cols-2 p-0"><div className="border-r border-border p-4"><div className="mb-3 flex items-center gap-2 text-sm font-medium"><Queue />命令队列</div><div className="grid grid-cols-2 gap-y-2 text-xs"><span className="text-muted-foreground">待处理</span><span>运行中　<strong>89</strong></span><strong className="text-xl">256</strong><span>已完成　<strong>1,186</strong></span></div></div><div className="p-4"><div className="mb-3 flex items-center gap-2 text-sm font-medium"><Envelope />回复处理</div><div className="grid grid-cols-2 gap-y-2 text-xs"><span className="text-muted-foreground">待处理</span><span>成功　<strong>1,032</strong></span><strong className="text-xl">18</strong><span>失败　<strong>34</strong></span></div></div></CardContent></Card>;
}

function DistributionPanel() {
  return <Card className="shadow-none"><PanelTitle>处理器分布</PanelTitle><CardContent className="flex items-center gap-6 p-5"><div className="relative grid size-28 shrink-0 place-items-center rounded-full" style={{ background: "conic-gradient(#d84a1b 0 30%, #c69a56 30% 50%, #d9c9ad 50% 67%, #9f9a92 67% 80%, #e6e1d8 80%)" }}><div className="grid size-[72px] place-items-center rounded-full bg-card text-center"><span className="text-xl font-semibold leading-none">30<small className="mt-1 block text-[10px] font-normal text-muted-foreground">总计</small></span></div></div><div className="grid flex-1 gap-2 text-xs">{[["华东 1（杭州）", "9　30%"], ["华北 1（北京）", "6　20%"], ["华南 1（广州）", "5　17%"], ["华东 2（上海）", "4　13%"], ["其他", "6　20%"]].map(([label, value], index) => <div key={label} className="flex justify-between"><span><i className={cn("mr-2 inline-block size-1.5 rounded-full", ["bg-primary", "bg-[#c69a56]", "bg-[#d9c9ad]", "bg-[#9f9a92]", "bg-[#e6e1d8]"][index])} />{label}</span><span>{value}</span></div>)}</div></CardContent></Card>;
}

function ExecutionTable({ compact = false }: { compact?: boolean }) {
  return <Card className="mt-4 overflow-hidden shadow-none"><PanelTitle action={<div className="hidden gap-2 md:flex"><Select defaultValue="all"><SelectTrigger className="w-[150px]"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">全部命令类型</SelectItem><SelectItem value="http">HTTP</SelectItem></SelectContent></Select><Select defaultValue="status"><SelectTrigger className="w-[120px]"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="status">全部状态</SelectItem><SelectItem value="success">成功</SelectItem><SelectItem value="failed">失败</SelectItem></SelectContent></Select><Button variant="outline"><Export />导出</Button></div>}>{compact ? "最近执行记录" : "执行记录"}</PanelTitle><div className="overflow-x-auto"><Table><TableHeader><TableRow>{["执行 ID", "命令类型", "收件人", "状态", "处理器", "耗时", "执行时间"].map(x => <TableHead key={x}>{x}</TableHead>)}</TableRow></TableHeader><TableBody>{executions.map(row => <TableRow key={row[0]}>{row.map((cell, i) => <TableCell key={i} className={i === 0 ? "font-mono text-xs" : ""}>{i === 3 ? <Badge className={cell === "成功" ? "border-emerald-200 bg-emerald-50 text-emerald-700" : "border-red-200 bg-red-50 text-red-700"} variant="outline">{cell}</Badge> : cell}</TableCell>)}</TableRow>)}</TableBody></Table></div>{compact && <button className="w-full border-t border-border py-3 text-sm font-medium text-primary">查看全部执行记录　→</button>}</Card>;
}

function CommandsPage() {
  return <PageFrame title="Command" description="预先声明、可审计的远程命令目录" action={<Button><PaperPlaneTilt />声明 Command</Button>}><Card className="shadow-none"><PanelTitle action={<Input className="w-[220px]" placeholder="筛选命令..." />}>命令目录</PanelTitle><Table><TableHeader><TableRow>{["命令标识", "名称", "处理器", "状态", "最近执行", "操作"].map(x => <TableHead key={x}>{x}</TableHead>)}</TableRow></TableHeader><TableBody>{commands.map(row => <TableRow key={row[0]}>{row.map((cell, i) => <TableCell key={i}>{i === 3 ? <Badge variant="outline" className={cell === "已启用" ? "border-emerald-200 bg-emerald-50 text-emerald-700" : "border-amber-200 bg-amber-50 text-amber-700"}>{cell}</Badge> : cell}</TableCell>)}<TableCell><Button size="sm" variant="ghost">查看</Button></TableCell></TableRow>)}</TableBody></Table></Card></PageFrame>;
}

function QueuePage() {
  return <PageFrame title="Queue 与死信" description="观察待处理任务、重试与死信状态"><div className="grid gap-4 md:grid-cols-3"><MetricCard icon={Queue} label="待处理" value="256" delta="↑ 12" /><MetricCard icon={Pulse} label="运行中" value="89" delta="↑ 4" /><MetricCard icon={Warning} label="死信" value="12" delta="↑ 5" /></div><Card className="mt-4 shadow-none"><PanelTitle action={<Button variant="outline">检查死信</Button>}>队列分区</PanelTitle><CardContent className="grid gap-3 p-5">{["command.default", "reply.outbox", "workflow.async"].map((name, i) => <div key={name} className="flex items-center gap-4 rounded-lg border border-border p-4"><Queue className="size-5" /><div className="flex-1"><div className="font-mono text-sm font-medium">{name}</div><div className="mt-1 text-xs text-muted-foreground">{["4 个消费者 · 延迟 1.2s", "2 个消费者 · 延迟 0.4s", "3 个消费者 · 延迟 3.8s"][i]}</div></div><Badge variant="outline" className="border-emerald-200 bg-emerald-50 text-emerald-700">正常</Badge></div>)}</CardContent></Card></PageFrame>;
}

function LogsPage() {
  return <PageFrame title="运行日志" description="按级别、组件与关联 ID 检索系统事件" action={<Button variant="outline"><SlidersHorizontal />筛选</Button>}><Card className="overflow-hidden bg-[#24211f] text-[#eee8df] shadow-none"><div className="border-b border-white/10 px-4 py-3 font-mono text-xs text-[#a89e94]">LIVE · mailrelay-production</div><div className="space-y-3 p-5 font-mono text-xs leading-6">{[["11:58:32", "INFO", "router", "command reset-password dispatched execution=EXE-00128"], ["11:58:31", "INFO", "imap", "message accepted sender=user1@example.com"], ["11:56:29", "ERROR", "handler", "notification failed: upstream timeout execution=EXE-00126"], ["11:55:12", "INFO", "workflow", "sync-data completed duration=3.05s"]].map(row => <div key={row.join()} className="grid grid-cols-[72px_56px_70px_1fr] gap-3"><span className="text-[#827970]">{row[0]}</span><span className={row[1] === "ERROR" ? "text-[#ff8a69]" : "text-[#7bc693]"}>{row[1]}</span><span className="text-[#d2ad74]">{row[2]}</span><span>{row[3]}</span></div>)}</div></Card></PageFrame>;
}

function SettingsPage({ security = false }: { security?: boolean }) {
  return <PageFrame title={security ? "邮箱与安全" : "系统设置"} description={security ? "连接状态、信任边界与敏感配置" : "控制台与运行环境配置"}><div className="grid gap-4 xl:grid-cols-[1fr_340px]"><Card className="shadow-none"><PanelTitle>{security ? "邮件服务连接" : "基础配置"}</PanelTitle><CardContent className="space-y-5 p-5">{security ? <><Field label="IMAP 服务器" value="imap.qq.com:993" /><Field label="SMTP 服务器" value="smtp.qq.com:465" /><Field label="邮箱账户" value="li***@qq.com" /><Field label="授权码" value="••••••••••••••••" /></> : <><Field label="实例名称" value="MailRelay Production" /><Field label="监听地址" value="127.0.0.1:8080" /><Field label="数据目录" value="./data/mailrelay.db" /><Field label="日志级别" value="info" /></>}<div className="flex justify-end"><Button>保存配置</Button></div></CardContent></Card><Card className="h-fit shadow-none"><PanelTitle>安全状态</PanelTitle><CardContent className="space-y-4 p-5">{["仅本机监听", "会话 Cookie 已加固", "敏感字段已脱敏", "审计日志已启用"].map(x => <div key={x} className="flex items-center gap-2 text-sm"><CheckCircle weight="fill" className="text-emerald-600" />{x}</div>)}</CardContent></Card></div></PageFrame>;
}

function Field({ label, value }: { label: string; value: string }) { return <label className="grid gap-2 text-sm font-medium"><span>{label}</span><Input defaultValue={value} /></label>; }

function PageFrame({ title, description, action, children }: { title: string; description: string; action?: React.ReactNode; children: React.ReactNode }) {
  return <><div className="mb-5 flex items-start justify-between gap-3"><div><h1 className="text-2xl font-semibold tracking-tight">{title}</h1><p className="mt-1 text-sm text-muted-foreground">{description}</p></div>{action}</div>{children}</>;
}

export function App() {
  const [page, setPage] = useState<Page>("dashboard");
  const content = useMemo(() => ({ dashboard: <Dashboard />, commands: <CommandsPage />, executions: <PageFrame title="执行记录" description="所有命令的不可变审计轨迹"><ExecutionTable /></PageFrame>, queue: <QueuePage />, logs: <LogsPage />, security: <SettingsPage security />, settings: <SettingsPage /> })[page], [page]);
  return <TooltipProvider><div className="min-h-screen bg-background text-foreground"><aside className="fixed inset-y-0 left-0 z-30 hidden lg:block"><SideNav page={page} setPage={setPage} /></aside><div className="lg:pl-[224px]"><Header onMobileNav={<Sheet><SheetTrigger asChild><Button className="lg:hidden" variant="outline" size="icon"><SidebarSimple /></Button></SheetTrigger><SheetContent side="left" className="w-[270px] p-0"><SideNav page={page} setPage={setPage} compact /></SheetContent></Sheet>} /><main className="mx-auto max-w-[1500px] p-4 lg:p-6">{content}</main></div></div></TooltipProvider>;
}
