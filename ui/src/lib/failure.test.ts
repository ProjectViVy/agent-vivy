import { describe, expect, it } from 'vitest';
import { classifyFailure, classifyFailureMessage, runFailedMessage } from './failure';

describe('classifyFailure', () => {
  it('maps bootstrap / proxy / fetch failures to the control-plane recovery path', () => {
    expect(classifyFailureMessage('Failed to fetch').kind).toBe('control_plane');
    expect(classifyFailure(new Error('connect ECONNREFUSED 127.0.0.1:8787')).kind).toBe('control_plane');
    expect(classifyFailureMessage('无法连接 Vivy control plane：/rpc/bootstrap 返回了非 JSON 响应').action).toBe('retry');
    expect(classifyFailureMessage('Vivy control plane disconnected').titleKey).toBe('errors.controlPlaneTitle');
    expect(classifyFailureMessage('Failed to fetch').detail).toContain('控制面');
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
  });
});

describe('runFailedMessage', () => {
  it('reads the structured run.failed payload and ignores empty values', () => {
    expect(runFailedMessage({ cause_category: 'provider_error', message: '  key missing  ' })).toBe('key missing');
    expect(runFailedMessage({ cause_category: 'internal_error' })).toBeNull();
    expect(runFailedMessage(undefined)).toBeNull();
  });
});
