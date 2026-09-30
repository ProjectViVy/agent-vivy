// VIVY-styled React Flow node for the workflow editor. Renders the semantic
// node (id, type, task preview, entry/exit markers) with host-kit tokens;
// switch nodes expose one labeled source handle per case port plus the
// default port, call nodes a single source handle.

import { Handle, Position, type Node, type NodeProps } from '@xyflow/react';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { usePluginTranslation } from '@vivy/ui-sdk';
import type { CanvasNodeData } from '../studio/graph';

function shortType(type: string | undefined): string {
  if (!type) return '';
  const stripped = type.replace(/^inofy\./, '').replace(/@[^@]*$/, '');
  return stripped || type;
}

function taskPreview(config: unknown): string {
  if (config && typeof config === 'object' && typeof (config as { task?: unknown }).task === 'string') {
    return (config as { task: string }).task;
  }
  return '';
}

export function WorkflowNode({ data, selected }: NodeProps<Node<CanvasNodeData>>) {
  const { t } = usePluginTranslation();
  const n = data.node;
  const task = taskPreview(n.config);
  const cases = Array.isArray(n.cases) ? n.cases : [];
  const ports = cases.map((c) => c.port).filter((p): p is string => typeof p === 'string' && p !== '');
  if (typeof n.default_port === 'string' && n.default_port !== '') ports.push(n.default_port);
  return (
    <div
      className={cn(
        'w-[200px] rounded-lg border bg-card text-card-foreground shadow-sm',
        selected && 'ring-2 ring-primary ring-offset-1',
        data.unresolved && 'border-destructive',
      )}
    >
      <Handle type="target" position={Position.Left} className="!h-2 !w-2 !border-border !bg-muted-foreground" />
      <div className="flex items-center gap-1.5 border-b px-2.5 py-1.5">
        {data.isEntry ? <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-primary" aria-hidden /> : null}
        <span className="truncate text-xs font-medium">{n.id}</span>
        {data.isExit ? (
          <Badge variant="secondary" className="ml-auto h-4 shrink-0 px-1 text-[10px]">
            {t('plugin.vivy/workflow-ui.node.exitBadge')}
          </Badge>
        ) : null}
      </div>
      <div className="px-2.5 py-1.5">
        <p className="truncate text-[11px] text-muted-foreground">{shortType(n.type) || t('plugin.vivy/workflow-ui.node.untyped')}</p>
        {task ? <p className="mt-0.5 line-clamp-2 text-[11px] leading-snug">{task}</p> : null}
      </div>
      {ports.length === 0 ? (
        <Handle type="source" position={Position.Right} className="!h-2 !w-2 !border-border !bg-muted-foreground" />
      ) : (
        ports.map((port, i) => (
          <Handle
            key={port}
            id={port}
            type="source"
            position={Position.Right}
            style={{ top: 24 + i * 14 }}
            className="!h-2 !w-2 !border-border !bg-muted-foreground"
          />
        ))
      )}
    </div>
  );
}
