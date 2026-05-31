import { Badge, Button, Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyTitle, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@byte-v-forge/common-ui';
import type { WaConnectionState } from './wa-api';
import { connectionHealthy, connectionStatusLabel, managedStatusLabel, proxyArea, type WaManagedNumber } from './wa-utils';

export function WaNumberTable({ rows, selectedId, busyId, connectionForRow, onSelect, onProbe, onRegister, onRemove }: {
  rows: WaManagedNumber[];
  selectedId?: string;
  busyId?: string;
  connectionForRow?: (row: WaManagedNumber) => WaConnectionState | null;
  onSelect: (row: WaManagedNumber) => void;
  onProbe: (row: WaManagedNumber) => void;
  onRegister: (row: WaManagedNumber) => void;
  onRemove: (row: WaManagedNumber) => void;
}) {
  if (rows.length === 0) return <Empty className="rounded-lg border py-10"><EmptyHeader><EmptyTitle>还没有号码</EmptyTitle><EmptyDescription>从左侧批量导入号码后，可按行检测、注册并观察长连接。</EmptyDescription></EmptyHeader><EmptyContent /></Empty>;
  return (
    <div className="overflow-hidden rounded-xl border bg-card">
      <Table>
        <TableHeader><TableRow><TableHead>号码</TableHead><TableHead>国家</TableHead><TableHead>代理</TableHead><TableHead>注册</TableHead><TableHead>长连接</TableHead><TableHead>更新时间</TableHead><TableHead className="text-right">动作</TableHead></TableRow></TableHeader>
        <TableBody>{rows.map((row) => {
          const busy = busyId === row.id;
          const connection = connectionForRow?.(row);
          return <TableRow key={row.id} data-state={selectedId === row.id ? 'selected' : undefined} onClick={() => onSelect(row)} className="cursor-pointer align-middle">
            <TableCell><div className="font-mono text-sm font-medium">{row.e164}</div><div className="text-xs text-muted-foreground">{row.input.workspace_id}</div></TableCell>
            <TableCell>{row.input.region}</TableCell>
            <TableCell className="text-xs text-muted-foreground">{proxyArea(row.result)}</TableCell>
            <TableCell><Badge variant={row.status === 'registered' || row.status === 'probe_passed' ? 'default' : row.status === 'failed' || row.status === 'probe_failed' ? 'destructive' : 'secondary'}>{managedStatusLabel(row.status)}</Badge></TableCell>
            <TableCell><Badge variant={connectionHealthy(connection) ? 'default' : connection?.status === 'LONG_CONNECTION_STATUS_FAILED' ? 'destructive' : 'outline'}>{connectionStatusLabel(connection?.status)}</Badge></TableCell>
            <TableCell className="text-xs text-muted-foreground">{row.updated_at || '-'}</TableCell>
            <TableCell className="space-x-2 text-right" onClick={(event) => event.stopPropagation()}>
              <Button size="sm" variant="outline" disabled={Boolean(busyId)} onClick={() => onProbe(row)}>{busy && row.status === 'probing' ? '检测中' : '检测'}</Button>
              <Button size="sm" disabled={Boolean(busyId)} onClick={() => onRegister(row)}>{busy && row.status === 'registering' ? '注册中' : '注册'}</Button>
              <Button size="sm" variant="ghost" disabled={Boolean(busyId)} onClick={() => onRemove(row)}>移除</Button>
            </TableCell>
          </TableRow>;
        })}</TableBody>
      </Table>
    </div>
  );
}
