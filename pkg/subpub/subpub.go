package subpub

import (
	"context"
	"sync"

	"go_vk/internal/config"

	"github.com/google/uuid" // For unique subscriber IDs
	"go.uber.org/zap"
)

// MessageHandler is a callback function that processes messages delivered to subscribers.
type MessageHandler func(msg interface{})

// Subscription interface allows unsubscribing.
type Subscription interface {
	// Unsubscribe will remove interest in the current subject subscription is for.
	Unsubscribe()
}

// SubPub defines the Publisher-Subscriber interface.
type SubPub interface {
	// Subscribe creates an asynchronous queue subscriber on the given subject.
	Subscribe(subject string, cb MessageHandler) (Subscription, error)
	// Publish publishes the msg argument to the given subject.
	Publish(subject string, msg interface{}) error
	// Close will shut down SubPub.
	// May be blocked by data delivery until the context is canceled.
	Close(ctx context.Context) error
}

const (
	defaultSubscriberBufferSize = 64 // default to use if config value is invalid
)

// subPub implements the SubPub interface.
type subPub struct {
	mu       sync.RWMutex // Protects subjects map and closed flag
	subjects map[string]map[string]*subscriber
	closed   bool
	wg       sync.WaitGroup // Waits for all subscriber goroutines to finish
	logger   *zap.SugaredLogger
	config   config.SubPubConfig
}

// NewSubPub creates a new SubPub instance.
func NewSubPub(logger *zap.SugaredLogger, cfg config.SubPubConfig) SubPub {
	logger.Info("Initializing subpub with config", cfg)
	if logger == nil {
		logger = zap.NewNop().Sugar()
		logger.Warn("No logger provided to NewSubPub, using NopLogger.")
	}

	bufferSize := cfg.BufferSize
	if bufferSize <= 0 {
		logger.Warnw("SubPubConfig.BufferSize is invalid, using default.",
			"configuredSize", cfg.BufferSize,
			"defaultSize", defaultSubscriberBufferSize,
		)
		bufferSize = defaultSubscriberBufferSize
	}

	return &subPub{
		subjects: make(map[string]map[string]*subscriber),
		logger:   logger,
		config:   config.SubPubConfig{BufferSize: bufferSize},
	}
}

// Subscribe creates an asynchronous queue subscriber on the given subject.
func (sp *subPub) Subscribe(subject string, cb MessageHandler) (Subscription, error) {
	if subject == "" {
		return nil, ErrSubjectEmpty
	}
	if cb == nil {
		return nil, ErrHandlerNil
	}

	sp.mu.Lock()
	if sp.closed {
		sp.mu.Unlock()
		return nil, ErrClosed
	}

	subID := uuid.NewString()
	s := &subscriber{
		id:            subID,
		handler:       cb,
		msgCh:         make(chan interface{}, sp.config.BufferSize),
		quitCh:        make(chan struct{}),
		wg:            &sp.wg,
		logger:        sp.logger,
		parentSubject: subject,
	}

	if _, ok := sp.subjects[subject]; !ok {
		sp.subjects[subject] = make(map[string]*subscriber)
	}
	sp.subjects[subject][subID] = s

	sp.mu.Unlock()

	sp.wg.Add(1)
	go s.handleMessages()

	sp.logger.Infow("New subscription created", "subject", subject, "subscriberID", subID, "bufferSize", sp.config.BufferSize)
	return &subscription{parent: sp, subject: subject, id: subID}, nil
}

// Publish publishes the msg argument to the given subject.
func (sp *subPub) Publish(subject string, msg interface{}) error {
	sp.mu.RLock()
	if sp.closed {
		sp.mu.RUnlock()
		return ErrClosed
	}

	subjectSubs, ok := sp.subjects[subject]
	if !ok || len(subjectSubs) == 0 {
		sp.mu.RUnlock()
		return nil // No subscribers for this subject, not an error
	}

	// Copy subscriber channels to a temporary slice to avoid holding lock during send.
	subsToNotify := make([]*subscriber, 0, len(subjectSubs))
	for _, s := range subjectSubs {
		subsToNotify = append(subsToNotify, s)
	}
	sp.mu.RUnlock()

	for _, s := range subsToNotify {
		select {
		case s.msgCh <- msg:
			// Message sent successfully
		case <-s.quitCh:
			// Subscriber is shutting down, don't send.
			sp.logger.Debugw("Publish: subscriber quitting, message not sent", "subject", subject, "subscriberID", s.id)
		default:
			// Subscriber's buffer is full. Message is dropped for this specific subscriber.
			// This ensures one slow subscriber does not block publishing to others.
			sp.logger.Warnw("Publish: subscriber buffer full, message dropped",
				"subject", subject, "subscriberID", s.id, "bufferSize", cap(s.msgCh))
		}
	}
	return nil
}

// unsubscribe is an internal helper called by Subscription.Unsubscribe.
func (sp *subPub) unsubscribe(subject string, subID string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	subjectSubs, subjectExists := sp.subjects[subject]
	if !subjectExists {
		sp.logger.Warnw("Unsubscribe: subject not found", "subject", subject, "subscriberID", subID)
		return
	}

	sub, subscriberExists := subjectSubs[subID]
	if !subscriberExists {
		sp.logger.Warnw("Unsubscribe: subscriber not found", "subject", subject, "subscriberID", subID)
		return
	}

	// Signal the subscriber goroutine to stop and clean up, only once.
	sub.once.Do(func() {
		sp.logger.Infow("Unsubscribing", "subject", subject, "subscriberID", subID)
		close(sub.quitCh)
		// TODO:
		//  some messages may be lost without trace in logs.
		//  - `Publish` gets a ref to subscriber
		//  - `unsubscribe()` happens, `subscriber.handleMessages()` drains queue and exits
		//  - `Publish` sends its message
	})

	delete(sp.subjects[subject], subID)
	if len(sp.subjects[subject]) == 0 {
		delete(sp.subjects, subject)
	}
	sp.logger.Infow("Unsubscribed and removed from map", "subject", subject, "subscriberID", subID)
}

// Close will shut down SubPub.
// May be blocked by data delivery until the context is canceled.
func (sp *subPub) Close(ctx context.Context) error {
	sp.mu.Lock()
	if sp.closed {
		sp.mu.Unlock()
		sp.logger.Info("Close: already closed.")
		return nil
	}
	sp.closed = true
	sp.logger.Info("Close: Shutting down SubPub...")

	// Signal all subscriber goroutines to stop and clear the subjects map
	for subj, subjectSubs := range sp.subjects {
		for id, s := range subjectSubs {
			s.once.Do(func() {
				sp.logger.Debugw("Close: signaling subscriber to quit", "subject", subj, "subscriberID", id)
				close(s.quitCh)
			})
		}
	}
	sp.subjects = make(map[string]map[string]*subscriber)
	sp.mu.Unlock()

	// Wait for all subscriber goroutines to finish, or for context cancellation
	done := make(chan struct{})
	go func() {
		sp.logger.Debug("Close: waiting for all subscriber goroutines to finish...")
		sp.wg.Wait()
		close(done)
		sp.logger.Info("Close: All subscriber goroutines finished.")
	}()

	select {
	case <-done:
		sp.logger.Info("SubPub closed gracefully.")
		return nil
	case <-ctx.Done():
		// Context was cancelled, return immediately
		// Active handlers continue because their quitCh has been closed, and they will drain and exit
		sp.logger.Warnw("SubPub close interrupted by context cancellation. Active handlers will continue to process queued messages.", "error", ctx.Err())
		return ctx.Err()
	}
}
