import type { ReactNode } from "react";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

export function Panel({ children, className }: { children: ReactNode; className?: string }) {
  return <Card className={cn("shadow-none", className)}>{children}</Card>;
}

export function PanelTitle({ children, action }: { children: ReactNode; action?: ReactNode }) {
  return (
    <CardHeader className="flex-row items-center justify-between border-b border-border px-5 py-4">
      <CardTitle className="text-base font-semibold">{children}</CardTitle>
      {action}
    </CardHeader>
  );
}
