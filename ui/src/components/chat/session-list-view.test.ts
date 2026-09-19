// @vitest-environment happy-dom
import { afterEach, describe, expect, it } from 'vitest';
import type { Session } from '@/lib/api';
import {
	SESSION_LIST_VIEW_KEY, SESSION_SEARCH_LIMIT,
	filterSessions, readSessionListView, storeSessionListView,
} from './session-list-view';

const labels = { defaultWorkspace: 'Default workspace', untitled: 'New session' };

function session(id: string, title: string, workspacePath?: string): Session {
	return { id, title, created_at: 1, ...(workspacePath === undefined ? {} : { workspace_path: workspacePath }) };
}

afterEach(() => localStorage.clear());

describe('filterSessions', () => {
	const sessions = [
		session('a', 'Alpha plan', '/code/agent-vivy'),
		session('b', 'Beta review', '/code/other-repo'),
		session('c', '', '/code/agent-vivy'),
	];

	it('returns nothing for a blank query instead of the whole list', () => {
		expect(filterSessions(sessions, '   ', labels)).toEqual([]);
	});

	it('matches titles case-insensitively', () => {
		expect(filterSessions(sessions, 'ALPHA', labels).map((item) => item.id)).toEqual(['a']);
	});

	it('matches the folder label of a session', () => {
		expect(filterSessions(sessions, 'other-repo', labels).map((item) => item.id)).toEqual(['b']);
	});

	it('matches an untitled session by its displayed placeholder', () => {
		expect(filterSessions(sessions, 'new session', labels).map((item) => item.id)).toEqual(['c']);
	});

	it('matches sessions without a workspace by the default label', () => {
		expect(filterSessions([session('d', 'Draft')], 'default', labels).map((item) => item.id)).toEqual(['d']);
	});

	it('caps the result list', () => {
		const many = Array.from({ length: SESSION_SEARCH_LIMIT + 5 }, (_, index) => session(`s${index}`, 'Match'));
		expect(filterSessions(many, 'match', labels)).toHaveLength(SESSION_SEARCH_LIMIT);
	});
});

describe('session list view persistence', () => {
	it('defaults to the grouped view', () => {
		expect(readSessionListView()).toBe('grouped');
	});

	it('round-trips the flat view', () => {
		storeSessionListView('flat');
		expect(localStorage.getItem(SESSION_LIST_VIEW_KEY)).toBe('flat');
		expect(readSessionListView()).toBe('flat');
	});

	it('falls back to the grouped view for an unknown stored value', () => {
		localStorage.setItem(SESSION_LIST_VIEW_KEY, 'grid');
		expect(readSessionListView()).toBe('grouped');
	});
});
