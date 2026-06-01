import { useState, type ReactNode } from 'react';
import { RefreshCw, Smartphone, Workflow } from 'lucide-react';
import { ACCOUNT_PAGE_SIZE, AccountManagementView, Alert, AlertDescription, AlertTitle, Badge, Button, Card, CardContent, ToastMessage, WorkspaceTabbedPanel, accountSubject, useAccountPages, useAsyncActionRunner, useQuery, useToastMessage, type AccountListPagination } from '@byte-v-forge/common-ui';
import type { ListWAAccountsResponse } from '../proto/byte/v/forge/waapp/v1/profile';
import { getWaAccounts, getWaHealth, probeWaPhoneSMS, waKeys, type WaAccountProjection, type WaWorkflowResponse } from './wa-api';
import { WaAccountAdd } from './wa-account-add';
import { WaPhoneSMSProbeForm } from './wa-phone-sms-probe-form';
import { WaResultPanel } from './wa-result-panel';
import type { WaResolvedPhone } from './wa-utils';

type WaTab = 'accounts' | 'toolbox' | 'workflows';

const ACCOUNT_WORKSPACE_ID = 'default';

export function WaPage() {
  const toast = useToastMessage();
  const health = useQuery({ queryKey: waKeys.health, queryFn: getWaHealth });
  const [checkedPhone, setCheckedPhone] = useState('');
  const [result, setResult] = useState<WaWorkflowResponse | null>(null);
  const runner = useAsyncActionRunner();
  const accounts = useAccountPages<WaAccountProjection, ListWAAccountsResponse>({
    queryKey: waKeys.accounts(ACCOUNT_WORKSPACE_ID),
    queryFn: (cursor) => getWaAccounts(ACCOUNT_WORKSPACE_ID, cursor),
    refetchInterval: 10000,
    pageSize: ACCOUNT_PAGE_SIZE
  });

  async function probePhoneSMS(target: WaResolvedPhone) {
    setCheckedPhone(target.e164);
    setResult(null);
    await runner.tryRun('wa-phone-sms-probe', async () => {
      const output = await probeWaPhoneSMS(target.input);
      setResult(output);
      toast.showOK('手机号/SMS 探测完成');
    }, { onError: toast.showError });
  }

  return <><ToastMessage toast={toast.toast} /><WorkspaceTabbedPanel<WaTab> defaultValue="accounts" title={<span className="inline-flex items-center gap-2"><Smartphone className="size-4" />WA 管理</span>} meta={`${accounts.accounts.length} 个账号 · ${health.data?.n8n_webhook_configured ? 'n8n 已接入' : '等待 n8n'}`} tabs={[
    { value: 'accounts', label: '账号', content: <WaAccountsTab accounts={accounts.accounts} loading={accounts.isLoading} pagination={accounts.pagination} onAccountAdded={async () => { toast.showOK('WAAccount 已添加'); await accounts.refetch(); }} onError={toast.showError} /> },
    { value: 'toolbox', label: '工具箱', content: <ToolboxTab result={result} phone={checkedPhone} busy={runner.busy} onCheck={probePhoneSMS} onError={toast.showError} /> },
    { value: 'workflows', label: '工作流', content: <WorkflowTab configured={Boolean(health.data?.n8n_webhook_configured)} workflows={health.data?.workflows || []} loading={health.isLoading} /> }
  ]} /></>;
}

function ToolboxTab(props: { result: WaWorkflowResponse | null; phone: string; busy: boolean; onCheck: (target: WaResolvedPhone) => void | Promise<void>; onError: (message: string) => void }) {
  const hasResult = props.busy || props.result || props.phone;
  return <div className="p-3"><WaPhoneSMSProbeForm disabled={props.busy} resultSlot={hasResult ? <WaResultPanel title="探测结果" phone={props.phone} result={props.result} loading={props.busy} /> : undefined} onCheck={props.onCheck} onError={props.onError} /></div>;
}

function WaAccountsTab(props: { accounts: WaAccountProjection[]; loading?: boolean; pagination?: AccountListPagination; onAccountAdded: () => void | Promise<void>; onError: (message: string) => void }) {
  return <AccountManagementView title="WAAccount" icon={<Smartphone size={16} />} actions={<WaAccountAdd disabled={props.loading} onCreated={props.onAccountAdded} onError={props.onError} />} carriers={props.accounts} loading={props.loading} loadingText="加载 WAAccount..." emptyText="暂无已持久化 WAAccount" pagination={props.pagination} config={{ icon: () => <Smartphone size={15} />, title: (record) => <span className="font-mono">{accountSubject(record) || record.key?.account_id}</span>, subtitle: (record) => record.key?.account_id || '', meta: (record) => <span className="text-xs text-muted-foreground">{record.status?.label || record.status?.value || '-'}</span> }} />;
}

function WorkflowTab({ configured, workflows, loading }: { configured: boolean; loading?: boolean; workflows: Array<{ key: string; label: string; webhook_path: string }> }) {
  return <div className="grid gap-4 p-4"><Alert><AlertTitle>{configured ? 'WA n8n 编排已接入' : 'WA n8n webhook 未配置'}</AlertTitle><AlertDescription>{loading ? '加载中...' : '注册流程走 workflow；工具箱号码/SMS 探测、登录态检测、长连接恢复和 OTP MQ 投放由 wa-app 直连服务完成。'}</AlertDescription></Alert><div className="grid gap-3 md:grid-cols-2"><InfoCard icon={<Workflow size={16} />} title="注册流程" badge="n8n" text="跨步骤注册和等待 OTP 仍由 n8n 编排。" /><InfoCard icon={<RefreshCw size={16} />} title="探测 / 登录态 / 长连接" badge="直连" text="号码/SMS 探测使用 1 分钟动态 IP 短租约，用完释放；登录态和长连接不进入 workflow。" /></div><div className="grid gap-2">{workflows.map((item) => <div key={item.key} className="flex items-center justify-between rounded-xl border bg-card p-3 text-sm"><span>{item.label}</span><code className="text-xs text-muted-foreground">{item.webhook_path}</code></div>)}</div><Button variant="outline" asChild><a href="/workflow" target="_blank" rel="noreferrer">打开 Workflow 状态页</a></Button></div>;
}

function InfoCard({ icon, title, badge, text }: { icon: ReactNode; title: string; badge: string; text: string }) {
  return <Card><CardContent className="grid gap-2 p-4"><div className="flex items-center justify-between"><div className="flex items-center gap-2 font-medium">{icon}{title}</div><Badge variant="outline">{badge}</Badge></div><p className="text-sm text-muted-foreground">{text}</p></CardContent></Card>;
}
