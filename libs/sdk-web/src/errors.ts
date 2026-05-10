import type { Problem } from './types';

export class IrisApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly problem: Problem | { detail?: string } | null,
    message?: string,
  ) {
    super(message ?? IrisApiError.deriveMessage(status, problem));
    this.name = 'IrisApiError';
  }

  static async fromResponse(res: Response): Promise<IrisApiError> {
    let problem: Problem | { detail?: string } | null = null;
    try {
      const ct = res.headers.get('content-type') ?? '';
      if (ct.includes('application/problem+json') || ct.includes('application/json')) {
        problem = (await res.json()) as Problem;
      } else {
        const text = await res.text();
        problem = text ? { detail: text } : null;
      }
    } catch {
      problem = null;
    }
    return new IrisApiError(res.status, problem);
  }

  private static deriveMessage(status: number, problem: Problem | { detail?: string } | null): string {
    if (problem && typeof problem === 'object') {
      const obj = problem as Record<string, unknown>;
      const detail = typeof obj.detail === 'string' ? obj.detail : undefined;
      const title = typeof obj.title === 'string' ? obj.title : undefined;
      if (detail) return `${status}: ${detail}`;
      if (title) return `${status}: ${title}`;
    }
    return `iris api error: ${status}`;
  }
}
