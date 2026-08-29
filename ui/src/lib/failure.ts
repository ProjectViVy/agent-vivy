import { t } from '@/i18n';

export type FailureKind = 'control_plane' | 'api_key' | 'run' | 'generic';
export type FailureAction = 'retry' | 'open_model_settings' | 'none';

export interface ClassifiedFailure {
  kind: FailureKind;
  titleKey: 'errors.controlPlaneTitle' | 'errors.apiKeyTitle' | 'errors.runFailedTitle' | 'errors.genericTitle';
  detail: string;
  action: FailureAction;
}

function messageOf(error: unknown): string {
  if (error instanceof Error && error.message.trim()) return error.message.trim();
  const text = String(error ?? '').trim();
  return text || t('errors.genericTitle');
}

function includesAny(haystack: string, needles: readonly string[]): boolean {
  return needles.some((needle) => haystack.includes(needle));
}

const CONTROL_PLANE_MARKERS = [
  'control plane',
  'controlplaneurl',
  'runtime config',
  '/rpc/bootstrap',
  'econnrefused',
  'failed to fetch',
  'networkerror',
  'load failed',
  'err_connection',
  'disconnected',
  'websocket',
  '控制面',
] as const;

const API_KEY_MARKERS = [
  'api key missing',
  'api key is missing',
  'missing api key',
  '密钥未配置',
  '没有可用的 api key',
] as const;

export function classifyFailure(error: unknown): ClassifiedFailure {
  return classifyFailureMessage(messageOf(error));
}

export function classifyFailureMessage(raw: string): ClassifiedFailure {
  const message = raw.trim();
  const normalized = message.toLowerCase();

  if (includesAny(normalized, API_KEY_MARKERS)) {
    return {
      kind: 'api_key',
      titleKey: 'errors.apiKeyTitle',
      detail: t('errors.apiKeyMissing'),
      action: 'open_model_settings',
    };
  }

  if (includesAny(normalized, CONTROL_PLANE_MARKERS)) {
    return {
      kind: 'control_plane',
      titleKey: 'errors.controlPlaneTitle',
      detail: t('errors.controlPlaneUnreachable'),
      action: 'retry',
    };
  }

  if (!message) {
    return {
      kind: 'generic',
      titleKey: 'errors.genericTitle',
      detail: t('errors.genericTitle'),
      action: 'none',
    };
  }

  return {
    kind: 'run',
    titleKey: 'errors.runFailedTitle',
    detail: message,
    action: 'none',
  };
}

export function runFailedMessage(payload: Record<string, unknown> | undefined): string | null {
  const message = payload?.message;
  if (typeof message === 'string' && message.trim()) return message.trim();
  return null;
}
