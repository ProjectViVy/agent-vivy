# Queue payload boundary

Enqueue reserves the full future follow_up restoration marker before acknowledging a queued steer. The track appears twice in that marker, so changing steer to follow_up adds eight encoded bytes. Previously an accepted steer near the journal ceiling could fail fallback admission despite fitting its original restoration marker.

The validation uses a copy of the turn with follow_up track. The original turn and turn.queued event retain steer track. No queue, admission, checkpoint, storage, scheduler, or UI behavior changed beyond this size reservation.

Work was isolated on fix/queue-payload-boundary from 245bbc15 in /workspace/agent-vivy-queue-boundary. Root integration owns sourcehash refresh, generated assembly regeneration, complete CI, and packaging. This change adds no release artifact.
