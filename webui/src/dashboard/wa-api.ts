import { ACCOUNT_PAGE_SIZE, api, fetchAccountList } from '@byte-v-forge/common-ui';
import type { GetLongConnectionStatusResponse, LongConnectionState } from '../proto/byte/v/forge/waapp/v1/messaging';
import type { CreateWAAccountResponse, ListWAAccountsResponse, WAAccount } from '../proto/byte/v/forge/waapp/v1/profile';

export type WaPhoneInput = {
  workspace_id: string;
  region: string;
  phone: string;
  e164_number: string;
  country_calling_code: string;
  country_iso2: string;
};

export type WaWorkflowResponse = {
  success?: boolean;
  passed?: boolean;
  request_failed?: boolean;
  status?: string;
  error_message?: string;
  phone_status?: Record<string, unknown>;
  account_probe?: Record<string, unknown>;
  sms_probe?: Record<string, unknown>;
  phone?: Record<string, unknown>;
  proxy?: Record<string, unknown>;
  registration?: Record<string, unknown>;
  login_state?: Record<string, unknown>;
  check?: Record<string, unknown>;
};

export type WaConnectionState = LongConnectionState;
export type WaAccountProjection = WAAccount;

export type WaHealthResponse = {
  ok: boolean;
  n8n_webhook_configured: boolean;
  workflows: Array<{ key: string; label: string; webhook_path: string }>;
};

export const waKeys = {
  health: ['wa', 'health'] as const,
  accounts: (workspaceId: string) => ['wa', 'accounts', workspaceId] as const,
  connections: (workspaceId: string) => ['wa', 'connections', workspaceId] as const
};

export function getWaHealth() {
  return api<WaHealthResponse>('/api/wa/health');
}

export function getWaConnections(workspaceId: string) {
  return api<GetLongConnectionStatusResponse>(`/api/wa/long-connections?workspace_id=${encodeURIComponent(workspaceId || 'default')}`);
}

export function getWaAccounts(workspaceId: string, cursor = '') {
  return fetchAccountList<WAAccount, ListWAAccountsResponse>({
    path: '/api/wa/accounts',
    cursor,
    limit: ACCOUNT_PAGE_SIZE,
    params: { workspace_id: workspaceId || 'default' }
  });
}

export async function createWaAccount(input: { phone: string; country_calling_code: string }, workspaceId = 'default') {
  const resp = await api<CreateWAAccountResponse>('/api/wa/accounts', {
    method: 'POST',
    body: JSON.stringify({ workspace_id: workspaceId, phone: input.phone, country_calling_code: input.country_calling_code })
  });
  if (resp.error?.message) throw new Error(resp.error.message);
  if (!resp.account) throw new Error('WAAccount response is empty');
  return resp.account;
}

export function probeWaPhoneSMS(input: WaPhoneInput) {
  return api<WaWorkflowResponse>('/api/wa/phone/sms-probe', { method: 'POST', body: JSON.stringify(input) });
}

export function checkWaLoginState(input: { workspace_id?: string; login_state_id?: string; registered_identity_id?: string; wa_account_id?: string; client_profile_id?: string; remote_timeout_seconds?: number }) {
  return api<WaWorkflowResponse>('/api/wa/login-state-check', { method: 'POST', body: JSON.stringify(input) });
}
