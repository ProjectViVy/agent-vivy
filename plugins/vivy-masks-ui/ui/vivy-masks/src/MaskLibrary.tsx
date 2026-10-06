import type { UITranslator } from '@vivy/ui-sdk';
import { useState } from 'react';
import { Check, Plus, Search } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardFooter } from '@/components/ui/card';
import { MaskIdentity } from './MaskIdentity';
import { maskKind, type MaskChoice } from './mask-view';

export function MaskLibrary({
  choices,
  selection,
  openedId,
  catalogPending,
  catalogError,
  onView,
  onUse,
  onNew,
  t,
}: {
  readonly choices: readonly MaskChoice[];
  readonly selection: {
    readonly id: string;
    readonly known: boolean;
    readonly canSelect: boolean;
  };
  readonly openedId: string | null;
  readonly catalogPending: boolean;
  readonly catalogError: boolean;
  readonly onView: (id: string) => void;
  readonly onUse: (id: string) => void;
  readonly onNew: () => void;
  readonly t: UITranslator;
}) {
  const [query, setQuery] = useState('');
  const filtered = choices.filter((mask) =>
    `${mask.name} ${mask.description}`
      .toLocaleLowerCase()
      .includes(query.toLocaleLowerCase()),
  );
  return (
    <section
      className="min-w-0"
      aria-label={t('plugin.vivy/masks-ui.catalogLabel')}
    >
      <div className="mb-4 flex items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold">
            {t('plugin.vivy/masks-ui.catalogTitle')}
          </h2>
          <p className="mt-1 text-xs text-muted-foreground">
            {t('plugin.vivy/masks-ui.libraryHint')}
          </p>
        </div>
        <Button size="sm" data-mask-action="new" onClick={onNew}>
          <Plus className="h-4 w-4" />
          {t('plugin.vivy/masks-ui.new')}
        </Button>
      </div>
      <label className="mb-4 flex items-center gap-2 rounded-xl border bg-card px-3 py-2.5">
        <Search className="h-4 w-4 shrink-0 text-muted-foreground" />
        <input
          aria-label={t('plugin.vivy/masks-ui.search')}
          placeholder={t('plugin.vivy/masks-ui.search')}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="min-w-0 flex-1 bg-transparent text-sm outline-none"
        />
      </label>
      <div className="grid min-w-0 gap-4 sm:grid-cols-2">
        {filtered.map((mask) => (
          <Card
            key={mask.id}
            data-mask-id={mask.id}
            className={`min-w-0 gap-0 overflow-hidden rounded-2xl py-0 ${selection.known && selection.id === mask.id ? 'border-primary/30 bg-primary/5' : openedId === mask.id ? 'border-primary/50' : 'bg-card'}`}
          >
            <button
              type="button"
              aria-pressed={openedId === mask.id}
              className="flex w-full cursor-pointer flex-col gap-3 p-4 text-left transition-colors hover:bg-accent/30"
              onClick={() => {
                onView(mask.id);
              }}
            >
              <span className="flex w-full min-w-0 items-center gap-3">
                <MaskIdentity id={mask.id} />
                <span className="min-w-0 flex-1">
                  <span className="block break-words text-sm font-semibold">
                    {mask.name}
                  </span>
                  <span className="mt-0.5 block text-xs text-muted-foreground">
                    {maskKind(t, mask)}
                  </span>
                </span>
                {selection.known && selection.id === mask.id ? (
                  <Badge variant="secondary" className="shrink-0 text-[10px]">
                    {t('plugin.vivy/masks-ui.currentBadge')}
                  </Badge>
                ) : null}
              </span>
              <span className="block min-h-10 break-words text-sm leading-relaxed text-muted-foreground">
                {mask.description}
              </span>
            </button>
            <CardFooter className="flex flex-wrap justify-between gap-2 px-4 pb-4">
              <Button
                variant="link"
                size="sm"
                className="h-8 px-0 text-xs"
                onClick={() => {
                  onView(mask.id);
                }}
              >
                {t('plugin.vivy/masks-ui.viewDetails')}
              </Button>
              <Button
                data-mask-use
                size="sm"
                variant={
                  selection.id === mask.id && selection.known
                    ? 'secondary'
                    : 'outline'
                }
                disabled={
                  !selection.canSelect ||
                  (selection.known && selection.id === mask.id)
                }
                onClick={() => onUse(mask.id)}
              >
                {selection.known && selection.id === mask.id ? (
                  <Check className="h-3.5 w-3.5" />
                ) : null}
                {t(
                  selection.known && selection.id === mask.id
                    ? 'plugin.vivy/masks-ui.inUse'
                    : 'plugin.vivy/masks-ui.apply',
                )}
              </Button>
            </CardFooter>
          </Card>
        ))}
      </div>
      {catalogPending ? (
        <p className="mt-4 text-sm text-muted-foreground">
          {t('plugin.vivy/masks-ui.loading')}
        </p>
      ) : !catalogError && choices.length === 1 ? (
        <p className="mt-4 text-sm text-muted-foreground">
          {t('plugin.vivy/masks-ui.empty')}
        </p>
      ) : null}
      {!filtered.length ? (
        <p className="mt-4 text-sm text-muted-foreground">
          {t('plugin.vivy/masks-ui.noMatches')}
        </p>
      ) : null}
    </section>
  );
}
