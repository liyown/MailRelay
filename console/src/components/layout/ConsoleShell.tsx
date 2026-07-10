import { useMemo, useState } from "react";
import { SidebarSimple } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";
import { TooltipProvider } from "@/components/ui/tooltip";
import type { Session } from "@/lib/api";
import { DashboardPage } from "@/features/dashboard/DashboardPage";
import { CommandsPage } from "@/features/commands/CommandsPage";
import { ExecutionsPage } from "@/features/executions/ExecutionsPage";
import { QueuePage } from "@/features/queue/QueuePage";
import { LogsPage } from "@/features/logs/LogsPage";
import { SettingsPage } from "@/features/settings/SettingsPage";
import { Header } from "./Header";
import { SideNav } from "./SideNav";
import type { PageId } from "./nav";

export function ConsoleShell({ session }: { session: Session }) {
  const [page, setPage] = useState<PageId>("dashboard");
  const [mobileOpen, setMobileOpen] = useState(false);
  const content = useMemo(
    () =>
      ({
        dashboard: <DashboardPage onOpenExecutions={() => setPage("executions")} />,
        commands: <CommandsPage csrf={session.csrf} />,
        executions: <ExecutionsPage />,
        queue: <QueuePage csrf={session.csrf} />,
        logs: <LogsPage />,
        settings: <SettingsPage />,
      })[page],
    [page, session.csrf],
  );
  return (
    <TooltipProvider>
      <div className="h-[100dvh] overflow-hidden bg-background text-foreground" data-user={session.user.id}>
        <aside className="fixed inset-y-0 left-0 z-30 hidden lg:block">
          <SideNav page={page} setPage={setPage} />
        </aside>
        <div className="flex h-full min-w-0 flex-col lg:pl-[224px]">
          <Header
            session={session}
            setPage={setPage}
            onMobileNav={
              <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
                <SheetTrigger asChild>
                  <Button aria-label="打开导航" className="lg:hidden" variant="outline" size="icon">
                    <SidebarSimple />
                  </Button>
                </SheetTrigger>
                <SheetContent side="left" className="w-[270px] p-0">
                  <SideNav page={page} setPage={setPage} compact onNavigate={() => setMobileOpen(false)} />
                </SheetContent>
              </Sheet>
            }
          />
          <main className="min-h-0 flex-1 overflow-y-auto">
            <div className="mx-auto max-w-[1500px] p-4 lg:p-6">{content}</div>
          </main>
        </div>
      </div>
    </TooltipProvider>
  );
}
