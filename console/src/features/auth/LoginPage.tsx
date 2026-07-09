import { useState, type FormEvent } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ShieldCheck } from "@phosphor-icons/react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Logo } from "@/components/layout/Logo";
import { api, APIError } from "@/lib/api";

export function LoginPage() {
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
  return (
    <main className="grid min-h-screen grid-cols-1 bg-background lg:grid-cols-[minmax(420px,0.8fr)_1.2fr]">
      <section className="flex min-h-screen flex-col border-r border-border bg-card px-6 py-5 sm:px-12 lg:px-16">
        <div className="-ml-7">
          <Logo />
        </div>
        <div className="my-auto w-full max-w-[420px] py-16">
          <div className="mb-8 grid size-12 place-items-center rounded-xl bg-primary text-white shadow-[0_12px_28px_rgba(216,74,27,0.2)]">
            <ShieldCheck className="size-6" />
          </div>
          <h1 className="text-3xl font-semibold tracking-tight">登录运行控制台</h1>
          <p className="mt-3 text-sm leading-6 text-muted-foreground">
            查看命令、执行记录、队列与系统健康状态，并重放死信项。控制台默认仅监听本机地址。
          </p>
          <form className="mt-8 space-y-5" onSubmit={submit}>
            <div className="space-y-2">
              <Label htmlFor="admin-password">管理员密码</Label>
              <Input
                id="admin-password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="输入控制台管理员密码"
                aria-invalid={Boolean(message)}
                autoFocus
              />
            </div>
            {message && (
              <p role="alert" className="text-sm text-destructive">
                {message}
              </p>
            )}
            <Button className="h-10 w-full" type="submit" disabled={!password || login.isPending}>
              {login.isPending ? "正在验证..." : "登录"}
            </Button>
          </form>
          <p className="mt-6 flex items-center gap-2 text-xs text-muted-foreground">
            <ShieldCheck className="size-4 text-emerald-600" />
            会话使用 HttpOnly Cookie 与 CSRF 防护
          </p>
        </div>
        <p className="text-xs text-muted-foreground">MailRelay · 少量、预先声明且可审计的远程命令</p>
      </section>
      <section className="relative hidden overflow-hidden bg-[#f1ece4] lg:block">
        <div className="absolute inset-0 grid place-items-center p-16">
          <div className="w-full max-w-[640px] rounded-2xl border border-white/80 bg-card/85 p-7 shadow-[0_30px_80px_rgba(70,49,32,0.12)] backdrop-blur">
            <div className="flex items-center justify-between border-b border-border pb-5">
              <div>
                <div className="text-sm font-semibold">运行状态</div>
                <div className="mt-1 text-xs text-muted-foreground">安全边界内的实时可见性</div>
              </div>
              <Badge variant="outline" className="border-emerald-200 bg-emerald-50 text-emerald-700">
                <span className="mr-1 size-1.5 rounded-full bg-emerald-600" />
                系统健康
              </Badge>
            </div>
            <div className="grid grid-cols-3 gap-3 py-6">
              {[
                ["命令", "预先声明"],
                ["执行", "全程审计"],
                ["敏感信息", "默认脱敏"],
              ].map(([name, value]) => (
                <div key={name} className="rounded-xl border border-border bg-card p-4">
                  <div className="text-xs text-muted-foreground">{name}</div>
                  <div className="mt-2 text-sm font-semibold">{value}</div>
                </div>
              ))}
            </div>
            <div className="rounded-xl bg-[#28211f] p-5 font-mono text-xs leading-6 text-[#eee6dd]">
              <span className="text-[#d48d65]">11:58:32</span>　command accepted
              <br />
              <span className="text-[#7bc693]">11:58:33</span>　policy verified
              <br />
              <span className="text-[#7bc693]">11:58:34</span>　audit record persisted
            </div>
          </div>
        </div>
      </section>
    </main>
  );
}
