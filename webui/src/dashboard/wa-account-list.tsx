import { Smartphone } from 'lucide-react';
import { ACCOUNT_PAGE_SIZE, AccountList, accountCarrierMap, accountKey, accountRecordsFromCarriers, accountSubject } from '@byte-v-forge/common-ui';
import type { WaAccountProjection } from './wa-api';

export function WaAccountList({ accounts, loading, hasNext, loadingNext, onLoadMore, onSelect }: {
  accounts: WaAccountProjection[];
  loading?: boolean;
  hasNext?: boolean;
  loadingNext?: boolean;
  onLoadMore?: () => void;
  onSelect?: (account: WaAccountProjection) => void;
}) {
  const records = accountRecordsFromCarriers(accounts);
  const byKey = accountCarrierMap(accounts);
  if (loading && records.length === 0) return <div className="rounded-xl border bg-card p-4 text-sm text-muted-foreground">加载 WAAccount...</div>;
  return (
    <AccountList
      accounts={records}
      emptyText="暂无已持久化 WAAccount"
      onSelect={onSelect ? (record) => {
        const item = byKey.get(accountKey(record));
        if (item) onSelect(item);
      } : undefined}
      config={{
        icon: () => <Smartphone size={15} />,
        title: (record) => <span className="font-mono">{accountSubject(record) || record.key?.account_id}</span>,
        meta: (record) => <span className="text-xs text-muted-foreground">{record.key?.account_id}</span>
      }}
      pagination={onLoadMore ? { pageSize: ACCOUNT_PAGE_SIZE, hasNext, loading: loadingNext, onLoadMore } : undefined}
    />
  );
}
