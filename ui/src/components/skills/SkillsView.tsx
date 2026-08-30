import { useCallback, useEffect, useState } from 'react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Switch } from '@/components/ui/switch';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { MasterDetail } from '@/components/layout/MasterDetail';
import { useTranslation } from '@/i18n';
import { useVivyStore } from '@/lib/store';
import { BookOpen, RefreshCw, Zap } from 'lucide-react';
import * as api from '@/lib/api';
import { MarketplaceTab } from './MarketplaceTab';

export function SkillsView() {
  const { t } = useTranslation();
  const capabilities = useVivyStore((state) => state.capabilities);
  const marketplaceEnabled = capabilities.includes('skills.marketplace');

  const [skills, setSkills] = useState<api.SkillSummary[]>([]);
  const [selected, setSelected] = useState<api.SkillView | null>(null);
  const [revisions, setRevisions] = useState<api.SkillRevision[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [toggling, setToggling] = useState(false);

  const loadSkills = useCallback(async () => {
    try {
      setError(null);
      const data = await api.listSkills();
      setSkills(data.skills);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('skills.errors.loadSkillsFailed'));
    }
  }, [t]);

  const loadRevisions = useCallback(async () => {
    try {
      const data = await api.listSkillRevisions();
      setRevisions(data.revisions);
    } catch {
      // Staged revisions are a secondary surface; the installed list stays usable.
    }
  }, []);

  useEffect(() => {
    void Promise.all([loadSkills(), loadRevisions()]).finally(() => setIsLoading(false));
  }, [loadSkills, loadRevisions]);

  const openSkill = useCallback(async (name: string, path?: string) => {
    try {
      setError(null);
      setSelected(await api.getSkill(name, path));
    } catch (err) {
      setError(err instanceof Error ? err.message : t('skills.errors.loadDocumentFailed'));
    }
  }, [t]);

  const toggleSkill = useCallback(async (summary: api.SkillSummary) => {
    try {
      setToggling(true);
      setError(null);
      const updated = await api.setSkillEnabled(summary.name, !summary.enabled, summary.hash);
      setSkills((prev) => prev.map((skill) => (skill.name === updated.name ? updated : skill)));
      setSelected((prev) =>
        prev && prev.name === updated.name
          ? { ...updated, content: prev.content, relative_path: prev.relative_path, supporting_files: prev.supporting_files }
          : prev,
      );
    } catch (err) {
      // A 409 here means the SKILL.md changed under us; re-read the catalog.
      setError(err instanceof Error ? err.message : t('skills.errors.toggleFailed'));
      void loadSkills();
    } finally {
      setToggling(false);
    }
  }, [loadSkills, t]);

  const selectedSummary = skills.find((skill) => skill.name === selected?.name);

  if (isLoading && skills.length === 0) {
    return <div className="p-6 text-sm text-muted-foreground">{t('skills.loading')}</div>;
  }

  if (error && skills.length === 0) {
    return (
      <div className="flex h-full items-center justify-center p-6">
        <Card className="max-w-md">
          <CardHeader>
            <CardTitle>{t('skills.loadFailed')}</CardTitle>
            <CardDescription>{error}</CardDescription>
          </CardHeader>
          <CardContent>
            <Button onClick={() => void Promise.all([loadSkills(), loadRevisions()])}>{t('common.retry')}</Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <Tabs defaultValue="skills" className="flex h-full min-h-0 flex-col p-4 sm:p-6">
      <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <TabsList>
          <TabsTrigger value="skills">{t('skills.installed', { count: skills.length })}</TabsTrigger>
          {marketplaceEnabled && <TabsTrigger value="marketplace">{t('skills.tabMarketplace')}</TabsTrigger>}
          <TabsTrigger value="requests">{t('skills.tabRequests', { count: revisions.length })}</TabsTrigger>
        </TabsList>
        <Button
          variant="outline"
          size="sm"
          disabled={isLoading}
          onClick={() => void Promise.all([loadSkills(), loadRevisions()])}
        >
          <RefreshCw className="mr-2 h-4 w-4" />
          {t('common.refresh')}
        </Button>
      </div>

      {error && <p className="mb-4 min-w-0 break-all text-sm text-destructive">{error}</p>}

      <TabsContent value="skills" className="mt-0 min-h-0 flex-1">
        {skills.length === 0 ? (
          <Card>
            <CardContent className="py-12 text-center text-muted-foreground">
              <Zap className="mx-auto mb-3 h-10 w-10 opacity-50" />
              {t('skills.empty')}
              <p className="mx-auto mt-2 max-w-md text-xs">{t('skills.emptyHint')}</p>
            </CardContent>
          </Card>
        ) : (
          <MasterDetail
            selected={selected !== null}
            onBack={() => setSelected(null)}
            columnsClassName="md:grid-cols-[minmax(16rem,0.8fr)_minmax(0,1.6fr)] md:gap-4"
            master={
              <ScrollArea className="h-full min-h-0 rounded-xl border bg-card">
                <div className="space-y-1 p-2">
                  {skills.map((skill) => (
                    <button
                      key={skill.name}
                      type="button"
                      onClick={() => void openSkill(skill.name)}
                      className={`w-full rounded-lg p-3 text-left transition-colors hover:bg-accent ${
                        selected?.name === skill.name ? 'bg-accent' : ''
                      }`}
                    >
                      <div className="flex items-start justify-between gap-2">
                        <span className="min-w-0 font-medium">{skill.name}</span>
                        <Badge variant={skill.enabled ? 'default' : 'secondary'}>
                          {skill.enabled ? t('common.enabled') : t('common.disabled')}
                        </Badge>
                      </div>
                      <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{skill.description}</p>
                      {skill.warnings.length > 0 && (
                        <p className="mt-2 text-xs text-amber-600 dark:text-amber-400">
                          {t('skills.warningCount', { count: skill.warnings.length })}
                        </p>
                      )}
                    </button>
                  ))}
                </div>
              </ScrollArea>
            }
            detail={
              selected ? (
                <article className="h-full overflow-auto rounded-xl border bg-card p-4 sm:p-6">
                  <div className="mb-5 flex flex-wrap items-start justify-between gap-3">
                    <div className="min-w-0">
                      <h2 className="text-xl font-semibold">{selectedSummary?.name ?? selected.name}</h2>
                      <p className="mt-1 text-sm text-muted-foreground">{selected.description}</p>
                    </div>
                    <div className="flex items-center gap-2">
                      <Badge variant={selected.enabled ? 'default' : 'secondary'}>
                        {selected.enabled ? t('common.enabled') : t('common.disabled')}
                      </Badge>
                      <Switch
                        checked={selected.enabled}
                        disabled={toggling}
                        onCheckedChange={() => {
                          const summary = selectedSummary;
                          if (summary) void toggleSkill(summary);
                        }}
                      />
                    </div>
                  </div>

                  <div className="mb-5 grid gap-3 text-sm sm:grid-cols-2">
                    <p className="min-w-0 break-all"><span className="text-muted-foreground">{t('skills.contentHash')}</span>{selected.hash}</p>
                    <p><span className="text-muted-foreground">{t('skills.supportingFiles')}</span>{selected.supporting_files.length}</p>
                  </div>

                  {selected.warnings.length > 0 && (
                    <ul className="mb-5 list-inside list-disc rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-300">
                      {selected.warnings.map((warning) => (
                        <li key={warning}>{warning}</li>
                      ))}
                    </ul>
                  )}

                  {selected.supporting_files.length > 0 && (
                    <div className="mb-5 flex flex-wrap gap-2">
                      {selected.supporting_files.map((file) => (
                        <Button key={file} variant="outline" size="sm" onClick={() => void openSkill(selected.name, file)}>
                          {file}
                        </Button>
                      ))}
                    </div>
                  )}

                  <div className="rounded-lg bg-muted p-4">
                    <p className="mb-2 text-xs text-muted-foreground">{selected.relative_path}</p>
                    <pre className="whitespace-pre-wrap break-words font-sans text-sm leading-6">{selected.content}</pre>
                  </div>
                </article>
              ) : (
                <div className="flex h-full min-h-64 items-center justify-center rounded-xl border bg-card p-6 text-center text-muted-foreground">
                  <div>
                    <BookOpen className="mx-auto mb-3 h-10 w-10 opacity-50" />
                    {t('skills.selectHint')}
                  </div>
                </div>
              )
            }
          />
        )}
      </TabsContent>

      {marketplaceEnabled && (
        <TabsContent value="marketplace" className="mt-0 min-h-0 flex-1">
          <MarketplaceTab
            installedNames={skills.map((skill) => skill.name)}
            onInstalledChange={() => void loadSkills()}
          />
        </TabsContent>
      )}

      <TabsContent value="requests" className="mt-0 min-h-0 flex-1 overflow-auto">
        {revisions.length === 0 ? (
          <Card>
            <CardContent className="py-12 text-center text-muted-foreground">{t('skills.requestsEmpty')}</CardContent>
          </Card>
        ) : (
          <div className="space-y-3">
            {revisions.map((revision) => (
              <Card key={revision.id}>
                <CardHeader>
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <CardTitle className="text-base">{revision.skill_name}</CardTitle>
                      <CardDescription>
                        {revision.action} · {revision.target_path}
                        {revision.run_id ? ` · ${t('skills.revisionRun')} ${revision.run_id}` : ''}
                        {' · '}
                        {new Date(revision.created_at).toLocaleString()}
                      </CardDescription>
                    </div>
                    <Badge variant={revision.status === 'pending' ? 'default' : 'secondary'}>
                      {t(`skills.status.${revision.status}`)}
                    </Badge>
                  </div>
                </CardHeader>
                <CardContent className="space-y-3">
                  {revision.warnings.length > 0 && (
                    <ul className="list-inside list-disc text-xs text-amber-600 dark:text-amber-400">
                      {revision.warnings.map((warning) => (
                        <li key={warning}>{warning}</li>
                      ))}
                    </ul>
                  )}
                  <div className="rounded-lg bg-muted p-3">
                    <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-words font-sans text-xs leading-5">{revision.preview}</pre>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </TabsContent>
    </Tabs>
  );
}
