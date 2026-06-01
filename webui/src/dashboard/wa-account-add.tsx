import { AccountAddDialog, ControlledInputFieldList } from '@byte-v-forge/common-ui';
import type { ControlledInputFieldDescriptor } from '@byte-v-forge/common-ui';
import { createWaAccount } from './wa-api';

type WaAddAccountValues = { phone: string; country_calling_code: string };

export function WaAccountAdd({ disabled, onCreated, onError }: {
  disabled?: boolean;
  onCreated: () => void | Promise<void>;
  onError: (message: string) => void;
}) {
  return (
    <AccountAddDialog<WaAddAccountValues>
      formId="wa-add-account-form"
      title="添加 WAAccount"
      description="输入手机号和国家拨号码；服务端归一化为 WAAccount。"
      defaultValues={{ phone: '', country_calling_code: '' }}
      disabled={disabled}
      submitDisabled={(values) => !values.phone.trim() || !values.country_calling_code.replace(/\D+/g, '')}
      onError={onError}
      onDone={onCreated}
      onSubmit={(values) => createWaAccount({ phone: values.phone, country_calling_code: values.country_calling_code })}
    >
      {(form) => <ControlledInputFieldList control={form.control} fields={waAddFields} />}
    </AccountAddDialog>
  );
}

const waAddFields: ControlledInputFieldDescriptor<WaAddAccountValues>[] = [{
  id: 'country_calling_code',
  name: 'country_calling_code',
  label: '拨号码',
  placeholder: '+1',
  inputId: 'wa-add-country-calling-code',
}, {
  id: 'phone',
  name: 'phone',
  label: '手机号',
  placeholder: '4155550123',
  inputId: 'wa-add-phone',
}];
