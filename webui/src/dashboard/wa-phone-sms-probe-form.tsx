import type { ReactNode } from 'react';
import { Search } from 'lucide-react';
import { Button, Card, CardContent, Controller, Input, Label, useForm, type Control } from '@byte-v-forge/common-ui';
import { resolveWaPhoneTarget, type WaResolvedPhone } from './wa-utils';

type FormValues = { phone: string; country_calling_code: string };

export function WaPhoneSMSProbeForm({ disabled, resultSlot, onCheck, onError }: {
  disabled?: boolean;
  resultSlot?: ReactNode;
  onCheck: (target: WaResolvedPhone) => void | Promise<void>;
  onError: (message: string) => void;
}) {
  const form = useForm<FormValues>({ defaultValues: { phone: '', country_calling_code: '' } });
  const submit = form.handleSubmit((values) => {
    const result = resolveWaPhoneTarget(values.phone, values.country_calling_code);
    if (!result.target) {
      onError(result.error || '请输入 E.164 格式手机号，或补充国家拨号码。');
      return;
    }
    void onCheck(result.target);
  });
  return (
    <Card className="w-full">
      <CardContent className="p-3">
        <div className="flex flex-wrap items-end gap-2">
          <div className="mb-1.5 mr-1 min-w-[5.5rem] text-sm font-medium">手机号/SMS 探测</div>
          <form className="flex shrink-0 flex-wrap items-end gap-2" onSubmit={submit}>
            <CompactInput control={form.control} name="country_calling_code" label="拨号码" placeholder="+992" className="w-[86px]" />
            <CompactInput control={form.control} name="phone" label="手机号" placeholder="007886231" className="w-[180px] sm:w-[220px]" />
            <Button className="size-8" type="submit" size="icon" aria-label="探测手机号和 SMS 状态" title="探测手机号和 SMS 状态" disabled={disabled}>
              <Search size={16} />
            </Button>
          </form>
          <div className="min-h-[58px] min-w-[300px] flex-1 rounded-lg border bg-muted/20 p-2">
            {resultSlot || <div className="flex h-full items-center text-xs text-muted-foreground">结果：注册 / SMS / Blocked</div>}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

function CompactInput({ control, name, label, placeholder, className }: {
  control: Control<FormValues>;
  name: keyof FormValues;
  label: string;
  placeholder: string;
  className: string;
}) {
  return (
    <div className={className}>
      <Label className="mb-1 text-[11px] text-muted-foreground">{label}</Label>
      <Controller control={control} name={name} render={({ field }) => <Input {...field} value={field.value || ''} type="tel" inputMode={name === 'country_calling_code' ? 'numeric' : 'tel'} placeholder={placeholder} className="h-8" />} />
    </div>
  );
}
