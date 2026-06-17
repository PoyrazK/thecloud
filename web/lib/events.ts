export type EventStatus = 'success' | 'failure';

export function eventStatus(action: string): EventStatus {
  const normalized = action.toLowerCase();
  if (normalized.includes('fail') || normalized.includes('error') || normalized.includes('deny')) {
    return 'failure';
  }
  return 'success';
}
