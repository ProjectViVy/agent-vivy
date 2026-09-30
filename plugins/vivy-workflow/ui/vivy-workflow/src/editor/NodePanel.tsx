// Properties panel for the selected graph node. Every edit funnels through
// the vendored edit helpers on the artifact — the panel never mutates node
// objects in place, so the definition stays the single source of truth.

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Separator } from '@/components/ui/separator';
import { Textarea } from '@/components/ui/textarea';
import type { UITranslator } from '@vivy/ui-sdk';
import { useEffect, useState } from 'react';
import { Trash2 } from 'lucide-react';
import type { NodeDescriptor } from '../studio/schema';
import { exitOutputName, parseJSONOrError, patchNode, setExit, setExitOutputName } from '../studio/edit';
import type { Artifact, Node } from '../studio/schema';

export interface NodePanelProps {
  artifact: Artifact;
  node: Node;
  descriptor: NodeDescriptor | undefined;
  t: UITranslator;
  disabled?: boolean;
  onChange: (artifact: Artifact) => void;
  onDelete: (id: string) => void;
}

function fieldValue(node: Node, key: string): unknown {
  const config = node.config && typeof node.config === 'object' ? (node.config as Record<string, unknown>) : {};
  return config[key];
}

function setConfigField(node: Node, key: string, value: unknown): Node {
  const config = node.config && typeof node.config === 'object' ? { ...(node.config as Record<string, unknown>) } : {};
  if (value === undefined) delete config[key];
  else config[key] = value;
  return { ...node, config: config as Node['config'] };
}

export function NodePanel({ artifact, node, descriptor, t, disabled, onChange, onDelete }: NodePanelProps) {
  const [inputsDraft, setInputsDraft] = useState(() => JSON.stringify(fieldValue(node, 'inputs') ?? {}, null, 2));
  const [inputsError, setInputsError] = useState<string | null>(null);
  useEffect(() => {
    setInputsDraft(JSON.stringify(fieldValue(node, 'inputs') ?? {}, null, 2));
    setInputsError(null);
  }, [node.id, node.config]);

  const isExit = artifact.definition.graph.exits.includes(node.id);
  const outputName = exitOutputName(artifact, node.id);
  const isCall = node.kind === 'call';
  const task = typeof fieldValue(node, 'task') === 'string' ? (fieldValue(node, 'task') as string) : '';
  const tools = Array.isArray(fieldValue(node, 'tool_names'))
    ? (fieldValue(node, 'tool_names') as unknown[]).filter((x): x is string => typeof x === 'string')
    : [];

  const patch = (next: Node) => onChange(patchNode(artifact, next));
  const commitInputs = (text: string) => {
    const parsed = parseJSONOrError(text);
    if (!parsed.ok) {
      setInputsError(parsed.error);
      return;
    }
    setInputsError(null);
    patch(setConfigField(node, 'inputs', parsed.value));
  };

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="border-b px-3 py-2">
        <div className="flex items-center gap-2">
          <h3 className="text-xs font-semibold">{t('plugin.vivy/workflow-ui.node.title')}</h3>
          <Badge variant="outline" className="text-[10px]">{node.kind}</Badge>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="ml-auto h-6 w-6 text-muted-foreground hover:text-destructive"
            disabled={disabled}
            onClick={() => onDelete(node.id)}
            title={t('plugin.vivy/workflow-ui.node.delete')}
          >
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
        <p className="mt-0.5 truncate text-[11px] text-muted-foreground">
          {node.id}
          {descriptor?.display?.title ? ` · ${descriptor.display.title}` : ''}
        </p>
      </div>
      <div className="min-h-0 flex-1 space-y-3 overflow-auto p-3">
        {!isCall ? (
          <p className="text-[11px] text-muted-foreground">{t('plugin.vivy/workflow-ui.node.advanced')}</p>
        ) : (
          <>
            <div className="space-y-1.5">
              <Label htmlFor="wf-node-task" className="text-xs">{t('plugin.vivy/workflow-ui.node.task')}</Label>
              <Textarea
                id="wf-node-task"
                value={task}
                disabled={disabled}
                rows={4}
                className="text-xs"
                onChange={(e) => patch(setConfigField(node, 'task', e.target.value))}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="wf-node-tools" className="text-xs">{t('plugin.vivy/workflow-ui.node.tools')}</Label>
              <Input
                id="wf-node-tools"
                value={tools.join(', ')}
                disabled={disabled}
                placeholder={t('plugin.vivy/workflow-ui.node.toolsPlaceholder')}
                className="h-8 text-xs"
                onChange={(e) => {
                  const list = e.target.value.split(',').map((x) => x.trim()).filter((x) => x !== '');
                  patch(setConfigField(node, 'tool_names', list.length > 0 ? list : undefined));
                }}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="wf-node-inputs" className="text-xs">{t('plugin.vivy/workflow-ui.node.inputs')}</Label>
              <Textarea
                id="wf-node-inputs"
                value={inputsDraft}
                disabled={disabled}
                rows={5}
                spellCheck={false}
                className="font-mono text-[11px]"
                onChange={(e) => setInputsDraft(e.target.value)}
                onBlur={(e) => commitInputs(e.target.value)}
              />
              {inputsError ? <p className="text-[11px] text-destructive">{inputsError}</p> : null}
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="wf-node-timeout" className="text-xs">{t('plugin.vivy/workflow-ui.node.timeout')}</Label>
              <Input
                id="wf-node-timeout"
                type="number"
                min={0}
                value={typeof node.timeout_ms === 'number' ? node.timeout_ms : ''}
                disabled={disabled}
                placeholder={t('plugin.vivy/workflow-ui.node.timeoutPlaceholder')}
                className="h-8 text-xs"
                onChange={(e) => {
                  const raw = e.target.value;
                  const next = { ...node };
                  if (raw === '') delete next.timeout_ms;
                  else next.timeout_ms = Math.max(0, Math.floor(Number(raw)));
                  patch(next);
                }}
              />
            </div>
            <Separator />
            <div className="flex items-center gap-2">
              <Checkbox
                id="wf-node-exit"
                checked={isExit}
                disabled={disabled}
                onCheckedChange={(checked) => onChange(setExit(artifact, node.id, checked === true))}
              />
              <Label htmlFor="wf-node-exit" className="text-xs">{t('plugin.vivy/workflow-ui.node.exit')}</Label>
            </div>
            {isExit ? (
              <div className="space-y-1.5">
                <Label htmlFor="wf-node-output" className="text-xs">{t('plugin.vivy/workflow-ui.node.outputName')}</Label>
                <Input
                  id="wf-node-output"
                  value={outputName}
                  disabled={disabled}
                  placeholder={node.id}
                  className="h-8 text-xs"
                  onChange={(e) => onChange(setExitOutputName(artifact, node.id, e.target.value))}
                />
              </div>
            ) : null}
          </>
        )}
      </div>
    </div>
  );
}
