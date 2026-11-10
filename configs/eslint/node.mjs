// Shared ESLint flat config for TypeScript packages that run on Node
// (apps/anti-fraud, libs/fraud). Each package re-exports it from its own
// eslint.config.mjs, so ESLint still finds a config next to package.json.
import { defineConfig } from 'eslint/config';
import tseslint from 'typescript-eslint';

// Frameworks and drivers that belong in infra/ or in the composition root.
const INFRA_MODULES = ['@nestjs/*', '@prisma/*', 'prisma', 'kafkajs', 'pg', 'express'];

export default defineConfig([
  {
    files: ['src/**/*.ts'],
    extends: [tseslint.configs.recommended],
    languageOptions: {
      parserOptions: {
        ecmaVersion: 'latest',
        sourceType: 'module',
      },
    },
    rules: {
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
    },
  },
  {
    // Clean architecture: domain and application never import frameworks or drivers.
    files: ['src/domain/**/*.ts', 'src/application/**/*.ts'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: INFRA_MODULES,
              message: 'domain/ and application/ must not import frameworks or drivers. Define a port and implement it in infra/.',
            },
          ],
        },
      ],
    },
  },
  {
    // Generated code (the Prisma client in apps/anti-fraud) is not linted.
    ignores: ['dist/', 'node_modules/', 'coverage/', 'src/**/generated/'],
  },
]);
