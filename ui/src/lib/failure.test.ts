import { resetLocaleForTests } from '@/i18n';
import { beforeEach, describe, expect, it } from 'vitest';
import { classifyFailure, classifyFailureMessage, runFailedMessage } from './failure';

beforeEach(() => resetLocaleForTests());

describe('classifyFailure', () => {
  it('maps bootstrap / proxy / fetch failures to the control-plane recovery path', () => {
    expect(classifyFailureMessage('Failed to fetch').kind).toBe('control_plane');
    expect(classifyFailure(new Error('connect ECONNREFUSED 127.0.0.1:8787')).kind).toBe('control_plane');
    expect(classifyFailureMessage('无法连接 Vivy control plane：/rpc/bootstrap 返回了非 JSON 响应').action).toBe('retry');
    expect(classifyFailureMessage('Vivy control plane disconnected').titleKey).toBe('errors.controlPlaneTitle');
    expect(classifyFailureMessage('Failed to fetch').detail).toContain('The control plane did not respond.');
  });

  it('maps missing provider keys to model settings, not the raw engine dump', () => {
    const failure = classifyFailureMessage(
      'engine event error: [NodeRunError] provider openai: API key missing: configure it in the welcome wizard or Settings → Model\n------------------------\nnode path: [node_1, ChatModel]',
    );
    expect(failure.kind).toBe('api_key');
    expect(failure.action).toBe('open_model_settings');
    expect(failure.detail).not.toContain('NodeRunError');
    expect(failure.detail).not.toContain('node path');
  });

  it('keeps a run payload message as the recoverable detail', () => {
    const failure = classifyFailureMessage('The model run could not be completed. Please try again.');
    expect(failure.kind).toBe('run');
    expect(failure.detail).toBe('The model run could not be completed. Please try again.');
    expect(failure.action).toBe('none');
    expect(failure.hintKey).toBe('errors.runFailedHint');
  });

  it('maps backend provider failures to the model-service category, not a connection issue', () => {
    const failure = classifyFailureMessage('无法连接！请检查供应商配置！');
    expect(failure.kind).toBe('provider');
    expect(failure.titleKey).toBe('errors.providerTitle');
    expect(failure.hintKey).toBe('errors.providerHint');
    expect(failure.detail).toBe('Unable to connect! Check your provider configuration!');
  });

  it('does not mistake the provider wording for a control-plane outage', () => {
    expect(classifyFailureMessage('The model service could not be reached.').kind).toBe('provider');
    expect(classifyFailureMessage('Vivy control plane disconnected').kind).toBe('control_plane');
  });

  it('keeps the backend status split as the provider detail', () => {
    const rejected = classifyFailureMessage(
      '模型服务拒绝了本次请求（HTTP 400）。请新建会话后重试；若仍出现，请附运行日志反馈。',
    );
    expect(rejected.kind).toBe('provider');
    expect(rejected.detail).toContain('HTTP 400');
    expect(rejected.hintKey).toBeUndefined();

    expect(classifyFailureMessage('模型服务暂时不可用（HTTP 500），请稍后重试。').kind).toBe('provider');
    expect(classifyFailureMessage('模型服务触发限流（HTTP 429），请稍后重试。').detail).toContain('HTTP 429');
  });

  it('routes an HTTP 401 message to model settings', () => {
    const failure = classifyFailureMessage(
      'API key missing or rejected by the model provider (HTTP 401). Update it in Settings → Model.',
    );
    expect(failure.kind).toBe('api_key');
    expect(failure.action).toBe('open_model_settings');
  });
});

describe('runFailedMessage', () => {
  it('prefers the backend message and falls back to the provider detail', () => {
    expect(runFailedMessage({ cause_category: 'provider_error', message: '  key missing  ' })).toBe('key missing');
    expect(runFailedMessage({ cause_category: 'provider_error' })).toBe('Unable to connect! Check your provider configuration!');
    expect(runFailedMessage({ cause_category: 'provider_error', message: '   ' })).toBe('Unable to connect! Check your provider configuration!');
    expect(runFailedMessage({ cause_category: 'internal_error' })).toBeNull();
    expect(runFailedMessage(undefined)).toBeNull();
  });
});
