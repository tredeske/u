package usched

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/ulog"
	"github.com/tredeske/u/usync"

	"gopkg.in/robfig/cron.v2"
)

//
// implement this with whatever you want scheduler to run
//
type Schedulable interface {
	OnSchedule()
}

//
// a func that is schedulable
//
type SchedulableFunc func()

func (this SchedulableFunc) OnSchedule() { this() }

//
// Use this to schedule jobs
//
// Manages job scheduling so that each job is unique by name.
//
// Prevents each job from running at the same time as itself.
//
// Prevents scheduling intervals that are too long or too short
//
// Randomizes interval to spread out the load.
//
// Performs early initialization of jobs, as directed.
//
type Scheduler struct {
	theCron *cron.Cron          // schedules jobs
	lock    sync.Mutex          // for handles map
	running bool                //
	handles map[string]*handle_ // name to handle
	Min     time.Duration       // minimum interval allowed
	Max     time.Duration       // maximum interval allowed
	Init    int                 // max allowed concurrent initializations, or -1
}

// implements cron.Job
type handle_ struct {
	cid          cron.EntryID     // cron handle
	running      usync.AtomicBool // detect already running
	schedulable  Schedulable      // does the work
	interval     string           // how often
	cronInterval string           // how often (based on interval)
	name         string           // unique name
	ancestor     *handle_         // detect ancestor already running
}

//
// Create a Scheduler
//
func NewScheduler() (rv *Scheduler) {

	rv = &Scheduler{
		theCron: cron.New(),
		handles: make(map[string]*handle_),
		Min:     7 * time.Minute,
		Max:     90 * 24 * time.Hour,
		Init:    1,
	}
	return
}

//
// Start the Scheduler
//
func (this *Scheduler) Start() (self *Scheduler) {
	this.running = true
	this.theCron.Start()

	if -1 == this.Init { // no limit - start everything

		log.Printf("sched: starting ALL jobs")
		for _, h := range this.handles {
			go h.Run()
		}

	} else if 0 < this.Init { // start limited number at a time

		log.Printf("sched: starting %d jobs", this.Init)

		initC := make(chan *handle_, 500)

		for i := 0; i < this.Init; i++ { // N goroutines to run jobs
			go func() {
				for h := range initC {
					h.Run()
				}
			}()
		}

		for _, h := range this.handles { // try to load them up
			select {
			case initC <- h:
			default:
			}
		}
		close(initC)
	}
	return this
}

//
// Stop the scheduler
//
func (this *Scheduler) Stop() {
	this.running = false
	this.theCron.Stop()
}

//
// Schedule or reschedule a func as a job
//
func (this *Scheduler) AddFunc(name, interval string, f func()) (err error) {
	return this.Add(name, interval, SchedulableFunc(f))
}

//
// Schedule or reschedule a job
//
func (this *Scheduler) Add(name, interval string, s Schedulable) (err error) {
	if nil == s {
		err = errors.New("No schedulable provided")
		return
	} else if 0 == len(name) {
		err = errors.New("No name provided")
		return
	} else if 0 == len(interval) {
		err = errors.New("No interval provided")
		return
	}

	this.lock.Lock()
	defer this.lock.Unlock()
	ancestor, ok := this.handles[name]
	if ok {
		this.remove(ancestor)
	}

	h := &handle_{
		name:        name,
		interval:    interval,
		schedulable: s,
		ancestor:    ancestor,
	}

	h.cronInterval, err = this.calcInterval(interval)
	if err != nil {
		return
	}

	h.cid, err = this.theCron.AddJob(h.cronInterval, h)
	if err != nil {
		return
	}
	this.handles[name] = h
	log.Printf("sched: Added job %s: interval='%s', cron='%s'",
		name, interval, h.cronInterval)

	if 0 != this.Init && this.running {
		// if cron is started and the dispatch time is too far away,
		// try to put it into the init queue to run right away
		nextT := this.theCron.Entry(h.cid).Next
		if !nextT.IsZero() && nextT.Sub(time.Now()) > time.Minute {
			go h.Run()
		}
	}
	return
}

//
// Cancel a job
//
// Note: if you Remove, then Add the same named job, you might run the risk of
// running the same job concurrently.  It is better in that case to just use Add.
//
func (this *Scheduler) Remove(name string) {
	this.lock.Lock()
	defer this.lock.Unlock()
	if h, ok := this.handles[name]; ok {
		this.remove(h)
	}
}

func (this *Scheduler) remove(h *handle_) {
	this.theCron.Remove(h.cid)
	delete(this.handles, h.name)
	log.Printf("sched: Removed job %s", h.name)
}

//
//
//

func (this *Scheduler) someHourMin() (m, h int) {
	m = rand.Intn(60) // any minute
	h = rand.Intn(12) // from 19:00 til 07:00
	if h >= 7 {
		h += 12
	}
	return
}

//
// validate and randomize interval
//
func (this *Scheduler) ValidInterval(in string) (err error) {
	_, err = this.calcInterval(in)
	return
}

// validate and randomize interval
func (this *Scheduler) calcInterval(in string) (out string, err error) {
	var dur time.Duration
	switch in {
	case "@hourly":
		dur = time.Hour
		m, _ := this.someHourMin()
		out = fmt.Sprintf("0 %d * * * *", m)
	case "@daily", "@nightly":
		dur = 24 * time.Hour
		m, h := this.someHourMin()
		out = fmt.Sprintf("0 %d %d * * *", m, h)
	case "@weekly":
		dur = 7 * 24 * time.Hour
		m, h := this.someHourMin()
		d := rand.Intn(7)
		out = fmt.Sprintf("0 %d %d * * %d", m, h, d)
	case "@monthly":
		dur = 30 * 24 * time.Hour
		m, h := this.someHourMin()
		d := rand.Intn(28) + 1 // month days start at 1, not 0
		out = fmt.Sprintf("0 %d %d %d * *", m, h, d)
	case "@semihourly": // twice an hour
		dur = 30 * time.Minute
		m, _ := this.someHourMin()
		m2 := (m + 30) % 60
		out = fmt.Sprintf("0 %d,%d * * * *", m, m2)
	case "@semidaily": // twice a day
		dur = 12 * time.Hour
		m, h := this.someHourMin()
		h2 := (h + 12) % 24
		out = fmt.Sprintf("0 %d %d,%d * * *", m, h, h2)
	case "@semiweekly": // twice a week
		dur = 3 * 24 * time.Hour
		m, h := this.someHourMin()
		d1 := rand.Intn(4)
		d2 := d1 + 3
		out = fmt.Sprintf("0 %d %d * * %d,%d", m, h, d1, d2)
	case "@semimonthly": // twice a month
		dur = 15 * 24 * time.Hour
		m, h := this.someHourMin()
		d1 := rand.Intn(14) + 1 // month days start at 1, not 0
		d2 := d1 + 14
		out = fmt.Sprintf("0 %d %d %d,%d * *", m, h, d1, d2)
	case "@quarterhourly": // 4x an hour
		dur = 15 * time.Minute
		m, _ := this.someHourMin()
		m2 := (m + 15) % 60
		m3 := (m2 + 15) % 60
		m4 := (m3 + 15) % 60
		out = fmt.Sprintf("0 %d,%d,%d,%d * * * *", m, m2, m3, m4)
	case "@quarterdaily": // 4x a day
		dur = 6 * time.Hour
		m, h := this.someHourMin()
		h2 := (h + 6) % 24
		h3 := (h2 + 6) % 24
		h4 := (h3 + 6) % 24
		out = fmt.Sprintf("0 %d %d,%d,%d,%d * * *", m, h, h2, h3, h4)
	case "@quarterly": // 4x a year
		dur = 90 * 24 * time.Hour
		m, h := this.someHourMin()
		out = fmt.Sprintf("0 %d %d 1 1,4,7,10 *", m, h)
	case "@yearly", "@annually", "@midnight":
		return out, fmt.Errorf("Invalid cron interval: %s", in)
	default:
		parts := strings.Split(in, " ")
		if ("@every" == parts[0] || "@each" == parts[0]) && 2 == len(parts) {
			dur, err = time.ParseDuration(parts[1])
			if err != nil {
				return
			}
			out = "@every " + parts[1]

		} else if "@rate" == parts[0] {
			out, err = this.calcRate(in, parts)
			if err != nil {
				return
			}

		} else if 5 <= len(parts) { // cron format
			i := 0
			if 6 == len(parts) {
				// disallow every second
				if "*" == parts[0] || -1 != strings.IndexRune(parts[0], '-') {
					return "", fmt.Errorf("Cron interval (%s) too short", in)
				}
				i = 1
			}
			minutes := parts[i]
			//hours := parts[i+1]
			// we disallow some cases of every minute, but not all.
			// this just moves the cookie jar a bit out of reach.
			// this is so we can put a more convoluted expression in for
			// test purposes
			//
			if "*" == minutes || "0-59" == minutes { // disallow every minute?
				err = this.validDur(in, time.Minute)
				if err != nil {
					return
				}
			}
			out = in
		} else {
			out = in
		}
	}
	_, err = cron.Parse(out)
	if err != nil {
		return
	}
	err = this.validDur(in, dur)
	return
}

func (this *Scheduler) validDur(in string, dur time.Duration) (err error) {
	if time.Second > this.Min {
		this.Min = time.Second
	}
	if dur < this.Min {
		err = fmt.Errorf("Cron interval (%s) needs to be at least %s", in, this.Min)
	} else if 0 != this.Max && dur > this.Max {
		err = fmt.Errorf("Cron interval (%s) needs to be less than %s", in, this.Max)
	}
	return
}

//
// @rate T per P
// T is a number
// P is one of: hour, day, week, month
//
func (this *Scheduler) calcRate(in string, parts []string) (out string, err error) {
	if 4 != len(parts) {
		err = fmt.Errorf("Interval format must be @rate T per P, is '%s'", in)
		return
	}
	times, err := strconv.Atoi(parts[1])
	if err != nil {
		err = uerr.Chainf(err, "Unable to convert 2nd term of rate")
		return
	} else if 0 >= times {
		err = fmt.Errorf("2nd term of rate must be positive.  Is %d", times)
		return
	}
	switch parts[3] {
	case "hour":
		err = this.validDur(in, time.Hour/time.Duration(times))
		if err != nil {
			return
		}
		m, _ := this.someHourMin()
		out = fmt.Sprintf("0 %d", m)
		fraction := 60 / times
		for i := 1; i < times; i++ {
			m = (m + fraction) % 60
			out += ","
			out += strconv.Itoa(m)
		}
		out += " * * * *"
	case "day":
		err = this.validDur(in, 24*time.Hour/time.Duration(times))
		if err != nil {
			return
		}
		m, h := this.someHourMin()
		out = fmt.Sprintf("0 %d %d", m, h)
		fraction := 24 / times
		for i := 1; i < times; i++ {
			h = (h + fraction) % 24
			out += ","
			out += strconv.Itoa(h)
		}
		out += " * * *"
	case "week":
		err = this.validDur(in, 7*24*time.Hour/time.Duration(times))
		if err != nil {
			return
		}
		m, h := this.someHourMin()
		d := rand.Intn(7)
		out = fmt.Sprintf("0 %d %d * * %d", m, h, d)
		fraction := 7 / times
		for i := 1; i < times; i++ {
			d = (d + fraction) % 7
			out += ","
			out += strconv.Itoa(d)
		}
	case "month":
		err = this.validDur(in, 30*24*time.Hour/time.Duration(times))
		if err != nil {
			return
		}
		m, h := this.someHourMin()
		d := rand.Intn(30) + 1 // month days start at 1, not 0
		out = fmt.Sprintf("0 %d %d %d", m, h, d)
		fraction := 30 / times
		for i := 1; i < times; i++ {
			d = ((d + fraction) % 30) + 1 // 1 based
			out += ","
			out += strconv.Itoa(d)
		}
		out += " * *"
	default:
		err = fmt.Errorf("unsupported rate period in '%s", in)
	}
	return
}

//
// start the job right now in its own goroutine
//
func (this *Scheduler) RunJobAsync(name string) {
	this.lock.Lock()
	j := this.handles[name]
	this.lock.Unlock()
	if nil != j {
		go j.Run()
	}
}

//
// run the job right now in the current goroutine
//
func (this *Scheduler) RunJobSync(name string) {
	this.lock.Lock()
	j := this.handles[name]
	this.lock.Unlock()
	if nil != j {
		j.Run()
	}
}

//
// prevent the scheduler from running the job, returning true if exclusive
// access was achieved
//
func (this *Scheduler) LockJob(name string) (locked bool) {
	this.lock.Lock()
	j := this.handles[name]
	this.lock.Unlock()
	if nil != j {
		locked = j.running.SetUnlessSet()
	}
	return
}

//
// allow the scheduler to run the job
//
func (this *Scheduler) UnlockJob(name string) {
	this.lock.Lock()
	j := this.handles[name]
	this.lock.Unlock()
	if nil != j {
		j.running.Clear()
	}
	return
}

//
//
//

// implement cron.Job
func (this *handle_) Run() {
	if this.running.SetUnlessSet() {
		defer this.running.Clear()

		if nil != this.ancestor {
			if this.ancestor.running.SetUnlessSet() {
				this.ancestor = nil // we can finally run
			} else {
				log.Printf("sched: ancestor of %s already running", this.name)
				return
			}
		}

		ulog.Debugf("sched: running %s", this.name)
		this.schedulable.OnSchedule()

	} else {
		log.Printf("sched: %s already running", this.name)
	}
}
