import { useSyncExternalStore } from 'react';

// 同伴面容单一入口：预设表情、本地照片、localStorage 持久化。
// 面容只是本机的呈现偏好，不是运行期事实：它不进入人格文档、提示词或
// Journal，也不会随账号同步。照片在浏览器内缩放后以 data URL 保存，
// 永远不会上传。
const STORAGE_KEY = 'vivy.companion.avatar';

export const COMPANION_EMOJI_PRESETS = [
  '😊', '🌙', '🫧', '🌱', '🕯️', '🐈', '🦋', '❄️', '🔆', '☕', '🎐', '🧭',
] as const;

/** Longest edge of the stored square photo, in CSS pixels. */
export const COMPANION_AVATAR_SIZE = 256;
/** Accepted size of the picked source file, before downscaling. */
export const COMPANION_PHOTO_MAX_BYTES = 5 * 1024 * 1024;
/** Budget for the stored data URL, so one photo cannot fill the origin quota. */
export const COMPANION_PHOTO_MAX_DATA_LENGTH = 120 * 1024;
export const COMPANION_PHOTO_MIMES = ['image/png', 'image/jpeg', 'image/webp'] as const;

export type CompanionAvatar =
  | { readonly kind: 'emoji'; readonly value: string }
  | { readonly kind: 'photo'; readonly value: string };

export const DEFAULT_COMPANION_AVATAR: CompanionAvatar = { kind: 'emoji', value: '😊' };

export function isCompanionEmoji(value: unknown): value is string {
  return typeof value === 'string' && (COMPANION_EMOJI_PRESETS as readonly string[]).includes(value);
}

/** A stored photo is a downscaled data URL of one accepted image mime. */
export function isCompanionPhoto(value: unknown): value is string {
  if (typeof value !== 'string' || value.length > COMPANION_PHOTO_MAX_DATA_LENGTH) return false;
  return COMPANION_PHOTO_MIMES.some((mime) => value.startsWith(`data:${mime};base64,`));
}

export function isCompanionAvatar(value: unknown): value is CompanionAvatar {
  if (!value || typeof value !== 'object') return false;
  const candidate = value as Partial<CompanionAvatar>;
  if (candidate.kind === 'emoji') return isCompanionEmoji(candidate.value);
  if (candidate.kind === 'photo') return isCompanionPhoto(candidate.value);
  return false;
}

export function readCompanionAvatar(): CompanionAvatar {
  try {
    const saved = window.localStorage.getItem(STORAGE_KEY);
    if (!saved) return DEFAULT_COMPANION_AVATAR;
    const parsed: unknown = JSON.parse(saved);
    return isCompanionAvatar(parsed) ? parsed : DEFAULT_COMPANION_AVATAR;
  } catch {
    return DEFAULT_COMPANION_AVATAR;
  }
}

let currentAvatar: CompanionAvatar = readCompanionAvatar();
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) listener();
}

/** 设置面容：持久化并通知订阅者。非法值被忽略，storage 不可用时仍生效。 */
export function setCompanionAvatar(value: CompanionAvatar): void {
  if (!isCompanionAvatar(value)) return;
  currentAvatar = value;
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(value));
  } catch {
    /* storage 不可用（隐私模式/配额）时本次会话仍使用新面容 */
  }
  emit();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function useCompanionAvatar(): {
  avatar: CompanionAvatar;
  presets: readonly string[];
  setAvatar: typeof setCompanionAvatar;
} {
  const avatar = useSyncExternalStore(subscribe, () => currentAvatar, () => DEFAULT_COMPANION_AVATAR);
  return { avatar, presets: COMPANION_EMOJI_PRESETS, setAvatar: setCompanionAvatar };
}

/** 仅供测试：重置模块内当前面容，不触碰 localStorage。 */
export function resetCompanionAvatarForTests(): void {
  currentAvatar = DEFAULT_COMPANION_AVATAR;
}
