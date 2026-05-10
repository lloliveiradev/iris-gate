import type { Merchant } from '@/lib/contracts';

const sample: Pick<Merchant, 'legal_name' | 'status'> = {
  legal_name: 'ACME Pagamentos LTDA',
  status: 'pending',
};

export default function HomePage() {
  return (
    <main style={{ padding: 24, fontFamily: 'system-ui, sans-serif' }}>
      <h1>iris-gate</h1>
      <p>Web placeholder — telas reais entram na Fase 5 do roadmap.</p>
      <p>
        contrato wiring: <code>{sample.legal_name}</code> · status: <code>{sample.status}</code>
      </p>
    </main>
  );
}
