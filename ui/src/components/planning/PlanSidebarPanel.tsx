/**
 * 计划侧边栏面板组件
 * 显示活跃计划的标题、目标、阶段、待办事项列表，支持展开/折叠
 */

import { useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import { Badge } from '@/components/ui/badge';
import { ScrollArea } from '@/components/ui/scroll-area';
import { ChevronDown, ChevronUp, Target, ListTodo, AlertCircle } from 'lucide-react';
import type { PlanRuntimeState, PlanRuntimeTodo } from '@/lib/types';
import { useTranslation } from '@/i18n';

interface PlanSidebarPanelProps {
  plan: PlanRuntimeState | null;
  todos: PlanRuntimeTodo[];
  validationIssues?: string[];
}

export function PlanSidebarPanel({ plan, todos, validationIssues }: PlanSidebarPanelProps) {
  const { t } = useTranslation();
  const [todosOpen, setTodosOpen] = useState(true);
  const [issuesOpen, setIssuesOpen] = useState(true);

  if (!plan) {
    return (
      <Card className="border-dashed">
        <CardContent className="py-8 text-center text-muted-foreground text-sm">
          <Target className="h-8 w-8 mx-auto mb-2 opacity-50" />
          <p>{t('planning.noPlan')}</p>
        </CardContent>
      </Card>
    );
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed':
        return 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300';
      case 'in_progress':
        return 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300';
      case 'blocked':
        return 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300';
      default:
        return 'bg-muted';
    }
  };

  const getPriorityColor = (priority: string) => {
    switch (priority) {
      case 'high':
        return 'destructive';
      case 'medium':
        return 'default';
      default:
        return 'secondary';
    }
  };

  return (
    <div className="space-y-3">
      {/* 计划基本信息 */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base flex items-center gap-2">
            <Target className="h-4 w-4" />
            {plan.title}
          </CardTitle>
          <div className="flex flex-wrap gap-2 mt-2">
            <Badge variant="outline">{plan.phase}</Badge>
            <Badge variant={plan.status === 'approved' ? 'default' : 'secondary'}>
              {plan.status}
            </Badge>
          </div>
        </CardHeader>
        <CardContent className="pt-0">
          <p className="text-sm text-muted-foreground line-clamp-3">{plan.goal}</p>
          {plan.summary && (
            <p className="text-xs text-muted-foreground mt-2 line-clamp-2">{plan.summary}</p>
          )}
        </CardContent>
      </Card>

      {/* 待办事项 */}
      {todos.length > 0 && (
        <Collapsible open={todosOpen} onOpenChange={setTodosOpen}>
          <Card>
            <CollapsibleTrigger asChild>
              <CardHeader className="pb-2 cursor-pointer hover:bg-muted/50 transition-colors">
                <div className="flex items-center justify-between">
                  <CardTitle className="text-sm flex items-center gap-2">
                    <ListTodo className="h-4 w-4" />
                    {t('planning.todos', { count: todos.length })}
                  </CardTitle>
                  {todosOpen ? (
                    <ChevronUp className="h-4 w-4 text-muted-foreground" />
                  ) : (
                    <ChevronDown className="h-4 w-4 text-muted-foreground" />
                  )}
                </div>
              </CardHeader>
            </CollapsibleTrigger>
            <CollapsibleContent>
              <ScrollArea className="max-h-[300px]">
                <CardContent className="pt-0 space-y-2">
                  {todos.map((todo) => (
                    <div
                      key={todo.id}
                      className="p-2 rounded-lg border bg-card text-sm"
                    >
                      <div className="flex items-start justify-between gap-2">
                        <span className="font-medium flex-1">{todo.title}</span>
                        <Badge variant={getPriorityColor(todo.priority)} className="shrink-0 text-xs">
                          {todo.priority}
                        </Badge>
                      </div>
                      {todo.detail && (
                        <p className="text-xs text-muted-foreground mt-1 line-clamp-2">
                          {todo.detail}
                        </p>
                      )}
                      <div className="flex items-center gap-2 mt-1.5">
                        <Badge variant="outline" className={`text-xs ${getStatusColor(todo.status)}`}>
                          {todo.status}
                        </Badge>
                        {todo.block_reason && (
                          <span className="text-xs text-destructive truncate">
                            {todo.block_reason}
                          </span>
                        )}
                      </div>
                    </div>
                  ))}
                </CardContent>
              </ScrollArea>
            </CollapsibleContent>
          </Card>
        </Collapsible>
      )}

      {/* 验证问题 */}
      {validationIssues && validationIssues.length > 0 && (
        <Collapsible open={issuesOpen} onOpenChange={setIssuesOpen}>
          <Card className="border-destructive/50">
            <CollapsibleTrigger asChild>
              <CardHeader className="pb-2 cursor-pointer hover:bg-destructive/5 transition-colors">
                <div className="flex items-center justify-between">
                  <CardTitle className="text-sm flex items-center gap-2 text-destructive">
                    <AlertCircle className="h-4 w-4" />
                    {t('planning.validationIssues', { count: validationIssues.length })}
                  </CardTitle>
                  {issuesOpen ? (
                    <ChevronUp className="h-4 w-4 text-muted-foreground" />
                  ) : (
                    <ChevronDown className="h-4 w-4 text-muted-foreground" />
                  )}
                </div>
              </CardHeader>
            </CollapsibleTrigger>
            <CollapsibleContent>
              <CardContent className="pt-0 space-y-1.5">
                {validationIssues.map((issue, index) => (
                  <p key={index} className="text-xs text-destructive flex items-start gap-1.5">
                    <span className="mt-0.5">•</span>
                    <span>{issue}</span>
                  </p>
                ))}
              </CardContent>
            </CollapsibleContent>
          </Card>
        </Collapsible>
      )}
    </div>
  );
}
