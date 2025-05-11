package subpub

// subscription implements the Subscription interface.
type subscription struct {
	parent  *subPub
	subject string
	id      string
}

func (s *subscription) Unsubscribe() {
	s.parent.unsubscribe(s.subject, s.id)
}
