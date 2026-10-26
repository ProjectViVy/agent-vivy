import { t } from '@/i18n';

export type FailureKind = 'control_plane' | 'api_key' | 'provider' | 'run' | 'generic';
export type FailureAction = 'retry' | 'open_model_settings' | 'none';

export interface ClassifiedFailure {
  kind: FailureKind;
  titleKey: 'errors.controlPlaneTitle' | 'errors.apiKeyTitle' | 'errors.providerTitle' | 'errors.runFailedTitle' | 'errors.genericTitle';
  /** 面向用户的一句话；机器噪声不会出现在这里。 */
  detail: string;
  action: FailureAction;
  /** 附加的本地化说明（比如「这不是后端连接问题」），不是原始错误文本。 */
  hintKey?: 'errors.runFailedHint' | 'errors.providerHint';
  /** 原始文本，仅在它是机器噪声时保留，供「详情 / 复制诊断」使用。 */
  technical?: string;
}

// 机器噪声的特征。只有命中这些标记的文本才折叠进详情；后端给出的人话
// （比如「模型服务触发限流」）继续作为主文案。
const TECHNICAL_MARKERS = [
  'rpc error',
  'code =',
  'code:',
  'grpc',
  '{"',
  'http://',
  'https://',
  'failed to fetch',
  'typeerror',
  'unexpected token',
  'econnrefused',
  'err_connection',
  'networkerror',
  'load failed',
  'invalid json',
  '\n    at ',
] as const;

export function looksTechnical(message: string): boolean {
  const normalized = message.toLowerCase();
  return TECHNICAL_MARKERS.some((marker) => normalized.includes(marker));
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

// 只认「缺失/被拒」这类机器侧结论；表单自己的「请输入 API key」属于本地
// 指引，不能被改写成通用的密钥缺失文案。
const API_KEY_MARKERS = [
  'api key missing',
  'api key rejected',
  'api key invalid',
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

// 后端按 HTTP 状态细分的 model 服务失败文案（已是用户可读中文）。
const STATUS_PROVIDER_MARKERS = [
  '模型服务暂时不可用',
  '模型服务拒绝',
  '模型服务触发限流',
] as const;

export function classifyFailure(error: unknown): ClassifiedFailure {
  return classifyFailureMessage(messageOf(error));
}

export function classifyFailureMessage(raw: string): ClassifiedFailure {
  const message = raw.trim();
  const normalized = message.toLowerCase();
  const technical = looksTechnical(message) ? message : undefined;

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
      technical,
    };
  }

  if (includesAny(message, STATUS_PROVIDER_MARKERS)) {
    return {
      kind: 'provider',
      titleKey: 'errors.providerTitle',
      detail: message,
      action: 'none',
    };
  }

  if (includesAny(normalized, PROVIDER_MARKERS)) {
    return {
      kind: 'provider',
      titleKey: 'errors.providerTitle',
      detail: t('errors.providerDetail'),
      action: 'none',
      hintKey: 'errors.providerHint',
      technical,
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
    detail: technical ? t('feedback.unknownDetail') : message,
    action: 'none',
    hintKey: 'errors.runFailedHint',
    technical,
  };
}

export function runFailedMessage(payload: Record<string, unknown> | undefined): string | null {
  const message = payload?.message;
  if (typeof message === 'string' && message.trim()) return message.trim();
  if (payload?.cause_category === 'provider_error') return t('errors.providerDetail');
  return null;
}
