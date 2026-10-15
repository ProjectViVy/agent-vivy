import type { PersonaKind } from './persona-client';

/**
 * The authority owns the document file names; the face only labels them, so
 * this table has exactly one home and every view reads it.
 */
export const KIND_LABELS: Record<PersonaKind, string> = {
  identity: 'IDENTITY.MD',
  relationship: 'RELATIONSHIP.MD',
  redline: 'REDLINE.MD',
  user: 'USER.MD',
  world: 'WORLD.MD',
  dream: 'DREAM.MD',
  dark: 'DARK.MD',
  mission: 'MISSION.MD',
};
