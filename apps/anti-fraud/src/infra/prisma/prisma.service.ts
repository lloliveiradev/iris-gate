import { Inject, Injectable, Logger, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { PrismaPg } from '@prisma/adapter-pg';
import { DATABASE_CONFIG, type DatabaseConfig } from '../../config/database.config';
import { PrismaClient } from './generated/client';

/** Prisma client for the anti-fraud database, connected through the pg driver adapter. */
@Injectable()
export class PrismaService extends PrismaClient implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(PrismaService.name);

  constructor(@Inject(DATABASE_CONFIG) config: DatabaseConfig) {
    super({ adapter: new PrismaPg({ connectionString: config.url }) });
  }

  async onModuleInit(): Promise<void> {
    await this.$connect();
    this.logger.log('prisma connected');
  }

  async onModuleDestroy(): Promise<void> {
    await this.$disconnect();
    this.logger.log('prisma disconnected');
  }
}
