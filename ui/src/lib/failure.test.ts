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
});

describe('runFailedMessage', () => {
  it('reads the structured run.failed payload and ignores empty values', () => {
    expect(runFailedMessage({ cause_category: 'provider_error', message: '  key missing  ' })).toBe('Unable to connect! Check your provider configuration!');
    expect(runFailedMessage({ cause_category: 'internal_error' })).toBeNull();
    expect(runFailedMessage(undefined)).toBeNull();
  });
});
