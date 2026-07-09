import { Logo } from "@/components/layout/Logo";
import { ConsoleShell } from "@/components/layout/ConsoleShell";
import { LoginPage } from "@/features/auth/LoginPage";
import { useSession } from "@/hooks/queries";

export function App() {
  const session = useSession();
  if (session.isPending) {
    return (
      <div className="grid min-h-screen place-items-center">
        <div className="text-center">
          <Logo />
          <p className="text-sm text-muted-foreground">正在连接运行控制台...</p>
        </div>
      </div>
    );
  }
  if (session.error) return <LoginPage />;
  return <ConsoleShell session={session.data} />;
}
