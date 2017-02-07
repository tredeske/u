package usched

import (
	"testing"
	"time"
)

type testSchedulable_ struct {
	times int
}

func (this *testSchedulable_) OnSchedule() {
	this.times++
}

func TestSchedule(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	dummy := &testSchedulable_{}

	err := s.Add("test", "@hourly", dummy)
	if err != nil {
		t.Fatalf("Unable to add job: %s", err)
	}

	s.Remove("test")

	err = s.Add("test", "@every 1s", dummy)
	if nil == err {
		t.Fatalf("Did not get expected error")
	}

	err = s.Add("test", "@every 1m", dummy)
	if nil == err {
		t.Fatalf("Did not get expected error")
	}

	err = s.ValidInterval("0 7 * * *") // daily at 0700
	if err != nil {
		t.Fatalf("Interval should be valid.  err=%s", err)
	}

	s.Min = time.Second

	err = s.Add("test", "@every 1m", dummy)
	if err != nil {
		t.Fatalf("Unable to add job: %s", err)
	}

	//
	// AddFunc
	//
	s.Min = time.Millisecond
	c := make(chan struct{})
	f := func() {
		c <- struct{}{}
	}

	err = s.AddFunc("testFunc", "@every 10ms", f)
	if err != nil {
		t.Fatalf("Unable to add func: %s", err)
	}

	<-c
}

func TestChanJob(t *testing.T) {
	s := NewScheduler()
	s.Min = time.Millisecond
	s.Start()
	defer s.Stop()

	job := &ChanJob{
		Name: "Test",
	}

	err := job.Schedule(s, "@every 1s")
	if err != nil {
		t.Fatalf("Unable to add chan job: %s", err)
	}

	<-job.StartC // await notification to start

	// do some work ...

	job.Close() // notify we're done (in this case, don't invoke us anymore)
}
