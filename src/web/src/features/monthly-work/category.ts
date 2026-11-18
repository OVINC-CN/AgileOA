export type WorkCategory = 'student' | 'cpc';

export function workAction(category: WorkCategory, operation: string) {
  return `${category === 'cpc' ? 'cpc.' : ''}monthly_work.${operation}`;
}
