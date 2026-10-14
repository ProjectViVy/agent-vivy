// 首次引导的本地记忆：用户选择「稍后再说」后，本机不再自动弹出面板。
// 人格状态本身仍由后端权威回答；这里只是一个本机礼貌标记，不代表人格状态。
const STORAGE_KEY = 'vivy.persona.onboarding.dismissed';

export function isPersonaOnboardingDismissed(): boolean {
  try {
    return window.localStorage.getItem(STORAGE_KEY) === '1';
  } catch {
    return false;
  }
}

export function dismissPersonaOnboarding(): void {
  try {
    window.localStorage.setItem(STORAGE_KEY, '1');
  } catch {
    /* storage 不可用（隐私模式）时本次会话仍保持关闭 */
  }
}

/** 仅供测试：清除本机标记。 */
export function resetPersonaOnboardingDismissalForTests(): void {
  try {
    window.localStorage.removeItem(STORAGE_KEY);
  } catch {
    /* nothing to clear */
  }
}
