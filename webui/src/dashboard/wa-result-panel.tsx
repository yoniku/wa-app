import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle, DescriptionLine } from '@byte-v-forge/common-ui';
import type { WaConnectionState, WaWorkflowResponse } from './wa-api';
import { connectionHealthy, connectionStatusLabel, proxyArea, redactSensitive, resultStatus } from './wa-utils';

export function WaResultPanel({ title, result, connection, loading }: { title: string; result?: WaWorkflowResponse | null; connection?: WaConnectionState | null; loading?: boolean }) {
  const ok = Boolean(result?.success || result?.passed);
  const status = loading ? '执行中' : resultStatus(result);
  return (
    <Card className="min-h-0 overflow-hidden">
      <CardHeader className="border-b bg-muted/20">
        <div className="flex items-start justify-between gap-3">
          <div>
            <CardTitle>{title}</CardTitle>
            <CardDescription>登录态、长连接和敏感结果摘要。</CardDescription>
          </div>
          <Badge variant={ok ? 'default' : result ? 'destructive' : 'secondary'}>{status}</Badge>
        </div>
      </CardHeader>
      <CardContent className="grid gap-4 p-4">
        <div className="grid gap-3 sm:grid-cols-3">
          <Metric label="登录态" value={String(result?.login_state?.status || '-')} />
          <Metric label="长连接" value={connectionStatusLabel(connection?.status)} ok={connectionHealthy(connection)} />
          <Metric label="心跳" value={connection?.heartbeat_supported ? 'chatd ping' : '-'} detail={shortTime(connection?.last_heartbeat_at)} />
        </div>
        <div className="grid gap-2 rounded-xl border bg-background p-3">
          <DescriptionLine label="号码状态" value={String(result?.phone_status?.account_status || '-')} />
          <DescriptionLine label="SMS 可发" value={String(result?.phone_status?.sms_status || '-')} />
          <DescriptionLine label="可注册" value={String(result?.phone_status?.can_register ?? '-')} />
          <DescriptionLine label="代理模式" value={proxyArea(result)} />
          <DescriptionLine label="登录态 ID" value={String(result?.login_state?.login_state_id || '-')} />
          <DescriptionLine label="消息 Session" value={String(connection?.message_session_id || '-')} />
          <DescriptionLine label="最近消息" value={shortTime(connection?.last_message_at)} />
          <DescriptionLine label="重连次数" value={String(connection?.reconnect_count ?? '-')} />
        </div>
        <pre className="max-h-[320px] overflow-auto rounded-xl bg-muted/70 p-3 text-xs text-muted-foreground">
          {JSON.stringify(redactSensitive({ ...(result || {}), connection: connection || undefined }), null, 2)}
        </pre>
      </CardContent>
    </Card>
  );
}

function Metric({ label, value, detail, ok }: { label: string; value: string; detail?: string; ok?: boolean }) {
  return <div className="rounded-xl border bg-card p-3"><div className="text-xs text-muted-foreground">{label}</div><div className={ok ? 'mt-1 truncate text-sm font-semibold text-primary' : 'mt-1 truncate text-sm font-semibold'}>{value}</div>{detail && <div className="mt-1 text-xs text-muted-foreground">{detail}</div>}</div>;
}

function shortTime(value?: string) {
  return value ? new Date(value).toLocaleString() : '-';
}
