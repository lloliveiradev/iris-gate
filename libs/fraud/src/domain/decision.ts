export const Decision = {
  APPROVE: 'APPROVE',
  REVIEW: 'REVIEW',
  REJECT: 'REJECT',
} as const;

export type Decision = (typeof Decision)[keyof typeof Decision];

/** Severity order: APPROVE < REVIEW < REJECT. */
const SEVERITY: Readonly<Record<Decision, number>> = {
  APPROVE: 0,
  REVIEW: 1,
  REJECT: 2,
};

export function isDecision(value: unknown): value is Decision {
  return value === 'APPROVE' || value === 'REVIEW' || value === 'REJECT';
}

/** Returns the more severe of two decisions (APPROVE < REVIEW < REJECT). */
export function mostSevere(a: Decision, b: Decision): Decision {
  return SEVERITY[b] > SEVERITY[a] ? b : a;
}
