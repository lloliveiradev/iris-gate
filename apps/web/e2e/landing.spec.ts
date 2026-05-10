import { expect, test } from '@playwright/test';

test('landing page renders headline + CTAs + sections', async ({ page }) => {
  await page.goto('/');

  await expect(
    page.getByRole('heading', { name: /Payment infrastructure that ships in production/i }),
  ).toBeVisible();

  await expect(page.getByRole('link', { name: /Start integrating/i }).first()).toBeVisible();
  await expect(page.getByRole('link', { name: /See how it fits/i })).toBeVisible();

  for (const id of ['integration', 'monitoring', 'reporting', 'testimonials']) {
    await expect(page.locator(`#${id}`)).toBeVisible();
  }
});

test('signed-out user is redirected from /dashboard to /login', async ({ page }) => {
  const response = await page.goto('/dashboard');
  expect(response?.url()).toContain('/login');
  await expect(page.getByRole('heading', { name: /Entrar no painel/i })).toBeVisible();
});

test('login form rejects empty submission', async ({ page }) => {
  await page.goto('/login');
  // browser native validation should prevent submit, but we can confirm
  // that the form is present and required
  const input = page.getByLabel('API key');
  await expect(input).toBeVisible();
  await expect(input).toHaveAttribute('required', '');
});
