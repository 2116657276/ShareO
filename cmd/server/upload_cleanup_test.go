package main

import (
	"context"
	"testing"
	"time"
)

type fakeUploadArtifactCleaner struct {
	keys []string
}

func (f fakeUploadArtifactCleaner) DeleteExpiredUnbound(context.Context, time.Time, int) ([]string, error) {
	return f.keys, nil
}

func TestCleanupExpiredUploadArtifactsRemovesCollectedObjects(t *testing.T) {
	var removed []string
	err := cleanupExpiredUploadArtifacts(
		context.Background(),
		fakeUploadArtifactCleaner{keys: []string{"posts/original/old.jpg", "posts/thumb/old.jpg"}},
		func(_ context.Context, keys []string) error {
			removed = append(removed, keys...)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("cleanupExpiredUploadArtifacts error: %v", err)
	}
	if len(removed) != 2 || removed[0] != "posts/original/old.jpg" || removed[1] != "posts/thumb/old.jpg" {
		t.Fatalf("removed keys = %v", removed)
	}
}
