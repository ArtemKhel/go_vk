package subpub

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go_vk/internal/config"

	"go.uber.org/zap"
)

func newSubPub() SubPub {
	l, _ := zap.NewDevelopment()
	logger := l.Sugar()
	c := config.SubPubConfig{
		BufferSize: 10,
	}
	sp := NewSubPub(logger, c)

	return sp
}

func TestBasicSubPub(t *testing.T) {
	sp := newSubPub()

	var receivedMsg interface{}
	var wg sync.WaitGroup
	wg.Add(1)

	sub, err := sp.Subscribe("test-subject", func(msg interface{}) {
		receivedMsg = msg
		wg.Done()
	})

	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	err = sp.Publish("test-subject", "hello world")
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	wg.Wait()

	if receivedMsg != "hello world" {
		t.Errorf("Expected to receive 'hello world', got %v", receivedMsg)
	}

	sub.Unsubscribe()

	// Should fail with ErrSubscriptionNotFound now
	err = sp.Publish("test-subject", "another message")
	//if err != nil && !errors.Is(err, ErrSubscribersNotFound) {
	//	t.Errorf("Expected ErrSubscriptionNotFound, got %v", err)
	//}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = sp.Close(ctx)
	if err != nil {
		t.Fatalf("Failed to close: %v", err)
	}
}

func TestMultipleSubscribers(t *testing.T) {
	sp := newSubPub()

	const subscriberCount = 5
	receivedCount := make([]int, subscriberCount)
	var wg sync.WaitGroup
	wg.Add(subscriberCount)

	subs := make([]Subscription, subscriberCount)
	for i := 0; i < subscriberCount; i++ {
		index := i // Capture the loop variable
		subs[i], _ = sp.Subscribe("test-multiple", func(msg interface{}) {
			receivedCount[index]++
			if receivedCount[index] == 3 { // Each subscriber gets 3 messages
				wg.Done()
			}
		})
	}

	// Publish 3 messages
	for i := 0; i < 3; i++ {
		err := sp.Publish("test-multiple", i)
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}
	}

	wg.Wait()

	// Verify each subscriber received all 3 messages
	for i, count := range receivedCount {
		if count != 3 {
			t.Errorf("Subscriber %d received %d messages, expected 3", i, count)
		}
	}

	// Unsubscribe all
	for _, sub := range subs {
		sub.Unsubscribe()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := sp.Close(ctx)
	if err != nil {
		t.Fatalf("Failed to close: %v", err)
	}
}

func TestSlowSubscriber(t *testing.T) {
	sp := newSubPub()

	// Create a fast subscriber
	fastReceived := 0
	fastWg := sync.WaitGroup{}
	fastWg.Add(1)

	fastSub, _ := sp.Subscribe("test-slow", func(msg interface{}) {
		fastReceived++
		if fastReceived == 5 {
			fastWg.Done()
		}
	})

	// Create a slow subscriber
	slowReceived := 0
	slowWg := sync.WaitGroup{}
	slowWg.Add(1)

	slowSub, _ := sp.Subscribe("test-slow", func(msg interface{}) {
		time.Sleep(100 * time.Millisecond) // Simulate slow processing
		slowReceived++
		if slowReceived == 5 {
			slowWg.Done()
		}
	})

	// Publish 5 messages quickly
	for i := 0; i < 5; i++ {
		err := sp.Publish("test-slow", i)
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}
	}

	// Fast subscriber should complete quickly
	fastWg.Wait()

	if fastReceived != 5 {
		t.Errorf("Fast subscriber received %d messages, expected 5", fastReceived)
	}

	// Wait for slow subscriber to finish
	slowWg.Wait()

	if slowReceived != 5 {
		t.Errorf("Slow subscriber received %d messages, expected 5", slowReceived)
	}

	fastSub.Unsubscribe()
	slowSub.Unsubscribe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := sp.Close(ctx)
	if err != nil {
		t.Fatalf("Failed to close: %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	sp := newSubPub()

	// Create a subscriber with a very slow handler
	_, err := sp.Subscribe("test-ctx", func(msg interface{}) {
		time.Sleep(5 * time.Second) // This is deliberately very slow
	})

	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Publish a message to trigger the slow handler
	err = sp.Publish("test-ctx", "trigger")
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// Create a context with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Close should respect the context timeout
	start := time.Now()
	err = sp.Close(ctx)
	elapsed := time.Since(start)

	// We expect the close to return quickly due to context cancellation
	if elapsed > 200*time.Millisecond {
		t.Errorf("Close took too long: %v", elapsed)
	}

	// We expect an error due to context cancellation
	if err == nil {
		t.Error("Expected error from Close due to context cancellation")
	}
}

func TestFIFOOrder(t *testing.T) {
	sp := newSubPub()

	received := make([]int, 0, 10)
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1)

	_, err := sp.Subscribe("test-fifo", func(msg interface{}) {
		mu.Lock()
		received = append(received, msg.(int))
		if len(received) == 10 {
			wg.Done()
		}
		mu.Unlock()
	})

	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Publish 10 messages in order
	for i := 0; i < 10; i++ {
		err := sp.Publish("test-fifo", i)
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}
	}

	wg.Wait()

	// Verify messages were received in order
	for i := 0; i < 10; i++ {
		if received[i] != i {
			t.Errorf("Expected message %d at position %d, got %d", i, i, received[i])
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = sp.Close(ctx)
	if err != nil {
		t.Fatalf("Failed to close: %v", err)
	}
}

func TestCloseOperations(t *testing.T) {
	sp := newSubPub()

	// First close the SubPub
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := sp.Close(ctx)
	if err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Operations after close should fail
	_, err = sp.Subscribe("test-close", func(msg interface{}) {})
	if err != nil && !errors.Is(err, ErrClosed) {
		t.Errorf("Expected ErrSubPubClosed on Subscribe after Close, got %v", err)
	}

	err = sp.Publish("test-close", "hello")
	if err == nil || !errors.Is(err, ErrClosed) {
		t.Errorf("Expected ErrSubPubClosed on Publish after Close, got %v", err)
	}

	// Calling Close multiple times should be safe
	err = sp.Close(ctx)
	if err != nil {
		t.Errorf("Second Close call should not return error, got: %v", err)
	}
}

func TestUnsubscribe(t *testing.T) {
	sp := newSubPub()

	received := 0
	var wg sync.WaitGroup
	wg.Add(1)

	sub, err := sp.Subscribe("test-unsub", func(msg interface{}) {
		received++
		wg.Done()
	})

	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Publish one message
	err = sp.Publish("test-unsub", "message")
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	wg.Wait()

	if received != 1 {
		t.Errorf("Expected to receive 1 message, got %d", received)
	}

	// Unsubscribe
	sub.Unsubscribe()

	// Publish should not fail if there are no subscribers
	err = sp.Publish("test-unsub", "another message")
	if err != nil {
		t.Errorf("Failde to publish %v", err)
	}

	// Test concurrent unsubscribes (should be safe with sync.Once)
	var unsubWg sync.WaitGroup
	for i := 0; i < 10; i++ {
		unsubWg.Add(1)
		go func() {
			defer unsubWg.Done()
			sub.Unsubscribe() // Should be safe to call multiple times concurrently
		}()
	}
	unsubWg.Wait()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = sp.Close(ctx)
	if err != nil {
		t.Fatalf("Failed to close: %v", err)
	}
}
