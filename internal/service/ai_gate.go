package service

const MaxConcurrentAISearchRequests = 4

var aiSearchSlots = make(chan struct{}, MaxConcurrentAISearchRequests)

func TryAcquireAISearchSlot() (func(), bool) {
	select {
	case aiSearchSlots <- struct{}{}:
		return func() { <-aiSearchSlots }, true
	default:
		return nil, false
	}
}
