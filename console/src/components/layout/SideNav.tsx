import { cn } from "@/lib/utils";
import { Logo } from "./Logo";
import { navItems, type PageId } from "./nav";

export function SideNav({
  page,
  setPage,
  compact = false,
  onNavigate,
}: {
  page: PageId;
  setPage: (page: PageId) => void;
  compact?: boolean;
  onNavigate?: () => void;
}) {
  return (
    <div className={cn("flex h-full flex-col bg-sidebar", compact ? "w-full" : "w-[224px] border-r border-border")}>
      <Logo />
      <nav className="space-y-1 px-3" aria-label="主导航">
        {navItems.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            type="button"
            onClick={() => {
              setPage(id);
              onNavigate?.();
            }}
            className={cn(
              "relative flex h-11 w-full items-center gap-3 rounded-lg px-3 text-sm font-medium transition-colors",
              page === id ? "bg-accent text-accent-foreground" : "text-foreground/80 hover:bg-muted",
            )}
          >
            {page === id && <span className="absolute -left-3 h-7 w-0.5 rounded-r bg-primary" />}
            <Icon className="size-[19px]" />
            {label}
          </button>
        ))}
      </nav>
      <div className="mt-auto border-t border-border p-4 text-xs text-muted-foreground">MailRelay 运行控制台</div>
    </div>
  );
}
