import { expect, test } from '@playwright/test';

// 设置 → 通用 → 高级特性：生成参数按模型独立编辑（模型下拉选择，仅对选中模型生效）。
// - 独立「生成参数」卡（CardTitle 标题）已不存在；
// - 模型下拉 = 已选模型列表；切换模型 = 载入该模型独立的演示参数；
// - vivy.demo.gen-params 按模型三元组键（provider/baseUrl/model）独立保存，互不覆盖。
// 默认运行束是 DeepSeek（一等运行束，baseUrl 空 = 内置地址）。
test('settings general tab edits generation params per selected model', async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('vivy.ui.welcome.completed', '1');
    localStorage.setItem('vivy.ui.savedModels', JSON.stringify([
      { provider: 'deepseek', baseUrl: '', model: 'deepseek-flash' },
      { provider: 'deepseek', baseUrl: '', model: 'deepseek-reasoner' },
    ]));
  });
  await page.goto('/settings');

  // 通用 Tab 出现「高级特性」卡；不存在独立卡片标题级的「生成参数」heading。
  await expect(page.getByText('高级特性', { exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: '生成参数' })).toHaveCount(0);

  // 默认选中第一个模型（deepseek-flash），载入默认演示参数（用 id 避开通用预览的「最大 tokens」输入）。
  const maxTokens = page.locator('#gen-params-max-tokens');
  await expect(maxTokens).toHaveValue('4096');
  await expect(page.getByText('正在编辑 deepseek-flash 的生成参数。', { exact: true })).toBeVisible();
  await expect(page.getByText('0.7', { exact: true })).toBeVisible();

  // 保存 deepseek-flash 的参数：写入 vivy.demo.gen-params（模型三元组键）。
  await maxTokens.fill('8192');
  await page.getByRole('button', { name: '保存演示参数' }).click();
  await expect(page.getByText('已保存到本地', { exact: true })).toBeVisible();
  const keyFlash = 'deepseek//deepseek-flash';
  let store = await page.evaluate(() => JSON.parse(localStorage.getItem('vivy.demo.gen-params') ?? '{}'));
  expect(store[keyFlash]).toEqual({ temperature: 0.7, max_tokens: 8192 });

  // 切换到 deepseek-reasoner：载入该模型独立参数（默认 4096），deepseek-flash 的键不受影响。
  await page.getByRole('combobox', { name: '模型' }).click();
  await page.getByRole('option').filter({ hasText: /deepseek-reasoner$/ }).click();
  await expect(page.getByText('正在编辑 deepseek-reasoner 的生成参数。', { exact: true })).toBeVisible();
  await expect(maxTokens).toHaveValue('4096');
  store = await page.evaluate(() => JSON.parse(localStorage.getItem('vivy.demo.gen-params') ?? '{}'));
  expect(store[keyFlash]).toEqual({ temperature: 0.7, max_tokens: 8192 });
  expect(store['deepseek//deepseek-reasoner']).toBeUndefined();

  // deepseek-reasoner 保存自己的参数，两个模型互不影响。
  await maxTokens.fill('1024');
  await page.getByRole('button', { name: '保存演示参数' }).click();
  await expect(page.getByText('已保存到本地', { exact: true })).toBeVisible();
  store = await page.evaluate(() => JSON.parse(localStorage.getItem('vivy.demo.gen-params') ?? '{}'));
  expect(store['deepseek//deepseek-reasoner']).toEqual({ temperature: 0.7, max_tokens: 1024 });
  expect(store[keyFlash]).toEqual({ temperature: 0.7, max_tokens: 8192 });

  // 刷新后保留：默认仍选中第一个模型（deepseek-flash），读取其已保存的 8192。
  await page.reload();
  await expect(page.getByText('高级特性', { exact: true })).toBeVisible();
  await expect(maxTokens).toHaveValue('8192');
  expect(await page.evaluate(() => Object.keys(localStorage).filter((key) => key.startsWith('vivy.demo.'))))
    .toContain('vivy.demo.gen-params');
});