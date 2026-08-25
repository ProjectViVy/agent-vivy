import { useSkills } from '@/hooks/useSkills';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { MasterDetail } from '@/components/layout/MasterDetail';
import { useTranslation } from '@/i18n';
import { BookOpen, RefreshCw, Zap } from 'lucide-react';

export function SkillsView() {
  const { t } = useTranslation();
  const {
    skills,
    selectedSkill,
    requests,
    isLoading,
    error,
    loadSkills,
    loadSkillDocument,
    clearSelectedSkill,
    loadRequests,
  } = useSkills();

  const selectedSummary = skills.find((skill) => skill.slug === selectedSkill?.slug);

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
            <Button onClick={() => void Promise.all([loadSkills(), loadRequests()])}>{t('common.retry')}</Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <Tabs defaultValue="skills" className="flex h-full min-h-0 flex-col p-4 sm:p-6">
      <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <TabsList>
          <TabsTrigger value="skills">{t('skills.installed')}</TabsTrigger>
          <TabsTrigger value="requests">{t('skills.changeRequests', { count: requests.length })}</TabsTrigger>
        </TabsList>
        <Button
          variant="outline"
          size="sm"
          disabled={isLoading}
          onClick={() => void Promise.all([loadSkills(), loadRequests()])}
        >
          <RefreshCw className="mr-2 h-4 w-4" />
          {t('common.refresh')}
        </Button>
      </div>

      {error && <p className="mb-4 text-sm text-destructive">{error}</p>}

      <TabsContent value="skills" className="mt-0 min-h-0 flex-1">
        {skills.length === 0 ? (
          <Card>
            <CardContent className="py-12 text-center text-muted-foreground">
              <Zap className="mx-auto mb-3 h-10 w-10 opacity-50" />
              {t('skills.empty')}
            </CardContent>
          </Card>
        ) : (
          <MasterDetail
            selected={selectedSkill !== null}
            onBack={clearSelectedSkill}
            columnsClassName="md:grid-cols-[minmax(16rem,0.8fr)_minmax(0,1.6fr)] md:gap-4"
            master={
              <ScrollArea className="h-full min-h-0 rounded-xl border bg-card">
                <div className="space-y-1 p-2">
                  {skills.map((skill) => (
                    <button
                      key={skill.slug}
                      type="button"
                      onClick={() => void loadSkillDocument(skill.slug)}
                      className={`w-full rounded-lg p-3 text-left transition-colors hover:bg-accent ${
                        selectedSkill?.slug === skill.slug ? 'bg-accent' : ''
                      }`}
                    >
                      <div className="flex items-start justify-between gap-2">
                        <span className="min-w-0 font-medium">{skill.name}</span>
                        <Badge variant={skill.enabled ? 'default' : 'secondary'}>
                          {skill.enabled ? t('common.enabled') : t('common.disabled')}
                        </Badge>
                      </div>
                      <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{skill.description}</p>
                      <p className="mt-2 text-xs text-muted-foreground">{skill.source === 'builtin' ? t('skills.builtin') : t('skills.user')}</p>
                    </button>
                  ))}
                </div>
              </ScrollArea>
            }
            detail={
              selectedSkill ? (
                <article className="h-full overflow-auto rounded-xl border bg-card p-4 sm:p-6">
                  <div className="mb-5 flex flex-wrap items-start justify-between gap-3">
                    <div className="min-w-0">
                      <h2 className="text-xl font-semibold">{selectedSummary?.name ?? selectedSkill.slug}</h2>
                      <p className="mt-1 text-sm text-muted-foreground">{selectedSkill.description}</p>
                    </div>
                    <div className="flex gap-2">
                      <Badge variant="outline">{selectedSkill.source === 'builtin' ? t('skills.builtin') : t('skills.user')}</Badge>
                      {selectedSkill.always && <Badge variant="secondary">{t('skills.alwaysLoaded')}</Badge>}
                    </div>
                  </div>
                  <div className="mb-5 grid gap-3 text-sm sm:grid-cols-2">
                    <p className="min-w-0 break-all"><span className="text-muted-foreground">{t('skills.slug')}</span>{selectedSkill.slug}</p>
                    <p><span className="text-muted-foreground">{t('skills.updatedAt')}</span>{new Date(selectedSkill.updated_at).toLocaleString()}</p>
                    <p><span className="text-muted-foreground">{t('skills.availableStatus')}</span>{selectedSkill.available ? t('skills.available') : t('skills.unavailable')}</p>
                    <p className="min-w-0 break-all"><span className="text-muted-foreground">{t('skills.contentHash')}</span>{selectedSkill.content_hash}</p>
                  </div>
                  <div className="rounded-lg bg-muted p-4">
                    <pre className="whitespace-pre-wrap break-words font-sans text-sm leading-6">{selectedSkill.markdown}</pre>
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

      <TabsContent value="requests" className="mt-0 min-h-0 flex-1 overflow-auto">
        {requests.length === 0 ? (
          <Card>
            <CardContent className="py-12 text-center text-muted-foreground">{t('skills.noRequests')}</CardContent>
          </Card>
        ) : (
          <div className="space-y-3">
            {requests.map((request) => (
              <Card key={request.id}>
                <CardHeader>
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <CardTitle className="text-base">{request.title}</CardTitle>
                      <CardDescription>{request.slug} · {request.reason}</CardDescription>
                    </div>
                    <Badge variant={request.status === 'pending' ? 'default' : 'secondary'}>{request.status}</Badge>
                  </div>
                </CardHeader>
              </Card>
            ))}
          </div>
        )}
      </TabsContent>
    </Tabs>
  );
}
