import { Decision } from './decision';
import {
  AssessmentAlreadyCompletedError,
  InvalidAssessmentInputError,
} from './errors';
import { FraudReason } from './reason';

export const AssessmentStatus = {
  PENDING: 'PENDING',
  COMPLETED: 'COMPLETED',
  FAILED: 'FAILED',
} as const;

export type AssessmentStatus = (typeof AssessmentStatus)[keyof typeof AssessmentStatus];

export interface RiskAssessmentInput {
  id: string;
  paymentIntentId: string;
  merchantId: string;
  accountRef: string;
  amountCents: number;
  currency: string;
  requestedAt: Date;
}

export interface RiskAssessmentSnapshot {
  id: string;
  paymentIntentId: string;
  merchantId: string;
  accountRef: string;
  amountCents: number;
  currency: string;
  status: AssessmentStatus;
  decision: Decision | null;
  score: number;
  reasons: FraudReason[];
  requestedAt: Date;
  completedAt: Date | null;
}

export class RiskAssessment {
  private constructor(
    public readonly id: string,
    public readonly paymentIntentId: string,
    public readonly merchantId: string,
    public readonly accountRef: string,
    public readonly amountCents: number,
    public readonly currency: string,
    public readonly requestedAt: Date,
    private _status: AssessmentStatus,
    private _decision: Decision | null,
    private _score: number,
    private _reasons: FraudReason[],
    private _completedAt: Date | null,
  ) {}

  static create(input: RiskAssessmentInput): RiskAssessment {
    if (!input.id || !input.paymentIntentId || !input.merchantId || !input.accountRef) {
      throw new InvalidAssessmentInputError('missing required identifiers');
    }
    if (!Number.isInteger(input.amountCents) || input.amountCents <= 0) {
      throw new InvalidAssessmentInputError(`invalid amountCents: ${input.amountCents}`);
    }
    if (!/^[A-Z]{3}$/.test(input.currency)) {
      throw new InvalidAssessmentInputError(`invalid currency: ${input.currency}`);
    }
    return new RiskAssessment(
      input.id,
      input.paymentIntentId,
      input.merchantId,
      input.accountRef,
      input.amountCents,
      input.currency,
      input.requestedAt,
      AssessmentStatus.PENDING,
      null,
      0,
      [],
      null,
    );
  }

  static rehydrate(snapshot: RiskAssessmentSnapshot): RiskAssessment {
    return new RiskAssessment(
      snapshot.id,
      snapshot.paymentIntentId,
      snapshot.merchantId,
      snapshot.accountRef,
      snapshot.amountCents,
      snapshot.currency,
      snapshot.requestedAt,
      snapshot.status,
      snapshot.decision,
      snapshot.score,
      [...snapshot.reasons],
      snapshot.completedAt,
    );
  }

  get status(): AssessmentStatus {
    return this._status;
  }

  get decision(): Decision | null {
    return this._decision;
  }

  get score(): number {
    return this._score;
  }

  get reasons(): readonly FraudReason[] {
    return this._reasons;
  }

  get completedAt(): Date | null {
    return this._completedAt;
  }

  complete(decision: Decision, score: number, reasons: FraudReason[], at: Date): void {
    if (this._status !== AssessmentStatus.PENDING) {
      throw new AssessmentAlreadyCompletedError(`assessment ${this.id} already ${this._status}`);
    }
    this._status = AssessmentStatus.COMPLETED;
    this._decision = decision;
    this._score = score;
    this._reasons = [...reasons];
    this._completedAt = at;
  }

  fail(at: Date): void {
    if (this._status !== AssessmentStatus.PENDING) {
      throw new AssessmentAlreadyCompletedError(`assessment ${this.id} already ${this._status}`);
    }
    this._status = AssessmentStatus.FAILED;
    this._completedAt = at;
  }

  toSnapshot(): RiskAssessmentSnapshot {
    return {
      id: this.id,
      paymentIntentId: this.paymentIntentId,
      merchantId: this.merchantId,
      accountRef: this.accountRef,
      amountCents: this.amountCents,
      currency: this.currency,
      status: this._status,
      decision: this._decision,
      score: this._score,
      reasons: [...this._reasons],
      requestedAt: this.requestedAt,
      completedAt: this._completedAt,
    };
  }
}
