import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle, ControlledInputField, ControlledSelectField, ControlledTextareaField, useForm } from '@byte-v-forge/common-ui';
import { waCountryOptions } from './wa-countries';
import { parseNumberLines, type WaManagedNumber } from './wa-utils';

type FormValues = { workspace_id: string; region: string; numbers: string };

export function WaNumberImport({ onAdd }: { onAdd: (rows: WaManagedNumber[]) => void }) {
  const form = useForm<FormValues>({ defaultValues: { workspace_id: 'default', region: 'ID', numbers: '' } });
  const submit = form.handleSubmit((values) => {
    const rows = parseNumberLines(values.workspace_id.trim() || 'default', values.region, values.numbers);
    if (rows.length > 0) onAdd(rows);
    form.setValue('numbers', '');
  });
  return (
    <Card>
      <CardHeader>
        <CardTitle>导入号码</CardTitle>
        <CardDescription>只选择号码国家/拨号码；WA 请求统一走美国随机动态 IP。</CardDescription>
      </CardHeader>
      <CardContent>
        <form className="grid gap-4" onSubmit={submit}>
          <div className="grid gap-3 md:grid-cols-2">
            <ControlledInputField control={form.control} name="workspace_id" label="Workspace" />
            <ControlledSelectField control={form.control} name="region" label="号码国家/拨号码" options={waCountryOptions.map((item) => ({ value: item.value, label: item.label }))} />
          </div>
          <ControlledTextareaField control={form.control} name="numbers" label="号码列表" rows={8} placeholder={'81234567890\n+6281234567890'} />
          <Button type="submit">加入号码池</Button>
        </form>
      </CardContent>
    </Card>
  );
}
