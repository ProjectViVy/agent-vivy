import { useEffect, useRef, useState } from 'react';
import { ChevronUp, Folder, HardDrive } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
	Dialog, DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader,
	DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { browseWorkspace, type WorkspaceBrowseResult } from '@/lib/api';
import { useTranslation } from '@/i18n';

interface Props {
	/** Controlled visibility; the owner opens it from its own trigger. */
	open: boolean;
	onOpenChange: (open: boolean) => void;
	/** Folder the dialog opens at and re-browses from when the owner switches it. */
	startPath: string;
	/**
	 * Confirming this folder is a no-op (it is already the owner's folder).
	 * Omit when the pick itself is the action, so re-picking it still reports.
	 */
	unchangedPath?: string;
	title: string;
	description: string;
	/** Offer the "use default workspace" escape next to the confirmation. */
	allowDefault?: boolean;
	onPick: (path: string) => Promise<unknown> | unknown;
}

/**
 * Single folder-picking surface: browse the host, then confirm one directory.
 * All browsing state is local; every owner decision (what the folder means)
 * stays with `onPick`.
 */
export function WorkspaceFolderDialog({
	open, onOpenChange, startPath, unchangedPath, title, description, allowDefault, onPick,
}: Props) {
	const { t } = useTranslation();
	const [path, setPath] = useState(startPath);
	const [browser, setBrowser] = useState<WorkspaceBrowseResult | null>(null);
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const browseEpoch = useRef(0);

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

	// Opening, and every owner-side target switch while open, restarts the flow:
	// a confirmed folder never survives the switch it was confirmed against.
	useEffect(() => {
		browseEpoch.current += 1;
		setPath(startPath);
		setBrowser(null);
		setLoading(false);
		setError(null);
		if (open) void browse(startPath);
		// `browse` is recreated per render; the epoch guard already rejects stale
		// results, so the flow is keyed on the owner's target and visibility only.
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [open, startPath]);

	const choose = async (nextPath: string) => {
		browseEpoch.current += 1;
		if (unchangedPath !== undefined && nextPath === unchangedPath) {
			setLoading(false);
			onOpenChange(false);
			return;
		}
		setLoading(true);
		setError(null);
		try {
			await onPick(nextPath);
			onOpenChange(false);
		} catch {
			setError(t('workspace.selectError'));
		} finally {
			setLoading(false);
		}
	};

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="h-[min(36rem,calc(100dvh-2rem))]">
				<DialogHeader>
					<DialogTitle>{title}</DialogTitle>
					<DialogDescription>{description}</DialogDescription>
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
					{allowDefault ? <Button type="button" variant="outline" disabled={loading} onClick={() => void choose('')}>{t('workspace.useDefault')}</Button> : null}
					<Button type="button" disabled={loading || !browser || browser.path !== path} onClick={() => void choose(browser?.path ?? '')}>{t('workspace.useFolder')}</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
