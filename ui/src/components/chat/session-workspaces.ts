import type { Session } from '@/lib/api';

export interface SessionWorkspaceGroup {
	key: string;
	label: string;
	sessions: Session[];
}

export function workspaceDisplayName(path: string, defaultLabel = 'Default workspace'): string {
	if (/^[\\/]+$/.test(path) || /^[A-Za-z]:[\\/]*$/.test(path)) return path;
	const normalized = path.replace(/[\\/]+$/, '');
	if (!normalized) return defaultLabel;
	const parts = normalized.split(/[\\/]/);
	return parts[parts.length - 1] || normalized;
}

export function groupSessionsByWorkspace(
	sessions: Session[],
	defaultLabel = 'Default workspace',
): SessionWorkspaceGroup[] {
	const groups = new Map<string, Session[]>();
	for (const session of sessions) {
		const path = session.workspace_path ?? '';
		groups.set(path, [...(groups.get(path) ?? []), session]);
	}
	return [...groups.entries()]
		.map(([path, items]) => ({ key: path, label: workspaceDisplayName(path, defaultLabel), sessions: items }))
		.sort((a, b) => {
			if (!a.key) return -1;
			if (!b.key) return 1;
			return a.label.localeCompare(b.label) || a.key.localeCompare(b.key);
		});
}
