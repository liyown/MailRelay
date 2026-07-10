import { useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Bell, CaretDown, ClipboardText, ListChecks, MagnifyingGlass, Question, SignOut, TerminalWindow } from "@phosphor-icons/react";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { api, type Session } from "@/lib/api";
import { useCommands } from "@/hooks/queries";
import { useHotkey } from "@/hooks/useHotkey";
import { formatClock } from "@/lib/format";
import type { PageId } from "./nav";

function IconButton({ label, children, onClick }: { label: string; children: ReactNode; onClick?: () => void }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button aria-label={label} variant="ghost" size="icon" onClick={onClick}>
          {children}
        </Button>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}

export function Header({
  onMobileNav,
  session,
  setPage,
}: {
  onMobileNav: ReactNode;
  session: Session;
  setPage: (page: PageId) => void;
}) {
  const queryClient = useQueryClient();
  const logout = useMutation({
    mutationFn: () => api.logout(session.csrf),
    onSuccess: () => queryClient.removeQueries(),
  });
  const [searchOpen, setSearchOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [notificationsOpen, setNotificationsOpen] = useState(false);
  const [helpOpen, setHelpOpen] = useState(false);
  useHotkey("k", () => setSearchOpen(true));
  const commandsResult = useCommands(searchOpen);
  const notices = useQuery({
    queryKey: ["header-events"],
    queryFn: () => api.events({ limit: 8 }),
    enabled: notificationsOpen,
    refetchInterval: notificationsOpen ? 10_000 : false,
  });
  const noticeItems = notices.data?.items ?? [];
  const matches = (commandsResult.data?.items ?? [])
    .filter((item) => `${item.name} ${item.description} ${item.handler}`.toLowerCase().includes(search.toLowerCase()))
    .slice(0, 8);

  return (
    <header className="z-20 flex h-[72px] shrink-0 items-center gap-4 border-b border-border bg-card/90 px-5 backdrop-blur lg:px-8">
      {onMobileNav}
      <Dialog open={searchOpen} onOpenChange={setSearchOpen}>
        <button
          type="button"
          onClick={() => setSearchOpen(true)}
          className="hidden h-10 min-w-0 max-w-[480px] flex-1 items-center gap-3 rounded-lg border border-border bg-background px-4 text-left text-sm text-muted-foreground md:flex"
        >
          <MagnifyingGlass className="size-4" />
          <span className="truncate">搜索 Command...</span>
          <kbd className="ml-auto text-xs">⌘ K</kbd>
        </button>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>搜索 Command</DialogTitle>
            <DialogDescription>在已声明的命令目录中搜索名称、描述或处理器。</DialogDescription>
          </DialogHeader>
          <Input autoFocus value={search} onChange={(event) => setSearch(event.target.value)} placeholder="输入名称或处理器" />
          <div className="max-h-72 space-y-2 overflow-y-auto">
            {matches.map((item) => (
              <button
                key={item.name}
                onClick={() => {
                  setSearchOpen(false);
                  setPage("commands");
                }}
                className="flex w-full items-center justify-between rounded-lg border border-border p-3 text-left hover:bg-muted"
              >
                <span>
                  <strong className="block font-mono text-sm">{item.name}</strong>
                  <small className="text-muted-foreground">{item.description || item.handler}</small>
                </span>
                <Badge variant="outline">{item.maturity}</Badge>
              </button>
            ))}
            {!commandsResult.isPending && matches.length === 0 && (
              <p className="py-8 text-center text-sm text-muted-foreground">没有匹配的 Command</p>
            )}
          </div>
        </DialogContent>
      </Dialog>
      <div className="ml-auto flex items-center gap-2 md:gap-4">
        <IconButton label="通知" onClick={() => setNotificationsOpen(true)}>
          <Bell />
        </IconButton>
        <IconButton label="帮助" onClick={() => setHelpOpen(true)}>
          <Question />
        </IconButton>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button className="flex items-center gap-2 rounded-lg p-1.5 outline-none hover:bg-muted">
              <Avatar className="size-9">
                <AvatarFallback className="bg-muted text-xs font-semibold">管</AvatarFallback>
              </Avatar>
              <div className="hidden text-left leading-tight xl:block">
                <div className="text-sm font-medium">{session.user.name}</div>
                <div className="text-xs text-muted-foreground">平台管理员</div>
              </div>
              <CaretDown className="hidden size-3.5 xl:block" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-52">
            <DropdownMenuLabel>已登录为 {session.user.id}</DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onSelect={() => logout.mutate()}>
              <SignOut />
              退出登录
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <Dialog open={notificationsOpen} onOpenChange={setNotificationsOpen}>
        <DialogContent className="max-h-[80vh] sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>运行通知</DialogTitle>
            <DialogDescription>近期运行事件与失败分类。</DialogDescription>
          </DialogHeader>
          <div className="max-h-[52vh] overflow-y-auto rounded-lg border border-border">
            {notices.isPending && <p className="p-4 text-sm text-muted-foreground">正在加载...</p>}
            {notices.isError && <p className="p-4 text-sm text-destructive">无法读取运行通知。</p>}
            {!notices.isPending && !notices.isError && noticeItems.length === 0 && (
              <p className="p-4 text-sm text-muted-foreground">暂无运行事件。</p>
            )}
            {noticeItems.map((item) => (
              <div key={item.id} className="border-b border-border px-4 py-3 last:border-0">
                <div className="flex items-start gap-2">
                  <Badge variant={item.severity === "error" ? "destructive" : "secondary"} className="shrink-0 text-[10px]">
                    {item.severity === "error" ? "错误" : "事件"}
                  </Badge>
                  <p className="min-w-0 flex-1 text-sm">{item.summary}</p>
                  <time className="shrink-0 text-xs text-muted-foreground">{formatClock(item.at)}</time>
                </div>
                {(item.command || item.error_kind) && (
                  <p className="mt-1 pl-11 font-mono text-[11px] text-muted-foreground">
                    {[item.command, item.error_kind].filter(Boolean).join(" · ")}
                  </p>
                )}
              </div>
            ))}
          </div>
          <div className="flex justify-end">
            <Button variant="outline" size="sm" onClick={() => { setNotificationsOpen(false); setPage("logs"); }}>
              查看全部日志
            </Button>
          </div>
        </DialogContent>
      </Dialog>
      <Dialog open={helpOpen} onOpenChange={setHelpOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>邮件命令帮助</DialogTitle>
            <DialogDescription>邮件主题的第一个词是命令名，正文提供 Token 和已声明参数。</DialogDescription>
          </DialogHeader>
          <pre className="overflow-x-auto rounded-lg border border-border bg-muted/50 p-3 font-mono text-xs leading-6 whitespace-pre">{`From: allowed@example.com\nSubject: command-name\n\n_token=your-token\nkey=value`}</pre>
          <div className="grid gap-2 sm:grid-cols-3">
            <Button variant="outline" className="justify-start" onClick={() => { setHelpOpen(false); setPage("commands"); }}>
              <ListChecks /> 命令目录
            </Button>
            <Button variant="outline" className="justify-start" onClick={() => { setHelpOpen(false); setPage("executions"); }}>
              <ClipboardText /> 执行记录
            </Button>
            <Button variant="outline" className="justify-start" onClick={() => { setHelpOpen(false); setPage("logs"); }}>
              <TerminalWindow /> 运行日志
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </header>
  );
}
