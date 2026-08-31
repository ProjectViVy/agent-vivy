import { describe, expect, it } from 'vitest';
import { diffSplitRows, looksLikeDiff, parseToolResultDiff, parseUnifiedDiff } from './diff';

const SAMPLE = ['--- a/notes.txt', '+++ b/notes.txt', '@@ -1,3 +1,3 @@', ' one', '-two', '+TWO', ' three'].join('\n');

const TWO_HUNKS = [
  '--- a/f', '+++ b/f',
  '@@ -1,2 +1,2 @@', '-a', '+b', ' ctx',
  '@@ -10,2 +10,2 @@', ' old', '-new',
].join('\n');

describe('parseUnifiedDiff', () => {
  it('parses hunks, line numbers and stats', () => {
    const parsed = parseUnifiedDiff(SAMPLE);
    expect(parsed).not.toBeNull();
    expect(parsed!.hunks).toHaveLength(1);
    expect(parsed!.additions).toBe(1);
    expect(parsed!.deletions).toBe(1);
    expect(parsed!.truncated).toBe(false);
    const [context, del, add, last] = parsed!.hunks[0].lines;
    expect(context).toMatchObject({ type: 'context', old: 1, new: 1, text: 'one' });
    expect(del).toMatchObject({ type: 'del', old: 2, text: 'two' });
    expect(add).toMatchObject({ type: 'add', new: 2, text: 'TWO' });
    expect(last).toMatchObject({ type: 'context', old: 3, new: 3, text: 'three' });
  });

  it('resets numbering per hunk header and aggregates stats', () => {
    const parsed = parseUnifiedDiff(TWO_HUNKS);
    expect(parsed).not.toBeNull();
    expect(parsed!.hunks).toHaveLength(2);
    expect(parsed!.additions).toBe(1);
    expect(parsed!.deletions).toBe(2);
    const second = parsed!.hunks[1].lines;
    expect(second[0]).toMatchObject({ type: 'context', old: 10, new: 10 });
    expect(second[1]).toMatchObject({ type: 'del', old: 11 });
  });

  it('rejects text without a file header or hunk', () => {
    expect(parseUnifiedDiff('')).toBeNull();
    expect(parseUnifiedDiff('just some text\nwith @@ -1,1 +1,1 @@ inside')).toBeNull();
    expect(parseUnifiedDiff('--- a/f\n+++ b/f\nno hunks here')).toBeNull();
    expect(looksLikeDiff('plain preview text')).toBe(false);
  });

  it('accepts diffs via looksLikeDiff', () => {
    expect(looksLikeDiff(SAMPLE)).toBe(true);
  });

  it('flags the server truncation marker', () => {
    const parsed = parseUnifiedDiff(`${SAMPLE}\n[diff truncated]`);
    expect(parsed!.truncated).toBe(true);
  });

  it('treats no-newline notices as meta lines', () => {
    const parsed = parseUnifiedDiff(`${SAMPLE}\n\\ No newline at end of file`);
    expect(parsed!.hunks[0].lines.at(-1)).toMatchObject({ type: 'meta' });
    expect(parsed!.additions).toBe(1);
  });

  it('tolerates legacy bare @@ and prefix-less context lines', () => {
    const parsed = parseUnifiedDiff(['--- f', '+++ f', '@@', '-a', ' b', '+c'].join('\n'));
    expect(parsed).not.toBeNull();
    const [del, context, add] = parsed!.hunks[0].lines;
    expect(del).toMatchObject({ type: 'del', text: 'a' });
    expect(del.old).toBeUndefined();
    expect(context).toMatchObject({ type: 'context', text: 'b' });
    expect(add).toMatchObject({ type: 'add', text: 'c' });
  });
});

describe('diffSplitRows', () => {
  it('pairs del/add runs index-wise and passes context through', () => {
    const parsed = parseUnifiedDiff([
      '--- a/f', '+++ b/f', '@@ -1,5 +1,5 @@',
      ' x', '-a1', '-a2', '+b1', '+b2', ' y',
    ].join('\n'))!;
    const rows = diffSplitRows(parsed.hunks[0]);
    expect(rows).toHaveLength(4);
    expect(rows[0].left!.type).toBe('context');
    expect(rows[0].right!.new).toBe(1);
    expect(rows[1]).toMatchObject({ left: { text: 'a1', type: 'del' }, right: { text: 'b1', type: 'add' } });
    expect(rows[2]).toMatchObject({ left: { text: 'a2' }, right: { text: 'b2' } });
    expect(rows[3].right!.text).toBe('y');
  });

  it('pads unbalanced runs with null sides', () => {
    const parsed = parseUnifiedDiff([
      '--- a/f', '+++ b/f', '@@ -1,3 +1,2 @@', '-only', '+one', '+two',
    ].join('\n'))!;
    const rows = diffSplitRows(parsed.hunks[0]);
    expect(rows).toHaveLength(2);
    expect(rows[0]).toMatchObject({ left: { text: 'only' }, right: { text: 'one' } });
    expect(rows[1]).toMatchObject({ left: null, right: { text: 'two' } });
  });
});

describe('parseToolResultDiff', () => {
  it('extracts diff and path from a FileMutationResult payload', () => {
    const payload = JSON.stringify({ path: 'notes.txt', changed: true, bytes: 9, sha256: 'ab', diff: SAMPLE });
    expect(parseToolResultDiff(payload)).toEqual({ diff: SAMPLE, path: 'notes.txt' });
  });

  it('returns null for non-diff tool results and invalid payloads', () => {
    expect(parseToolResultDiff(JSON.stringify({ path: 'f', content: 'no diff here' }))).toBeNull();
    expect(parseToolResultDiff(JSON.stringify({ diff: '   ' }))).toBeNull();
    expect(parseToolResultDiff('plain text result')).toBeNull();
    expect(parseToolResultDiff('{broken json')).toBeNull();
  });
});
