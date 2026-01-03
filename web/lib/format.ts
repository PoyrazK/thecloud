export function formatDateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

export function formatShortID(value: string): string {
  if (!value) {
    return '-';
  }
  return value.length > 8 ? value.slice(0, 8) : value;
}
