package aiciv

import (
	"context"
	"time"
)

// ChooseTarget picks a farming target, skipping subjects that recently failed.
// candidates are hostile NPC ids (e.g. "dr1", "dr2"). Falls back to the first candidate if all are blocked.
func ChooseTarget(ctx context.Context, store *Store, agentID string, candidates []string, now time.Time) (string, error) {
	if store == nil || agentID == "" || len(candidates) == 0 {
		return "", ErrInput
	}
	events, err := store.Recall(ctx, agentID, now)
	if err != nil {
		return "", err
	}
	failed := map[string]bool{}
	for _, e := range events {
		if e.Type == "failure" && e.Subject != "" {
			failed[e.Subject] = true
		}
	}
	for _, c := range candidates {
		if c != "" && !failed[c] {
			return c, nil
		}
	}
	// all blocked — take the first anyway (do not stall forever)
	return candidates[0], nil
}

// RecordFailure stores a short-lived failure memory so the agent avoids the same subject for a while.
func RecordFailure(ctx context.Context, store *Store, agentID, subject string, now time.Time) error {
	if store == nil || agentID == "" || subject == "" {
		return ErrInput
	}
	return store.Observe(ctx, agentID, MemoryEvent{
		ID:      "fail-" + subject + "-" + now.Format("20060102T150405"),
		Type:    "failure",
		Subject: subject,
		At:      now,
		Expires: now.Add(30 * time.Minute),
	})
}

// RecordObserved stores a confirmed observation (e.g. started farming a target).
func RecordObserved(ctx context.Context, store *Store, agentID, subject string, now time.Time) error {
	if store == nil || agentID == "" || subject == "" {
		return ErrInput
	}
	return store.Observe(ctx, agentID, MemoryEvent{
		ID:      "obs-" + subject + "-" + now.Format("20060102T150405"),
		Type:    "observed",
		Subject: subject,
		At:      now,
		Expires: now.Add(2 * time.Hour),
	})
}
