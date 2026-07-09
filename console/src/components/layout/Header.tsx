import { useState, type ReactNode } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Bell, CaretDown, Envelope, MagnifyingGlass, Question, SignOut } from "@phosphor-icons/react";
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
import type { PageId } from "./nav";

function IconButton({ label, children }: { label: string; children: ReactNode }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button aria-label={label} variant="ghost" size="icon">
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
  useHotkey("k", () => setSearchOpen(true));
  const commandsResult = useCommands(searchOpen);
  const matches = (commandsResult.data?.items ?? [])
    .filter((item) => `${item.name} ${item.description} ${item.handler}`.toLowerCase().includes(search.toLowerCase()))
    .slice(0, 8);

  return (
    <header className="flex h-[72px] items-center gap-4 border-b border-border bg-card/90 px-5 backdrop-blur lg:px-8">
      {onMobileNav}
      <div className="hidden h-10 w-[164px] shrink-0 items-center rounded-lg border border-border bg-background px-3 text-sm lg:flex">
        <Envelope className="mr-2 size-4" />
        生产环境
      </div>
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
        <div className="hidden items-center gap-2 rounded-lg border border-border bg-card px-3 py-2 text-xs font-medium sm:flex">
          <span className="size-2 rounded-full bg-emerald-600" />
          系统健康
          <br />
          正常
        </div>
        <IconButton label="通知">
          <Bell />
        </IconButton>
        <IconButton label="帮助">
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
    </header>
  );
}
