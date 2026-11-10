import { Global, Module } from '@nestjs/common';
import { DATABASE_CONFIG, loadDatabaseConfig } from '../../config/database.config';
import { PrismaService } from './prisma.service';

@Global()
@Module({
  providers: [{ provide: DATABASE_CONFIG, useFactory: loadDatabaseConfig }, PrismaService],
  exports: [PrismaService],
})
export class PrismaModule {}
