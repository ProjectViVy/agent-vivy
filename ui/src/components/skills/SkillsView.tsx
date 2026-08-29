import { useSkills } from '@/hooks/useSkills';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ScrollArea } from '@/components/ui/scroll-area';
import { MasterDetail } from '@/components/layout/MasterDetail';
import { useTranslation } from '@/i18n';
import { BookOpen, FileText, RefreshCw, Zap } from 'lucide-react';

export function SkillsView() {
  const { t } = useTranslation();
  const { skills, selected, isLoading, error, loadSkills, loadSkillDocument, clearSelected } = useSkills();

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
            <Button onClick={() => void loadSkills()}>{t('common.retry')}</Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col p-4 sm:p-6">
      <div className="mb-4 flex items-center justify-between gap-3">
        <h1 className="text-lg font-semibold">{t('skills.installed')}</h1>
        <Button variant="outline" size="sm" disabled={isLoading} onClick={() => void loadSkills()}>
          <RefreshCw className="mr-2 h-4 w-4" />
          {t('common.refresh')}
        </Button>
      </div>

      {error ? <p className="mb-4 text-sm text-destructive">{error}</p> : null}

      {skills.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center text-muted-foreground">
            <Zap className="mx-auto mb-3 h-10 w-10 opacity-50" />
            <p>{t('skills.empty')}</p>
            <p className="mt-2 text-sm">{t('skills.emptyHint')}</p>
            <Button className="mt-4" variant="outline" size="sm" onClick={() => void loadSkills()}>
              {t('common.refresh')}
            </Button>
          </CardContent>
        </Card>
      ) : (
        <MasterDetail
          selected={selected !== null}
          onBack={clearSelected}
          columnsClassName="md:grid-cols-[minmax(16rem,0.8fr)_minmax(0,1.6fr)] md:gap-4"
          master={
            <ScrollArea className="h-full min-h-0 rounded-xl border bg-card">
              <div className="space-y-1 p-2">
                {skills.map((skill) => (
                  <button
                    key={skill.name}
                    type="button"
                    onClick={() => void loadSkillDocument(skill.name)}
                    className={`w-full rounded-lg p-3 text-left transition-colors hover:bg-accent ${
                      selected?.name === skill.name ? 'bg-accent' : ''
                    }`}
                  >
                    <span className="min-w-0 font-medium">{skill.name}</span>
                    <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{skill.description}</p>
                    {skill.warnings.length > 0 ? (
                      <p className="mt-2 text-xs text-amber-700 dark:text-amber-300">
                        {t('skills.warningCount', { count: skill.warnings.length })}
                      </p>
                    ) : null}
                  </button>
                ))}
              </div>
            </ScrollArea>
          }
          detail={
            selected ? (
              <article className="h-full overflow-auto rounded-xl border bg-card p-4 sm:p-6">
                <div className="mb-5 min-w-0">
                  <h2 className="text-xl font-semibold">{selected.name}</h2>
                  <p className="mt-1 text-sm text-muted-foreground">{selected.description}</p>
                </div>
                {selected.warnings.length > 0 ? (
                  <div className="mb-5 space-y-1 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-sm">
                    {selected.warnings.map((warning) => (
                      <p key={warning}>{warning}</p>
                    ))}
                  </div>
                ) : null}
                {selected.supporting_files.length > 0 ? (
                  <div className="mb-5">
                    <p className="mb-2 text-xs font-medium text-muted-foreground">{t('skills.supportingFiles')}</p>
                    <div className="flex flex-wrap gap-2">
                      <Button
                        type="button"
                        size="sm"
                        variant={selected.relative_path === 'SKILL.md' ? 'secondary' : 'outline'}
                        onClick={() => void loadSkillDocument(selected.name)}
                      >
                        SKILL.md
                      </Button>
                      {selected.supporting_files.map((file) => (
                        <Button
                          key={file}
                          type="button"
                          size="sm"
                          variant={selected.relative_path.endsWith(file) ? 'secondary' : 'outline'}
                          onClick={() => void loadSkillDocument(selected.name, file)}
                        >
                          <FileText className="mr-1 h-3.5 w-3.5" />
                          {file}
                        </Button>
                      ))}
                    </div>
                  </div>
                ) : null}
                <div className="rounded-lg bg-muted p-4">
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
    </div>
  );
}
