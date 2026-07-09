import type { ReactNode } from "react";
import { Skeleton } from "@/components/ui/skeleton";

type DataStateProps = {
  isLoading: boolean;
  isError: boolean;
  isEmpty: boolean;
  emptyText?: string;
  errorText?: string;
  rows?: number;
  children: ReactNode;
};

// DataState centralizes the loading / error / empty presentation so every page
// handles those states the same way.
export function DataState({
  isLoading,
  isError,
  isEmpty,
  emptyText = "暂无数据",
  errorText = "无法读取数据，请稍后重试。",
  rows = 4,
  children,
}: DataStateProps) {
  if (isError) {
    return (
      <div role="alert" className="rounded-lg border border-destructive/20 bg-red-50 p-4 text-sm text-destructive">
        {errorText}
      </div>
    );
  }
  if (isLoading) {
    return (
      <div className="space-y-3 p-1">
        {Array.from({ length: rows }).map((_, index) => (
          <Skeleton key={index} className="h-10 w-full" />
        ))}
      </div>
    );
  }
  if (isEmpty) {
    return <div className="grid h-24 place-items-center text-sm text-muted-foreground">{emptyText}</div>;
  }
  return <>{children}</>;
}
