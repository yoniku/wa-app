export type WaCountryOption = { value: string; label: string; callingCode: string };

export const waCountryOptions: WaCountryOption[] = [
  { value: 'ID', label: '印度尼西亚 +62', callingCode: '62' },
  { value: 'US', label: '美国 +1', callingCode: '1' },
  { value: 'IN', label: '印度 +91', callingCode: '91' },
  { value: 'PH', label: '菲律宾 +63', callingCode: '63' },
  { value: 'VN', label: '越南 +84', callingCode: '84' },
  { value: 'TH', label: '泰国 +66', callingCode: '66' },
  { value: 'GB', label: '英国 +44', callingCode: '44' },
  { value: 'BR', label: '巴西 +55', callingCode: '55' }
];

export function countryByValue(value: string) {
  const normalized = value.trim().toUpperCase().replace(/^\+/, '');
  return waCountryOptions.find((item) => item.value === normalized || item.callingCode === normalized) || waCountryOptions[1];
}

export function countryFromE164(value: string) {
  const digits = value.replace(/\D+/g, '');
  return [...waCountryOptions].sort((a, b) => b.callingCode.length - a.callingCode.length).find((item) => digits.startsWith(item.callingCode));
}
