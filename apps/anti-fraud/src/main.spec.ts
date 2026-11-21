import { DEFAULT_PUBLIC_PORT, readPublicPort } from './main';

describe('readPublicPort', () => {
  it('defaults to 13001 when PORT is missing or blank', () => {
    expect(readPublicPort({})).toBe(DEFAULT_PUBLIC_PORT);
    expect(readPublicPort({ PORT: '' })).toBe(DEFAULT_PUBLIC_PORT);
    expect(readPublicPort({ PORT: '   ' })).toBe(DEFAULT_PUBLIC_PORT);
  });

  it.each(['1', '13001', '65535'])('accepts PORT %p', (rawPort) => {
    expect(readPublicPort({ PORT: rawPort })).toBe(Number(rawPort));
  });

  it.each(['0', '-1', '65536', '13001.5', 'not-a-port'])('rejects invalid PORT %p', (rawPort) => {
    expect(() => readPublicPort({ PORT: rawPort })).toThrow(
      `PORT must be an integer in 1..65535, got ${rawPort}`,
    );
  });
});
