import { expect, test } from '@playwright/test';
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';

test('opens Toolkits with the keyboard and fits a narrow viewport', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
  await page.goto('/');

  const openNavigation = page.getByRole('button', { name: /open navigation/i });
  await expect(openNavigation).toBeVisible();
  await openNavigation.focus();
  await page.keyboard.press('Enter');

  const vivyNavigation = page.getByRole('button', { name: 'VIVY', exact: true });
  await expect(vivyNavigation).toBeVisible();
  await vivyNavigation.focus();
  await page.keyboard.press('Enter');

  const toolkitsLink = page.getByRole('link', { name: 'Toolkits', exact: true });
  await expect(toolkitsLink).toBeVisible();
  await toolkitsLink.focus();
  await page.keyboard.press('Enter');

  const toolkitPage = page.getByTestId('toolkit-page');
  await expect(toolkitPage).toBeVisible();
  await expect(toolkitPage.getByRole('heading', { name: 'Toolkits', exact: true })).toBeVisible();

  const dimensions = await page.evaluate(() => ({
    viewport: document.documentElement.clientWidth,
    document: document.documentElement.scrollWidth,
  }));
  expect(dimensions.document).toBeLessThanOrEqual(dimensions.viewport);
});

test('reads a populated Toolkit on mobile and keeps long content inside its keyboard-dismissible dialog', async ({ page }) => {
  const fixtureRoot = mkdtempSync(path.join(tmpdir(), 'vivy-toolkit-mobile-content-'));
  const source = path.join(fixtureRoot, 'native-toolkit');
  const skillPath = path.join(source, 'skills', 'long-content', 'SKILL.md');
  const longToken = 'x'.repeat(1800);
  mkdirSync(path.dirname(skillPath), { recursive: true });
  mkdirSync(path.join(source, 'masks'), { recursive: true });
  writeFileSync(skillPath, `---\ndescription: Mobile content check\n---\nDETAIL-START\n${longToken}\nDETAIL-END\n`, 'utf8');
  writeFileSync(path.join(source, 'masks', 'pm.md'), '# PM\nReview the user outcome first.\n', 'utf8');
  writeFileSync(path.join(source, 'vivy-bundle.json'), `${JSON.stringify({
    apiVersion: 'vivy.bundle/v1',
    id: 'test/mobile-content',
    version: '1.0.0',
    name: 'Mobile Content Check',
    components: {
      skills: [{ id: 'long-content', path: 'skills/long-content/SKILL.md' }],
      masks: [{ id: 'pm', name: 'PM', path: 'masks/pm.md', recommendedSkills: ['long-content'] }],
    },
  }, null, 2)}\n`, 'utf8');

  try {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));
    await page.goto('/toolkits');
    await expect(page.getByTestId('toolkit-page')).toBeVisible();

    const sourceInput = page.getByLabel('Host directory path');
    await sourceInput.fill(path.join(fixtureRoot, 'missing'));
    await page.getByRole('button', { name: 'Preview source', exact: true }).click();
    await expect(page.getByRole('alert')).toContainText('Preview failed');

    await sourceInput.fill(source);
    await page.getByRole('button', { name: 'Preview source', exact: true }).click();
    await expect(page.getByTestId('toolkit-preview')).toContainText('Mobile Content Check');
    await page.getByTestId('toolkit-preview').getByRole('button', { name: 'Install disabled', exact: true }).click();
    await expect(page.getByRole('status')).toContainText('Toolkit installed disabled');

    const skillSection = page.getByRole('heading', { name: 'Skills', exact: true }).locator('xpath=../..');
    await expect(skillSection).toContainText('long-content');
    const readContent = skillSection.getByRole('button', { name: 'View content', exact: true });
    await readContent.focus();
    await page.keyboard.press('Enter');

    const dialog = page.getByRole('dialog');
    await expect(dialog).toContainText('DETAIL-START');
    await expect(dialog).toContainText('DETAIL-END');
    const dimensions = await page.evaluate(() => {
      const dialogElement = document.querySelector<HTMLElement>('[role="dialog"]');
      const content = dialogElement?.querySelector<HTMLElement>('pre');
      const rect = dialogElement?.getBoundingClientRect();
      return {
        viewport: document.documentElement.clientWidth,
        document: document.documentElement.scrollWidth,
        dialogLeft: rect?.left ?? -1,
        dialogRight: rect?.right ?? Number.POSITIVE_INFINITY,
        contentWidth: content?.clientWidth ?? 0,
        contentScrollWidth: content?.scrollWidth ?? Number.POSITIVE_INFINITY,
      };
    });
    expect(dimensions.document).toBeLessThanOrEqual(dimensions.viewport);
    expect(dimensions.dialogLeft).toBeGreaterThanOrEqual(0);
    expect(dimensions.dialogRight).toBeLessThanOrEqual(dimensions.viewport);
    expect(dimensions.contentScrollWidth).toBeLessThanOrEqual(dimensions.contentWidth);

    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expect(readContent).toBeFocused();
  } finally {
    rmSync(fixtureRoot, { recursive: true, force: true });
  }
});
