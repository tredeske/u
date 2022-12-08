package unet

import "time"

// perform an action when the deadline expires.  allow the deadline to be
// changed or cancelled.
type deadliner_ struct {
	deadlineC chan time.Time
}

func newDeadliner(deadline time.Time, onDeadline func()) (rv *deadliner_) {
	rv = &deadliner_{
		deadlineC: make(chan time.Time, 2),
	}
	go rv.run(deadline, onDeadline)
	return rv
}

func (this *deadliner_) Reset(deadline time.Time) { this.deadlineC <- deadline }
func (this *deadliner_) Cancel()                  { close(this.deadlineC) }

func (this *deadliner_) run(deadline time.Time, onDeadline func()) {
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	for {
		select {
		case newDeadline, ok := <-this.deadlineC:
			if !ok || newDeadline.IsZero() {
				return
			} else if !timer.Stop() {
				<-timer.C // see docs for timer.Reset
			}
			timer.Reset(time.Until(newDeadline))
		case <-timer.C:
			onDeadline()
			return
		}
	}
}
