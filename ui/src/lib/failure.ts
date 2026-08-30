import { t } from '@/i18n';

export type FailureKind = 'control_plane' | 'api_key' | 'provider' | 'run' | 'generic';
export type FailureAction = 'retry' | 'open_model_settings' | 'none';

export interface ClassifiedFailure {
  kind: FailureKind;
  titleKey: 'errors.controlPlaneTitle' | 'errors.apiKeyTitle' | 'errors.providerTitle' | 'errors.runFailedTitle' | 'errors.genericTitle';
  detail: string;
  action: FailureAction;
  /** 附加的本地化说明（比如「这不是后端连接问题」），不是原始错误文本。 */
  hintKey?: 'errors.runFailedHint' | 'errors.providerHint';
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

// 后端 run.failed 对 provider 失败给出的稳定文案前缀；「连接被拒/超时」等
// 传输细节本身属于前端控制面代理层，不能用来判 provider，避免误归类。
const PROVIDER_MARKERS = [
  'model service',
  'model provider',
  '无法连接！请检查供应商配置！',
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

  if (includesAny(normalized, PROVIDER_MARKERS)) {
    return {
      kind: 'provider',
      titleKey: 'errors.providerTitle',
      detail: t('errors.providerDetail'),
      action: 'none',
      hintKey: 'errors.providerHint',
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
    hintKey: 'errors.runFailedHint',
  };
}

export function runFailedMessage(payload: Record<string, unknown> | undefined): string | null {
  if (payload?.cause_category === 'provider_error') return t('errors.providerDetail');
  const message = payload?.message;
  if (typeof message === 'string' && message.trim()) return message.trim();
  return null;
}
