import {
  createFeatureSnapshot,
  FeatureSnapshot,
  FeatureSnapshotQuery,
  FeatureSnapshotReader,
} from './feature-snapshot';
import { IDGen } from './ports';

export class SequentialIdGen implements IDGen {
  constructor(
    private counter = 0,
    private readonly prefix = 'gen',
  ) {}

  newId(): string {
    this.counter += 1;
    return `${this.prefix}_${String(this.counter).padStart(3, '0')}`;
  }
}

/** In-memory reader: unknown cards read as first-time cards. */
export class InMemoryFeatureSnapshotReader implements FeatureSnapshotReader {
  private readonly snapshots = new Map<string, FeatureSnapshot>();
  readonly queries: FeatureSnapshotQuery[] = [];

  set(snapshot: FeatureSnapshot): void {
    this.snapshots.set(snapshot.cardFingerprint, createFeatureSnapshot(snapshot));
  }

  async load(query: FeatureSnapshotQuery): Promise<FeatureSnapshot> {
    this.queries.push({ ...query });
    return (
      this.snapshots.get(query.cardFingerprint) ??
      createFeatureSnapshot({
        cardFingerprint: query.cardFingerprint,
        attempts24h: 0,
        declines24h: 0,
        avgCapturedCents: null,
        blocked: false,
      })
    );
  }
}
