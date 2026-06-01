export type Tone = 'ok' | 'warn' | 'bad' | 'idle';

export function registeredLabel(value?: boolean, flow?: string) {
  if (value === true) return '已注册';
  if (value === false) return '未注册';
  if (flow === 'not_registered') return '未检测到';
  return '未知';
}

export function booleanLabel(value?: boolean) {
  if (value === true) return '是';
  if (value === false) return '否';
  return '未知';
}

export function smsLabel(available?: boolean, waitSeconds?: number | null) {
  if (waitSeconds && waitSeconds > 0) return `冷却 ${formatSeconds(waitSeconds)}`;
  if (available === true) return '可发';
  if (available === false) return '不可发';
  return '未知';
}

export function cooldownLabel(value?: number | null) {
  return value && value > 0 ? `冷却 ${formatSeconds(value)}` : '';
}

export function toneClass(tone: Tone, chip = false) {
  const base = tone === 'ok' ? 'text-primary' : tone === 'bad' ? 'text-destructive' : tone === 'warn' ? 'text-amber-600 dark:text-amber-400' : 'text-muted-foreground';
  if (!chip) return base;
  if (tone === 'ok') return 'border-primary/30 bg-primary/5 text-primary';
  if (tone === 'bad') return 'border-destructive/30 bg-destructive/5 text-destructive';
  if (tone === 'warn') return 'border-amber-500/30 bg-amber-500/5 text-amber-700 dark:text-amber-300';
  return 'bg-muted/30 text-muted-foreground';
}

export function methodLabels(value: unknown) {
  const raw = Array.isArray(value) ? value.map(textOf) : textOf(value).split(',');
  const seen = new Set<string>();
  return raw.map(methodLabel).filter((label) => label && !seen.has(label) && seen.add(label));
}

function formatSeconds(value: number) {
  if (value < 60) return `${Math.ceil(value)}s`;
  const minutes = Math.ceil(value / 60);
  if (minutes < 60) return `${minutes}m`;
  return `${Math.ceil(minutes / 60)}h`;
}

export function methodLabel(value: string) {
  const normalized = value.trim().toUpperCase().replace(/^VERIFICATION_DELIVERY_METHOD_/, '');
  if (!normalized || normalized === 'UNSPECIFIED') return '';
  if (normalized === 'SMS') return 'SMS';
  if (normalized === 'VOICE') return '语音';
  if (normalized === 'IN_APP_MESSAGE') return 'App 验证';
  return normalized.replaceAll('_', ' ').toLowerCase().replace(/\b\w/g, (char) => char.toUpperCase());
}

function textOf(value: unknown) {
  return typeof value === 'string' ? value.trim() : '';
}
