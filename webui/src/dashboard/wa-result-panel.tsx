import { Badge } from '@byte-v-forge/common-ui';
import type { WaWorkflowResponse } from './wa-api';
import { booleanLabel, cooldownLabel, registeredLabel, smsLabel, toneClass, type Tone } from './wa-result-labels';
import { metaItems, outcomeMeta, waProbeStatus, type VerificationMethodStatus, type WaProbeStatus } from './wa-result-model';

export function WaResultPanel({ title, phone, result, loading }: { title: string; phone?: string; result?: WaWorkflowResponse | null; loading?: boolean }) {
  const status = waProbeStatus(result);
  const outcome = outcomeMeta(status, result, loading);
  const meta = metaItems(status, result);
  return (
    <div className="grid gap-2">
      <div className="flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-baseline gap-2">
          <span className="shrink-0 text-xs font-medium">{title}</span>
          <span className="truncate font-mono text-[11px] text-muted-foreground">{phone || '-'}</span>
        </div>
        <Badge variant={outcome.variant}>{outcome.label}</Badge>
      </div>
      <div className="flex flex-wrap gap-1.5">{status.requestFailed ? <FailedMetrics /> : <NormalMetrics status={status} />}</div>
      {(status.methodStatuses.length > 0 || meta.length > 0) && <div className="flex flex-wrap items-center gap-1.5 text-[11px]">{!status.requestFailed && <MethodGroup methods={status.methodStatuses} />}{meta.map((item) => <MetaItem key={item.label} {...item} />)}</div>}
    </div>
  );
}

function FailedMetrics() {
  return <MetricChip label="请求" value="失败" tone="bad" />;
}

function NormalMetrics({ status }: { status: WaProbeStatus }) {
  const registrationTone = status.registered === true ? 'warn' : status.accountFlow === 'not_registered' || status.registered === false ? 'ok' : 'idle';
  return (
    <>
      <MetricChip label="注册" value={registeredLabel(status.registered, status.accountFlow)} tone={registrationTone} />
      <MetricChip label="SMS" value={smsLabel(status.smsAvailable, status.smsWaitSeconds)} tone={status.smsAvailable === true && !status.smsWaitSeconds ? 'ok' : status.smsAvailable === false || Boolean(status.smsWaitSeconds) ? 'warn' : 'idle'} />
      <MetricChip label="封禁" value={booleanLabel(status.blocked)} tone={status.blocked === true ? 'bad' : status.blocked === false ? 'ok' : 'idle'} />
    </>
  );
}

function MetricChip({ label, value, tone }: { label: string; value: string; tone: Tone }) {
  return <span className={`inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-[11px] ${toneClass(tone, true)}`}><span className="text-muted-foreground">{label}</span><span className="font-semibold">{value}</span></span>;
}

function MethodGroup({ methods }: { methods: VerificationMethodStatus[] }) {
  if (methods.length === 0) return null;
  return <span className="inline-flex items-center gap-1 text-muted-foreground"><span>方式</span>{methods.map((method) => <span key={method.key} className="rounded bg-muted/60 px-1.5 py-0.5 font-medium text-foreground">{method.label}{cooldownLabel(method.cooldownSeconds) ? ` · ${cooldownLabel(method.cooldownSeconds)}` : ''}</span>)}</span>;
}

function MetaItem({ label, value, tone = 'idle' }: { label: string; value: string; tone?: Tone }) {
  return <span className="inline-flex min-w-0 items-center gap-1 text-muted-foreground"><span>{label}</span><span className={`max-w-[220px] truncate font-medium ${toneClass(tone)}`} title={value}>{value}</span></span>;
}
