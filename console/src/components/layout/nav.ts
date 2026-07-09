import type { ComponentType } from "react";
import { ChartDonut, Envelope, Gear, ListChecks, PaperPlaneTilt, Pulse, ShieldCheck } from "@phosphor-icons/react";

export type PageId = "dashboard" | "commands" | "executions" | "queue" | "logs" | "security" | "settings";

type NavItem = { id: PageId; label: string; icon: ComponentType<{ className?: string }> };

export const navItems: readonly NavItem[] = [
  { id: "dashboard", label: "仪表盘", icon: ChartDonut },
  { id: "commands", label: "Command", icon: PaperPlaneTilt },
  { id: "executions", label: "执行记录", icon: ListChecks },
  { id: "queue", label: "Queue 与死信", icon: Envelope },
  { id: "logs", label: "运行日志", icon: Pulse },
  { id: "security", label: "邮箱与安全", icon: ShieldCheck },
  { id: "settings", label: "系统设置", icon: Gear },
] as const;
