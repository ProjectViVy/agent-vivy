import { expect, test, type Page } from '@playwright/test';

// UI-COMPOSER / UI-CHAT-TOOLBAR 门控与伪操作清理：
// 1. 思考强度 D9 能力门控：思考按钮常驻在「智能」旁。能力不支持时必须置灰禁用、
//    点不开菜单并给出原因（不再整体隐藏）；能力支持时必须可选，菜单只列后端声明的
//    级别面（声明了扩展思考但未声明级别的模型用七级顺序），附件等其余按钮不受影响。
// 2. 伪操作清理：AutoDream 图标彻底移除；执行模式下拉仅保留真实支持的智能体与
//    计划模式，询问模式彻底隐退。
//
// 能力状态由活动模型决定，这里走产品自己的模型选择路径
// （Settings → 模型，与 model-refresh.spec.ts 相同）把活动模型固定成两种：
// deepseek-flash 声明 supports_thinking、deepseek-chat 未声明，两种状态都真实
// 走过 session/context 投影，不依赖 CI 机器的环境变量。
//
// 每次 settings/update 会用后端确认的语言重刷界面，而首次加载用的是请求语言，
// 因此本 spec 的界面文案匹配两种语言。

/** 全新状态会先弹欢迎向导 / 人格引导，跳过它们再操作页面。 */
async function dismissOnboarding(page: Page): Promise<void> {
  const welcome = page.getByRole('dialog', { name: /欢迎使用 Vivy|Welcome to Vivy/ });
  if (await welcome.waitFor({ state: 'visible', timeout: 5_000 }).then(() => true).catch(() => false)) {
    await welcome.getByRole('button', { name: /跳过向导|Skip/ }).click();
    await expect(welcome).toBeHidden();
  }
  const persona = page.getByRole('dialog', { name: /人格初始化引导|Persona onboarding/ });
  if (await persona.waitFor({ state: 'visible', timeout: 5_000 }).then(() => true).catch(() => false)) {
    await persona.getByRole('button', { name: /稍后再说|Not now/ }).click();
    await expect(persona).toBeHidden();
  }
}

/** 通过 Settings → 模型 选中 DeepSeek 端点上的一个模型（真实 UI 路径）。 */
async function selectModel(page: Page, model: string): Promise<void> {
  await page.goto('/settings?tab=model');
  await dismissOnboarding(page);
  await page.locator('[data-testid="provider-row-deepseek-openai-completions"]').click();
  const row = page.locator(`[data-testid="provider-model-${model}"]`);
  await row.click();
  await expect(row).toHaveAttribute('aria-pressed', 'true');
}

/** 回到聊天页并保证聊天输入框已就绪。 */
async function openChat(page: Page): Promise<void> {
  await page.goto('/');
  await dismissOnboarding(page);
  await expect(page.getByRole('combobox', { name: /消息|Message/ })).toBeVisible({ timeout: 30_000 });
}

const thinkingButton = (page: Page) => page.getByRole('button', { name: /思考模式|Thinking mode/ });

test('thinking control stays visible but disabled without a thinking-capable model, and fake controls are removed', async ({ page }) => {
  test.setTimeout(180_000);
  await page.addInitScript(() => localStorage.setItem('vivy.ui.welcome.completed', '1'));

  // 未声明扩展思考的模型：按钮不消失，但置灰、不可改、点不开菜单，并给出原因。
  await selectModel(page, 'deepseek-chat');
  await openChat(page);
  const greyed = thinkingButton(page);
  await expect(greyed).toBeVisible();
  await expect(greyed).toBeDisabled();
  await expect(greyed).toHaveAttribute(
    'title',
    /当前模型未声明扩展思考支持|The active model does not declare extended-thinking support/,
  );
  await greyed.click({ force: true });
  await expect(page.getByRole('menu')).toHaveCount(0);

  // 声明扩展思考的模型：同一个按钮变为可用，菜单给出后端声明的级别面。
  await selectModel(page, 'deepseek-flash');
  await openChat(page);
  const thinking = thinkingButton(page);
  await expect(thinking).toBeVisible();
  await expect(thinking).toBeEnabled();
  await thinking.click();
  await expect(page.getByRole('menuitem', { name: /跟随默认|Follow default/ })).toBeVisible();
  await expect(page.getByRole('menuitem', { name: 'medium' })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('menu')).toHaveCount(0);

  await expect(page.getByLabel(/附件|Attachment/)).toBeVisible();

  // 伪操作清理：AutoDream 图标彻底移除；执行模式里被删掉的「询问模式」不再出现。
  await expect(page.getByTitle(/AutoDream/)).toHaveCount(0);
  await expect(page.getByLabel(/AutoDream/)).toHaveCount(0);
  await expect(page.getByRole('button', { name: /询问模式|Ask mode/ })).toHaveCount(0);
  await expect(page.getByRole('menuitem', { name: /询问模式|Ask mode/ })).toHaveCount(0);
});
