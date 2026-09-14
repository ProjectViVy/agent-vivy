import { useEffect, useRef, useState } from 'react';
import { ChevronUp, Folder, FolderOpen, HardDrive } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
	Dialog, DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader,
	DialogTitle, DialogTrigger,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { browseWorkspace, type WorkspaceBrowseResult } from '@/lib/api';
import { useTranslation } from '@/i18n';
import { workspaceDisplayName } from './session-workspaces';

interface Props {
	workspacePath: string;
	disabled?: boolean;
	onSelect: (path: string) => Promise<unknown> | unknown;
}

export function WorkspaceSelector({ workspacePath, disabled, onSelect }: Props) {
	const { t } = useTranslation();
	const [open, setOpen] = useState(false);
	const [path, setPath] = useState(workspacePath);
	const [browser, setBrowser] = useState<WorkspaceBrowseResult | null>(null);
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const browseEpoch = useRef(0);
	const label = workspaceDisplayName(workspacePath, t('workspace.default'));

	const browse = async (nextPath: string) => {
		const epoch = ++browseEpoch.current;
		setLoading(true);
		setError(null);
		try {
			const result = await browseWorkspace(nextPath);
			if (epoch !== browseEpoch.current) return;
			setBrowser(result);
			setPath(result.path);
		} catch {
			if (epoch !== browseEpoch.current) return;
			setError(t('workspace.browseError'));
		} finally {
			if (epoch === browseEpoch.current) setLoading(false);
		}
	};

	useEffect(() => {
		browseEpoch.current += 1;
		setPath(workspacePath);
		setBrowser(null);
		setLoading(false);
		setError(null);
		if (open) void browse(workspacePath);
		// workspacePath is the authority switch; open is intentionally read as
		// current state without making ordinary dialog toggles repeat this reset.
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [workspacePath]);

	const setDialogOpen = (nextOpen: boolean) => {
		setOpen(nextOpen);
		if (nextOpen) {
			browseEpoch.current += 1;
			setPath(workspacePath);
			setBrowser(null);
			setLoading(false);
			setError(null);
			void browse(workspacePath);
		} else {
			browseEpoch.current += 1;
			setLoading(false);
		}
	};

	const choose = async (nextPath: string) => {
		browseEpoch.current += 1;
		if (nextPath === workspacePath) {
			setLoading(false);
			setOpen(false);
			return;
		}
		setLoading(true);
		setError(null);
		try {
			await onSelect(nextPath);
			setOpen(false);
		} catch {
			setError(t('workspace.selectError'));
		} finally {
			setLoading(false);
		}
	};

	return (
		<Dialog open={open} onOpenChange={setDialogOpen}>
			<DialogTrigger asChild>
				<button
					type="button"
					disabled={disabled}
					title={workspacePath || t('workspace.default')}
					className="flex h-7 min-w-0 max-w-48 items-center gap-1.5 rounded-md px-2 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground disabled:pointer-events-none disabled:opacity-50"
				>
					<FolderOpen className="h-3.5 w-3.5 shrink-0" />
					<span className="truncate">{label}</span>
				</button>
			</DialogTrigger>
			<DialogContent className="h-[min(36rem,calc(100dvh-2rem))]">
				<DialogHeader>
					<DialogTitle>{t('workspace.pickerTitle')}</DialogTitle>
					<DialogDescription>{t('workspace.pickerDescription')}</DialogDescription>
				</DialogHeader>
				<div className="flex gap-2">
					<Input aria-label={t('workspace.path')} value={path} onChange={(event) => {
						browseEpoch.current += 1;
						setPath(event.target.value);
						setBrowser(null);
						setLoading(false);
						setError(null);
					}} onKeyDown={(event) => { if (event.key === 'Enter') void browse(path); }} />
					<Button type="button" variant="outline" disabled={loading || !path.trim()} onClick={() => void browse(path)}>{t('workspace.browse')}</Button>
				</div>
				{browser?.roots.length ? <div className="flex flex-wrap gap-1">
					{browser.roots.map((root) => <Button key={root} type="button" size="sm" variant="ghost" disabled={loading} onClick={() => void browse(root)}><HardDrive className="h-3.5 w-3.5" />{root}</Button>)}
				</div> : null}
				<DialogBody className="rounded-lg border">
					<ScrollArea className="h-full p-2">
						{browser?.parent ? <button type="button" disabled={loading} onClick={() => void browse(browser.parent!)} className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm hover:bg-muted disabled:pointer-events-none disabled:opacity-50"><ChevronUp className="h-4 w-4" /><span>{t('workspace.parent')}</span></button> : null}
						{browser?.directories.map((directory) => <button key={directory.path} type="button" title={directory.path} disabled={loading} onClick={() => void browse(directory.path)} className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm hover:bg-muted disabled:pointer-events-none disabled:opacity-50"><Folder className="h-4 w-4 text-primary" /><span className="truncate">{directory.name}</span></button>)}
						{loading ? <div className="px-3 py-8 text-center text-sm text-muted-foreground">{t('workspace.loading')}</div> : null}
						{!loading && browser && browser.directories.length === 0 ? <div className="px-3 py-8 text-center text-sm text-muted-foreground">{t('workspace.empty')}</div> : null}
						{!loading && browser?.truncated ? <div className="px-3 py-2 text-xs text-muted-foreground" role="status">{t('workspace.truncated')}</div> : null}
						{error ? <div className="px-3 py-3 text-sm text-destructive" role="alert">{error}</div> : null}
					</ScrollArea>
				</DialogBody>
				<DialogFooter>
					<Button type="button" variant="outline" disabled={loading} onClick={() => void choose('')}>{t('workspace.useDefault')}</Button>
					<Button type="button" disabled={loading || !browser || browser.path !== path} onClick={() => void choose(browser?.path ?? '')}>{t('workspace.useFolder')}</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
