import { Pulse, Queue, Warning } from "@phosphor-icons/react";
import { PageFrame } from "@/components/layout/PageFrame";
import { MetricCard } from "@/components/common/MetricCard";
import { useDashboard, useJobs, useReplies, flatten } from "@/hooks/queries";
import { QueueTable, type QueueRow } from "./QueueTable";

export function QueuePage({ csrf }: { csrf: string }) {
  const dashboard = useDashboard("24h");
  const jobs = useJobs();
  const replies = useReplies();

  const jobRows: QueueRow[] = flatten(jobs.data?.pages).map((job) => ({
    id: job.id,
    target: job.command,
    status: job.status,
    attempts: `${job.attempts}/${job.max_attempts}`,
    at: job.available_at,
  }));
  const replyRows: QueueRow[] = flatten(replies.data?.pages).map((reply) => ({
    id: reply.id,
    target: reply.recipient,
    status: reply.status,
    attempts: `${reply.attempts}/${reply.max_attempts}`,
    at: reply.available_at,
  }));

  const totals = dashboard.data
    ? {
        pending: dashboard.data.queue.pending + dashboard.data.replies.pending,
        running: dashboard.data.queue.running + dashboard.data.replies.running,
        dead: dashboard.data.queue.dead + dashboard.data.replies.dead,
      }
    : { pending: 0, running: 0, dead: 0 };

  return (
    <PageFrame title="Queue 与死信" description="观察待处理任务、重试与死信状态，并重放死信项">
      <div className="grid gap-4 md:grid-cols-3">
        <MetricCard icon={Queue} label="待处理" value={String(totals.pending)} delta="队列 + 回复" />
        <MetricCard icon={Pulse} label="运行中" value={String(totals.running)} delta="队列 + 回复" good />
        <MetricCard icon={Warning} label="死信" value={String(totals.dead)} delta="需要人工重放" />
      </div>
      <div className="mt-4 space-y-4">
        <QueueTable
          title="命令队列任务"
          kind="job"
          rows={jobRows}
          csrf={csrf}
          isLoading={jobs.isPending}
          isError={jobs.isError}
          hasNextPage={jobs.hasNextPage}
          isFetchingNextPage={jobs.isFetchingNextPage}
          onLoadMore={() => jobs.fetchNextPage()}
        />
        <QueueTable
          title="邮件回复"
          kind="reply"
          rows={replyRows}
          csrf={csrf}
          isLoading={replies.isPending}
          isError={replies.isError}
          hasNextPage={replies.hasNextPage}
          isFetchingNextPage={replies.isFetchingNextPage}
          onLoadMore={() => replies.fetchNextPage()}
        />
      </div>
    </PageFrame>
  );
}
