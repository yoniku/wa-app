import type { WaConnectionState, WaPhoneInput, WaWorkflowResponse } from './wa-api';
import { countryByValue, countryFromE164 } from './wa-countries';

export type WaManagedNumber = {
  id: string;
  e164: string;
  input: WaPhoneInput;
  status: 'idle' | 'probing' | 'probe_passed' | 'probe_failed' | 'registering' | 'registered' | 'failed';
  updated_at?: string;
  result?: WaWorkflowResponse | null;
};

export function parseNumberLines(workspaceID: string, region: string, text: string): WaManagedNumber[] {
  return text.split(/\r?\n|,/).map((line) => line.trim()).filter(Boolean).map((line) => numberFromLine(workspaceID, region, line));
}

export function numberFromLine(workspaceID: string, region: string, line: string): WaManagedNumber {
  const option = line.startsWith('+') ? countryFromE164(line) || countryByValue(region) : countryByValue(region);
  const digits = line.replace(/\D+/g, '');
  const national = line.startsWith('+') && digits.startsWith(option.callingCode) ? digits.slice(option.callingCode.length) : digits;
  const e164 = `+${option.callingCode}${national}`;
  return { id: `${workspaceID}:${option.value}:${e164}`, e164, input: { workspace_id: workspaceID, region: option.value, phone: national }, status: 'idle' };
}

export function mergeNumbers(current: WaManagedNumber[], incoming: WaManagedNumber[]) {
  const byID = new Map(current.map((item) => [item.id, item]));
  for (const item of incoming) byID.set(item.id, byID.get(item.id) || item);
  return [...byID.values()].sort((left, right) => left.e164.localeCompare(right.e164));
}

export function withResult(row: WaManagedNumber, result: WaWorkflowResponse, action: 'probe' | 'register'): WaManagedNumber {
  const ok = Boolean(result.success || result.passed);
  const status = action === 'register' ? (ok ? 'registered' : 'failed') : (ok ? 'probe_passed' : 'probe_failed');
  return { ...row, status, result, updated_at: new Date().toLocaleString() };
}

export function resultStatus(result?: WaWorkflowResponse | null) {
  if (!result) return '未执行';
  if (result.success || result.passed) return result.status || '通过';
  return result.status || result.error_message || '失败';
}

export function managedStatusLabel(status: WaManagedNumber['status']) {
  return ({ idle: '待处理', probing: '检测中', probe_passed: '检测通过', probe_failed: '检测失败', registering: '注册中', registered: '已注册', failed: '失败' } as const)[status];
}

export function connectionStatusLabel(status?: string) {
  return ({ LONG_CONNECTION_STATUS_STARTING: '启动中', LONG_CONNECTION_STATUS_CONNECTED: '已连接', LONG_CONNECTION_STATUS_HEARTBEAT_WAITING: '心跳等待', LONG_CONNECTION_STATUS_RECONNECTING: '重连中', LONG_CONNECTION_STATUS_FAILED: '失败', LONG_CONNECTION_STATUS_STOPPED: '已停止' } as Record<string, string>)[status || ''] || '未连接';
}

export function connectionHealthy(connection?: WaConnectionState | null) {
  return connection?.status === 'LONG_CONNECTION_STATUS_CONNECTED' || connection?.status === 'LONG_CONNECTION_STATUS_HEARTBEAT_WAITING';
}

export function proxyArea(result?: WaWorkflowResponse | null) {
  const proxy = result?.proxy || {};
  const mode = text(proxy.proxy_mode);
  if (mode === 'US_RANDOM_DYNAMIC_IP') return '美国随机动态 IP';
  return mode || '-';
}

export function redactSensitive(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(redactSensitive);
  if (!value || typeof value !== 'object') return value;
  return Object.fromEntries(Object.entries(value).map(([key, nested]) => [key, sensitiveKey(key) ? '***' : redactSensitive(nested)]));
}

function sensitiveKey(key: string) {
  return /(otp|code|token|auth|key|cookie|secret|password|session|enc|body|proxy_url)/i.test(key);
}

function text(value: unknown) {
  return typeof value === 'string' ? value : '';
}
