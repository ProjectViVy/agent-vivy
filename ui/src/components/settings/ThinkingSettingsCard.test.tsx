// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { hydrateLocale } from '@/i18n';

// A non-thinking active model answers `levels: null` on an older backend; the
// card must render the unsupported state instead of crashing the app frame on
// `levels.length` (2026-10-15 settings crash).
const api = vi.hoisted(() => ({
  getThinking: vi.fn(),
  thinkingLevels: vi.fn(),
}));

vi.mock('@/lib/api', () => api);

const { ThinkingSettingsCard } = await import('./ThinkingSettingsCard');

const unsupportedText = 'The active model does not declare extended-thinking support.';

describe('ThinkingSettingsCard', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    hydrateLocale('en');
    api.getThinking.mockResolvedValue({
      thinking: 'auto',
      effective: 'auto',
      supported: null,
      supports_thinking: false,
      default_thinking: '',
      read_only: false,
    });
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  async function render() {
    await act(async () => root.render(<ThinkingSettingsCard />));
    await act(async () => {});
  }

  it('renders the unsupported state when the declared levels list is null', async () => {
    api.thinkingLevels.mockResolvedValue({ levels: null, default: '', supports_thinking: false });
    await render();
    expect(container.textContent).toContain(unsupportedText);
  });

  it('lists the declared levels plus the three aliases when the model supports thinking', async () => {
    api.getThinking.mockResolvedValue({
      thinking: 'auto',
      effective: 'high',
      supported: ['low', 'high'],
      supports_thinking: true,
      default_thinking: 'high',
      read_only: false,
    });
    api.thinkingLevels.mockResolvedValue({ levels: ['low', 'high'], default: 'high', supports_thinking: true });
    await render();
    expect(container.textContent).toContain('Effective for the active model: high');
    expect(container.textContent).not.toContain(unsupportedText);
  });
});
