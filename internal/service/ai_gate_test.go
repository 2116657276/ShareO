package service

import "testing"

func TestTryAcquireAISearchSlotRejectsRequestsAboveCapacity(t *testing.T) {
	releases := make([]func(), 0, MaxConcurrentAISearchRequests)
	for i := 0; i < MaxConcurrentAISearchRequests; i++ {
		release, ok := TryAcquireAISearchSlot()
		if !ok {
			t.Fatalf("slot %d was rejected within capacity", i)
		}
		releases = append(releases, release)
	}
	if release, ok := TryAcquireAISearchSlot(); ok {
		release()
		t.Fatal("request above capacity was admitted")
	}
	for _, release := range releases {
		release()
	}
}
