package subpub

import (
	"sync"

	"go.uber.org/zap"
)

// subscriber holds information about a single subscriber.
type subscriber struct {
	id            string
	handler       MessageHandler
	msgCh         chan interface{} // Channel to send messages to this subscriber
	quitCh        chan struct{}    // Channel to signal this subscriber's goroutine to stop
	once          sync.Once        // Ensures quitCh is closed only once
	wg            *sync.WaitGroup  // Pointer to the main subPub WaitGroup
	parentSubject string           // For logging context
	logger        *zap.SugaredLogger
}

// handleMessages is the goroutine function for a subscriber.
// It listens for messages on msgCh and calls the handler.
// It stops when quitCh is closed.
func (s *subscriber) handleMessages() {
	defer s.wg.Done()
	s.logger.Debugw("Subscriber goroutine started", "subject", s.parentSubject, "subscriberID", s.id)
	defer s.logger.Debugw("Subscriber goroutine stopped", "subject", s.parentSubject, "subscriberID", s.id)

	for {
		select {
		case msg, ok := <-s.msgCh:
			if !ok {
				s.logger.Debugw("Message channel closed, subscriber exiting", "subject", s.parentSubject, "subscriberID", s.id)
				return
			}
			s.handler(msg)
		case <-s.quitCh:
			s.logger.Infow("Quit signal received, draining messages", "subject", s.parentSubject, "subscriberID", s.id)
			// Drain remaining messages from msgCh.
		DrainLoop:
			for {
				select {
				case msg, ok := <-s.msgCh:
					if !ok {
						// msgCh was closed, likely by Unsubscribe or Close path.
						s.logger.Debugw("Message channel closed during drain, subscriber exiting", "subject", s.parentSubject, "subscriberID", s.id)
						return
					}
					s.handler(msg)
				default:
					s.logger.Infow("Message channel drained", "subject", s.parentSubject, "subscriberID", s.id)
					break DrainLoop
				}
			}
			return
		}
	}
}
