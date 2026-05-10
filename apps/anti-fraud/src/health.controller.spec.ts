import { Test } from '@nestjs/testing';
import { HealthController } from './health.controller';

describe('HealthController', () => {
  let controller: HealthController;

  beforeAll(async () => {
    const moduleRef = await Test.createTestingModule({
      controllers: [HealthController],
    }).compile();
    controller = moduleRef.get(HealthController);
  });

  it('healthz returns ok', () => {
    expect(controller.healthz()).toEqual({ status: 'ok' });
  });

  it('readyz returns ready', () => {
    expect(controller.readyz()).toEqual({ status: 'ready' });
  });
});
