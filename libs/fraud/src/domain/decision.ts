export const Decision = {
  APPROVE: 'APPROVE',
  REVIEW: 'REVIEW',
  REJECT: 'REJECT',
} as const;

export type Decision = (typeof Decision)[keyof typeof Decision];

export function isDecision(value: unknown): value is Decision {
  return value === 'APPROVE' || value === 'REVIEW' || value === 'REJECT';
}
