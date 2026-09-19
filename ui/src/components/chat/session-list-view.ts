import type { Session } from '@/lib/api';
import { workspaceDisplayName } from './session-workspaces';

/** Browser-local session-list view mode (a display preference, not session truth). */
export const SESSION_LIST_VIEW_KEY = 'vivy.ui.sessionListView';
/** Metadata matches shown at once; the list is a navigation aid, not a report. */
export const SESSION_SEARCH_LIMIT = 20;
export type SessionListView = 'grouped' | 'flat';

export function readSessionListView(): SessionListView {
	try { return localStorage.getItem(SESSION_LIST_VIEW_KEY) === 'flat' ? 'flat' : 'grouped'; } catch { return 'grouped'; }
}

export function storeSessionListView(view: SessionListView): void {
	try { localStorage.setItem(SESSION_LIST_VIEW_KEY, view); } catch { /* view persistence is best effort */ }
}

interface Labels {
	/** Folder label for sessions without a workspace. */
	defaultWorkspace: string;
	/** Row label for a session that has no title yet. */
	untitled: string;
}

/**
 * Metadata search over the session list: title, folder label or the untitled
 * placeholder. There is no content index, so a query never claims to search
 * transcript text.
 */
export function filterSessions(sessions: Session[], query: string, labels: Labels): Session[] {
	const needle = query.trim().toLowerCase();
	if (!needle) return [];
	return sessions
		.filter((session) => {
			const title = session.title || labels.untitled;
			const folder = workspaceDisplayName(session.workspace_path ?? '', labels.defaultWorkspace);
			return title.toLowerCase().includes(needle) || folder.toLowerCase().includes(needle);
		})
		.slice(0, SESSION_SEARCH_LIMIT);
}
