// @vitest-environment happy-dom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { Outlet, RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter } from '@tanstack/react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { hydrateLocale } from '@/i18n';
import * as api from '@/lib/api';
import type { Session } from '@/lib/api';
import { SESSION_LIST_VIEW_KEY } from './session-list-view';
import { ConversationSidebar } from './ConversationSidebar';

function session(id: string, title: string, workspacePath?: string): Session {
	return { id, title, created_at: 1, ...(workspacePath === undefined ? {} : { workspace_path: workspacePath }) };
}

const sessions = [
	session('a', 'Alpha plan', '/code/agent-vivy'),
	session('b', 'Beta review', '/code/other-repo'),
];

function browseResult(path: string): api.WorkspaceBrowseResult {
	return { path, roots: ['/'], directories: [], truncated: false };
}

function button(label: string): HTMLButtonElement {
	const match = [...document.querySelectorAll<HTMLButtonElement>('button')].find((node) => node.getAttribute('aria-label') === label);
	expect(match, `button: ${label}`).toBeDefined();
	return match!;
}

function hasButton(label: string): boolean {
	return [...document.querySelectorAll<HTMLButtonElement>('button')].some((node) => node.getAttribute('aria-label') === label);
}

describe('ConversationSidebar session list header', () => {
	let container: HTMLDivElement;
	let root: Root;
	let onChooseWorkspace: ReturnType<typeof vi.fn>;
	let onCreateSession: ReturnType<typeof vi.fn>;

	beforeEach(async () => {
		vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
		localStorage.clear();
		hydrateLocale('en');
		container = document.createElement('div');
		document.body.append(container);
		root = createRoot(container);
		onChooseWorkspace = vi.fn(async () => undefined);
		onCreateSession = vi.fn(async () => null);
		await render();
	});

	afterEach(async () => {
		await act(async () => root.unmount());
		container.remove();
		document.querySelectorAll('[role="dialog"]').forEach((node) => node.remove());
		vi.restoreAllMocks();
		vi.unstubAllGlobals();
		localStorage.clear();
	});

	async function render() {
		const rootRoute = createRootRoute({ component: () => <Outlet /> });
		const indexRoute = createRoute({
			getParentRoute: () => rootRoute,
			path: '/',
			component: () => (
				<ConversationSidebar
					sessions={sessions}
					activeSessionId="a"
					busyId={null}
					onSelectSession={() => undefined}
					onRenameSession={async () => undefined}
					onDeleteSession={async () => undefined}
					onCreateSession={onCreateSession}
					onChooseWorkspace={onChooseWorkspace}
				/>
			),
		});
		const router = createRouter({
			routeTree: rootRoute.addChildren([indexRoute]),
			history: createMemoryHistory({ initialEntries: ['/'] }),
		});
		await act(async () => { root.render(<RouterProvider router={router} />); });
		await act(async () => { await Promise.resolve(); });
	}

	it('offers search, view options and folder entry in the section header', () => {
		expect(hasButton('Search sessions')).toBe(true);
		expect(hasButton('View options')).toBe(true);
		expect(hasButton('Choose a folder and enter')).toBe(true);
		// The header no longer creates a session blind; the explicit new-session
		// entries stay in the sidebar body.
		expect(hasButton('New session')).toBe(true);
	});

	it('filters the list by session metadata and clears on cancel', async () => {
		expect(container.textContent).toContain('Beta review');
		await act(async () => button('Search sessions').click());
		const input = document.querySelector<HTMLInputElement>('input[aria-label="Search sessions"]')!;
		expect(input).not.toBeNull();
		await act(async () => {
			const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set;
			setter?.call(input, 'alpha');
			input.dispatchEvent(new Event('input', { bubbles: true }));
		});
		expect(container.textContent).toContain('Alpha plan');
		expect(container.textContent).not.toContain('Beta review');

		await act(async () => button('Cancel').click());
		expect(container.textContent).toContain('Beta review');
		expect(document.querySelector('input[aria-label="Search sessions"]')).toBeNull();
	});

	it('reports no match instead of an empty list', async () => {
		await act(async () => button('Search sessions').click());
		const input = document.querySelector<HTMLInputElement>('input[aria-label="Search sessions"]')!;
		await act(async () => {
			const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set;
			setter?.call(input, 'nothing-matches');
			input.dispatchEvent(new Event('input', { bubbles: true }));
		});
		expect(container.textContent).toContain('No matching sessions');
	});

	it('switches to the flat view and remembers it', async () => {
		await act(async () => button('View options').click());
		expect(document.querySelector('[role="menu"]')).not.toBeNull();
		const flat = [...document.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]')]
			.find((node) => node.textContent?.includes('Flat list'))!;
		await act(async () => flat.click());

		expect(localStorage.getItem(SESSION_LIST_VIEW_KEY)).toBe('flat');
		expect(document.querySelector('[role="menu"]')).toBeNull();
		expect(container.textContent).toContain('Alpha plan');
		expect(container.textContent).toContain('Beta review');
		// Folder group rows (and their per-folder new-session action) are gone.
		expect(hasButton('New session')).toBe(false);
		expect(container.textContent).not.toContain('agent-vivy');
		expect(container.textContent).not.toContain('other-repo');
	});

	it('enters the folder picked from the dialog instead of creating a blind session', async () => {
		vi.spyOn(api, 'browseWorkspace').mockResolvedValue(browseResult('/code/agent-vivy'));
		await act(async () => button('Choose a folder and enter').click());
		await act(async () => { await Promise.resolve(); });

		expect(document.querySelector('[role="dialog"]')?.textContent).toContain('Choose a folder');
		const confirm = [...document.querySelectorAll<HTMLButtonElement>('button')]
			.find((node) => node.textContent === 'Use this folder')!;
		await act(async () => confirm.click());

		expect(onChooseWorkspace).toHaveBeenCalledWith('/code/agent-vivy');
		expect(onCreateSession).not.toHaveBeenCalled();
		expect(document.querySelector('[role="dialog"]')).toBeNull();
	});

	it('keeps the dialog open with a localized error when the folder cannot be entered', async () => {
		vi.spyOn(api, 'browseWorkspace').mockResolvedValue(browseResult('/code/agent-vivy'));
		onChooseWorkspace.mockRejectedValueOnce(new Error('host detail'));
		await act(async () => button('Choose a folder and enter').click());
		await act(async () => { await Promise.resolve(); });
		const confirm = [...document.querySelectorAll<HTMLButtonElement>('button')]
			.find((node) => node.textContent === 'Use this folder')!;
		await act(async () => confirm.click());

		expect(document.querySelector('[role="dialog"]')).not.toBeNull();
		expect(document.querySelector('[role="alert"]')?.textContent).toBe('Could not select this workspace.');
		expect(document.body.textContent).not.toContain('host detail');
	});
});
