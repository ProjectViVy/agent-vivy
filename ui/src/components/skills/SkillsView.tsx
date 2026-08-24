import { useSkills } from '@/hooks/useSkills';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { BookOpen, RefreshCw, Zap } from 'lucide-react';

export function SkillsView() {
  const {
    skills,
    selectedSkill,
    requests,
    isLoading,
    error,
    loadSkills,
    loadSkillDocument,
    loadRequests,
  } = useSkills();

  const selectedSummary = skills.find((skill) => skill.slug === selectedSkill?.slug);

  if (isLoading && skills.length === 0) {
    return <div className="p-6 text-sm text-muted-foreground">正在加载技能…</div>;
  }

  if (error && skills.length === 0) {
    return (
      <div className="flex h-full items-center justify-center p-6">
        <Card className="max-w-md">
          <CardHeader>
            <CardTitle>无法加载技能</CardTitle>
            <CardDescription>{error}</CardDescription>
          </CardHeader>
          <CardContent>
            <Button onClick={() => void Promise.all([loadSkills(), loadRequests()])}>重试</Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <Tabs defaultValue="skills" className="flex h-full min-h-0 flex-col p-6">
      <div className="mb-4 flex items-center justify-between gap-3">
        <TabsList>
          <TabsTrigger value="skills">已安装技能</TabsTrigger>
          <TabsTrigger value="requests">变更请求 ({requests.length})</TabsTrigger>
        </TabsList>
        <Button
          variant="outline"
          size="sm"
          disabled={isLoading}
          onClick={() => void Promise.all([loadSkills(), loadRequests()])}
        >
          <RefreshCw className="mr-2 h-4 w-4" />
          刷新
        </Button>
      </div>

      {error && <p className="mb-4 text-sm text-destructive">{error}</p>}

      <TabsContent value="skills" className="mt-0 min-h-0 flex-1">
        {skills.length === 0 ? (
          <Card>
            <CardContent className="py-12 text-center text-muted-foreground">
              <Zap className="mx-auto mb-3 h-10 w-10 opacity-50" />
              暂无可用技能
            </CardContent>
          </Card>
        ) : (
          <div className="grid h-full min-h-0 gap-4 md:grid-cols-[minmax(16rem,0.8fr)_minmax(0,1.6fr)]">
            <ScrollArea className="min-h-0 rounded-xl border bg-card">
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
                      <span className="font-medium">{skill.name}</span>
                      <Badge variant={skill.enabled ? 'default' : 'secondary'}>
                        {skill.enabled ? '已启用' : '已停用'}
                      </Badge>
                    </div>
                    <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{skill.description}</p>
                    <p className="mt-2 text-xs text-muted-foreground">{skill.source === 'builtin' ? '内置' : '用户'}</p>
                  </button>
                ))}
              </div>
            </ScrollArea>

            <ScrollArea className="min-h-0 rounded-xl border bg-card">
              {selectedSkill ? (
                <article className="p-6">
                  <div className="mb-5 flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <h2 className="text-xl font-semibold">{selectedSummary?.name ?? selectedSkill.slug}</h2>
                      <p className="mt-1 text-sm text-muted-foreground">{selectedSkill.description}</p>
                    </div>
                    <div className="flex gap-2">
                      <Badge variant="outline">{selectedSkill.source === 'builtin' ? '内置' : '用户'}</Badge>
                      {selectedSkill.always && <Badge variant="secondary">始终加载</Badge>}
                    </div>
                  </div>
                  <div className="mb-5 grid gap-3 text-sm sm:grid-cols-2">
                    <p><span className="text-muted-foreground">标识：</span>{selectedSkill.slug}</p>
                    <p><span className="text-muted-foreground">更新时间：</span>{new Date(selectedSkill.updated_at).toLocaleString()}</p>
                    <p><span className="text-muted-foreground">可用状态：</span>{selectedSkill.available ? '可用' : '不可用'}</p>
                    <p><span className="text-muted-foreground">内容哈希：</span>{selectedSkill.content_hash}</p>
                  </div>
                  <div className="rounded-lg bg-muted p-4">
                    <pre className="whitespace-pre-wrap font-sans text-sm leading-6">{selectedSkill.markdown}</pre>
                  </div>
                </article>
              ) : (
                <div className="flex h-full min-h-64 items-center justify-center p-6 text-center text-muted-foreground">
                  <div>
                    <BookOpen className="mx-auto mb-3 h-10 w-10 opacity-50" />
                    选择一个技能查看说明
                  </div>
                </div>
              )}
            </ScrollArea>
          </div>
        )}
      </TabsContent>

      <TabsContent value="requests" className="mt-0 min-h-0 flex-1 overflow-auto">
        {requests.length === 0 ? (
          <Card>
            <CardContent className="py-12 text-center text-muted-foreground">暂无技能变更请求</CardContent>
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
