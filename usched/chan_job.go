package usched

import "github.com/tredeske/u/usync"

//
// Adapt Schedulable to provide notifications to a chan
//
type ChanJob struct {
	Name       string        // name of job (will be sent in StartC)
	StartC     chan *ChanJob // chan scheduler uses to notify job should start
	StopC      chan struct{} // chan job uses to notify that it is done
	s          *Scheduler    //
	CloseStart bool          // true if we Close StartC
	CloseStop  bool          // true if we Close StopC
}

//
// Schedule this with the Scheduler
//
// You may specify your own chan(s) ahead of time
//
func (this *ChanJob) Schedule(s *Scheduler, interval string) (err error) {
	if nil == this.StartC {
		this.StartC = make(chan *ChanJob) // unbuffered ok
		this.CloseStart = true
	}
	if nil == this.StopC {
		this.StopC = make(chan struct{}) // unbuffered ok
		this.CloseStop = true
	}
	this.s = s
	err = s.Add(this.Name, interval, this)
	return
}

//
// Invoked by Scheduler to run this periodically
//
// implement Schedulable
//
func (this *ChanJob) OnSchedule() {
	defer usync.IgnorePanic()
	this.StartC <- this // ready, steady, go
	<-this.StopC        // wait til done
}

//
// Notify done doing work for this invocation
//
func (this *ChanJob) Done() {
	this.StopC <- struct{}{}
}

//
// Notify we should no longer be invoked
//
func (this *ChanJob) Close() {
	this.s.Remove(this.Name)
	if this.CloseStart {
		close(this.StartC)
		this.CloseStart = false
	}
	if this.CloseStop {
		close(this.StopC)
		this.CloseStop = false
	}
}
