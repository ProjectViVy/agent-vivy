import { createModuleActionClient, type FaceClientRPC } from '@vivy/ui-sdk';

/** The backend owner is the generation's cognitive authority Module. */
export const PERSONA_MODULE_ID = 'vivy/diva-cognitive' as const;

export const PERSONA_ACTIONS = Object.freeze({
  status: 'diva.cognitive.status',
  initialize: 'diva.cognitive.persona.initialize',
  read: 'diva.cognitive.persona.read',
  save: 'diva.cognitive.persona.save',
  reviewsList: 'diva.cognitive.persona.reviews.list',
  reviewDecide: 'diva.cognitive.persona.review.decide',
} as const);

export const PERSONA_KINDS = [
  'identity',
  'relationship',
  'redline',
  'user',
  'world',
  'dream',
  'dark',
  'mission',
] as const;

export type PersonaKind = (typeof PERSONA_KINDS)[number];
export type PersonaReviewDecision = 'accept' | 'reject';

export interface PersonaOutcome<T> {
  readonly status: string;
  readonly value?: T;
  readonly error?: {
    readonly code: string;
    readonly message: string;
    readonly retryable: boolean;
  };
}

export interface PersonaStatus {
  readonly profile_id?: string;
  readonly scope?: { readonly subject_id?: string; readonly kind?: string };
  readonly destination_id?: string;
  readonly persona: {
    readonly state: 'uninitialized' | 'ready' | 'incomplete' | string;
    readonly current_revisions: Readonly<Record<string, number>>;
  };
}

export interface PersonaDocument {
  readonly kind: PersonaKind;
  readonly file_name: string;
  readonly exists: boolean;
  readonly valid: boolean;
  readonly content: string;
  readonly revision: number;
  readonly content_hash: string;
  readonly updated_at: string | null;
  readonly pending_count: number;
}

export interface PersonaWriteOutcome {
  readonly document: PersonaDocument;
  readonly changed: boolean;
}

export interface PersonaReview {
  readonly id: string;
  readonly kind: PersonaKind;
  readonly base_revision: number;
  readonly base_hash: string;
  readonly proposed_markdown: string;
  readonly actor: string;
  readonly reason: string;
  readonly created_at: string;
  readonly state: 'pending' | 'accepted' | 'rejected' | 'stale' | string;
  readonly decided_at: string | null;
}

export interface PersonaReviewPage {
  readonly items: readonly PersonaReview[];
  readonly next_cursor?: string;
}

export interface PersonaInitialization {
  readonly identity: string;
  readonly relationship: string;
  readonly redline: string;
  readonly user: string;
  readonly world: string;
}

export interface PersonaSaveInput {
  readonly kind: PersonaKind;
  readonly content: string;
  readonly base_revision: number;
  readonly reason?: string;
}

export interface PersonaActionTransport {
  invoke<Input = unknown, Result = unknown>(request: {
    readonly moduleId: string;
    readonly actionId: string;
    readonly input: Input;
  }): Promise<Result>;
}

export class PersonaClient {
  constructor(private readonly actions: PersonaActionTransport) {}

  static fromRPC(rpc: FaceClientRPC): PersonaClient {
    return new PersonaClient(createModuleActionClient(rpc));
  }

  status(sessionId: string): Promise<PersonaOutcome<PersonaStatus>> {
    return this.invoke(PERSONA_ACTIONS.status, sessionId, {});
  }

  initialize(sessionId: string, initialization: PersonaInitialization, reason?: string): Promise<PersonaOutcome<PersonaWriteOutcome>> {
    return this.invoke(PERSONA_ACTIONS.initialize, sessionId, {
      initialization,
      ...(reason ? { reason } : {}),
    });
  }

  read(sessionId: string, kind: PersonaKind): Promise<PersonaOutcome<PersonaDocument>> {
    return this.invoke(PERSONA_ACTIONS.read, sessionId, { kind });
  }

  save(sessionId: string, input: PersonaSaveInput): Promise<PersonaOutcome<PersonaWriteOutcome>> {
    return this.invoke(PERSONA_ACTIONS.save, sessionId, input);
  }

  listReviews(sessionId: string, kind?: PersonaKind, cursor?: string): Promise<PersonaOutcome<PersonaReviewPage>> {
    return this.invoke(PERSONA_ACTIONS.reviewsList, sessionId, {
      ...(kind ? { kind } : {}),
      ...(cursor ? { cursor } : {}),
      limit: 100,
    });
  }

  decideReview(sessionId: string, reviewId: string, decision: PersonaReviewDecision): Promise<PersonaOutcome<PersonaWriteOutcome>> {
    return this.invoke(PERSONA_ACTIONS.reviewDecide, sessionId, {
      review_id: reviewId,
      decision,
    });
  }

  private invoke<Input extends object, Result>(
    actionId: string,
    sessionId: string,
    input: Input,
  ): Promise<PersonaOutcome<Result>> {
    const normalizedSessionId = sessionId.trim();
    if (!normalizedSessionId) return Promise.reject(new Error('Persona actions require an active session'));
    return this.actions.invoke<Input & { readonly session_id: string }, PersonaOutcome<Result>>({
      moduleId: PERSONA_MODULE_ID,
      actionId,
      input: { session_id: normalizedSessionId, ...input },
    });
  }
}
