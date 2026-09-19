import { useState } from 'react';
import { FolderOpen } from 'lucide-react';
import { useTranslation } from '@/i18n';
import { workspaceDisplayName } from './session-workspaces';
import { WorkspaceFolderDialog } from './WorkspaceFolderDialog';

interface Props {
	workspacePath: string;
	disabled?: boolean;
	onSelect: (path: string) => Promise<unknown> | unknown;
}

/** Chat-side trigger: shows the active session's folder and switches it. */
export function WorkspaceSelector({ workspacePath, disabled, onSelect }: Props) {
	const { t } = useTranslation();
	const [open, setOpen] = useState(false);
	const label = workspaceDisplayName(workspacePath, t('workspace.default'));

	return (
		<>
			<button
				type="button"
				disabled={disabled}
				title={workspacePath || t('workspace.default')}
				onClick={() => setOpen(true)}
				className="flex h-7 min-w-0 max-w-48 items-center gap-1.5 rounded-md px-2 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground disabled:pointer-events-none disabled:opacity-50"
			>
				<FolderOpen className="h-3.5 w-3.5 shrink-0" />
				<span className="truncate">{label}</span>
			</button>
			<WorkspaceFolderDialog
				open={open}
				onOpenChange={setOpen}
				startPath={workspacePath}
				unchangedPath={workspacePath}
				title={t('workspace.pickerTitle')}
				description={t('workspace.pickerDescription')}
				allowDefault
				onPick={onSelect}
			/>
		</>
	);
}
