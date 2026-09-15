// @vitest-environment happy-dom
import React from 'react';
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { renderToStaticMarkup } from 'react-dom/server';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { hydrateLocale } from '@/i18n';
import * as api from '@/lib/api';
import { WorkspaceSelector } from './WorkspaceSelector';

function deferred<T>() {
	let resolve!: (value: T) => void;
	let reject!: (reason?: unknown) => void;
	const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
	return { promise, resolve, reject };
}

function browseResult(path: string): api.WorkspaceBrowseResult {
	return { path, roots: ['/'], directories: [], truncated: false };
}

describe('WorkspaceSelector', () => {
	it('shows the default workspace beside the context meter', () => {
		const html = renderToStaticMarkup(<WorkspaceSelector workspacePath="" onSelect={async () => undefined} />);
		expect(html).toContain('Default workspace');
	});

	it('shows the selected folder name and keeps the full path accessible', () => {
		const html = renderToStaticMarkup(<WorkspaceSelector workspacePath="/code/agent-vivy" onSelect={async () => undefined} />);
		expect(html).toContain('agent-vivy');
		expect(html).toContain('/code/agent-vivy');
	});
});

describe('WorkspaceSelector interactions', () => {
	let container: HTMLDivElement;
	let root: Root;

	beforeEach(() => {
		vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
		hydrateLocale('en');
		container = document.createElement('div');
		document.body.append(container);
		root = createRoot(container);
	});

	afterEach(async () => {
		await act(async () => root.unmount());
		container.remove();
		document.querySelectorAll('[role="dialog"]').forEach((node) => node.remove());
		vi.restoreAllMocks();
		vi.unstubAllGlobals();
	});

	function button(label: string): HTMLButtonElement {
		const match = [...document.querySelectorAll<HTMLButtonElement>('button')].find((node) => node.textContent === label);
		expect(match, `button: ${label}`).toBeDefined();
		return match!;
	}

	async function renderAndOpen(onSelect = vi.fn()) {
		await act(async () => root.render(<WorkspaceSelector workspacePath="/old" onSelect={onSelect} />));
		await act(async () => container.querySelector<HTMLButtonElement>('button')!.click());
		return onSelect;
	}

	async function enterPath(value: string) {
		const input = document.querySelector<HTMLInputElement>('input[aria-label="Folder path"]')!;
		await act(async () => {
			const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set;
			setter?.call(input, value);
			input.dispatchEvent(new Event('input', { bubbles: true }));
		});
		return input;
	}

	it('invalidates a browsed folder when the user types another path', async () => {
		vi.spyOn(api, 'browseWorkspace').mockResolvedValue(browseResult('/old'));
		const onSelect = await renderAndOpen();
		await act(async () => { await Promise.resolve(); });

		const input = await enterPath('/new');
		expect(input.value).toBe('/new');
		expect(button('Use this folder').disabled).toBe(true);
		await act(async () => button('Use this folder').click());
		expect(onSelect).not.toHaveBeenCalled();
	});

	it('ignores a stale browse response that resolves after a newer request', async () => {
		const oldBrowse = deferred<api.WorkspaceBrowseResult>();
		const newBrowse = deferred<api.WorkspaceBrowseResult>();
		vi.spyOn(api, 'browseWorkspace').mockReturnValueOnce(oldBrowse.promise).mockReturnValueOnce(newBrowse.promise);
		await renderAndOpen();
		const input = await enterPath('/new');
		await act(async () => button('Browse').click());

		await act(async () => newBrowse.resolve(browseResult('/new')));
		expect(input.value).toBe('/new');
		expect(button('Use this folder').disabled).toBe(false);
		await act(async () => oldBrowse.resolve(browseResult('/old')));
		expect(input.value).toBe('/new');
		expect(button('Use this folder').disabled).toBe(false);
	});

	it('invalidates the confirmed folder when the active session changes', async () => {
		vi.spyOn(api, 'browseWorkspace')
			.mockResolvedValueOnce(browseResult('/old'))
			.mockRejectedValueOnce(new Error('new workspace unavailable'));
		const onSelect = await renderAndOpen();
		await act(async () => { await Promise.resolve(); });
		expect(button('Use this folder').disabled).toBe(false);

		await act(async () => root.render(<WorkspaceSelector workspacePath="/new" onSelect={onSelect} />));
		await act(async () => { await Promise.resolve(); });

		expect(document.querySelector<HTMLInputElement>('input[aria-label="Folder path"]')?.value).toBe('/new');
		expect(button('Use this folder').disabled).toBe(true);
		expect(document.querySelector('[role="alert"]')?.textContent).toBe('Could not browse this folder.');
	});

	it('closes without mutation when the selected folder is confirmed again', async () => {
		vi.spyOn(api, 'browseWorkspace').mockResolvedValue(browseResult('/old'));
		const onSelect = await renderAndOpen();
		await act(async () => { await Promise.resolve(); });
		await act(async () => button('Use this folder').click());

		expect(onSelect).not.toHaveBeenCalled();
		expect(document.querySelector('[role="dialog"]')).toBeNull();
	});

	it('shows a localized browse failure without exposing backend text', async () => {
		vi.spyOn(api, 'browseWorkspace').mockRejectedValue(new Error('host path detail'));
		await renderAndOpen();
		await act(async () => { await Promise.resolve(); });

		expect(document.querySelector('[role="alert"]')?.textContent).toBe('Could not browse this folder.');
		expect(document.body.textContent).not.toContain('host path detail');
	});

	it('warns when a directory listing is truncated', async () => {
		vi.spyOn(api, 'browseWorkspace').mockResolvedValue({ ...browseResult('/old'), truncated: true });
		await renderAndOpen();
		await act(async () => { await Promise.resolve(); });

		expect(document.querySelector('[role="status"]')?.textContent).toContain('Only the first folders are shown');
	});
});
