package usync

// mixin to be added to a service goroutine to allow clients to make sync and/or
// async calls to the service.  rather than marshaling args into a struct and
// passing them to the service, a closure is passed to the service to execute.
//
// The closure then has access to the variables it closes on, as well as the
// internal service state, which it can safely manipulate or use because it is
// operating within the service thread context.
//
// When a closure is invoked by the service, an error may be passed to the
// service that there's a problem.  This error is not passed to the caller
// unless the caller capture the error in a different arror variable.
//
// Async calls are fire and forget.  They will be executed in the future by the
// service.
//
// Sync calls wait for the service to perform the invocation before returning.
//
//	type Svc struct {
//		proc: usync.Proc
//		state: string
//	}
//	func(svc *Svc) AsyncSvcCall(arg string) {
//		proc.Async(func() (svcErr error) {
//			// ... do something with arg in Svc context
//			// ... do something with state in Svc context
//			return svcErr // tell service call succeeded or failed
//		})
//	}
//	func(svc *Svc) SyncSvcCall(arg string) (err error) {
//		// this call will not return until service done invoking it
//		proc.Call(func() (svcErr error) {
//			// ... do something with arg in Svc context
//			// ... do something with state in Svc context
//			// ... set err if needed
//			return svcErr       // tell service call succeeded or failed
//		})
//		return
//	}
type Proc struct {
	ProcC chan ProcF // queue of closures to invoke
}

// a closure to invoke on a service.  any error returned is for the service to
// handle
type ProcF func() (svcError error)

func NewProc(backlog int) *Proc {
	return &Proc{ProcC: make(chan ProcF, backlog)}
}

func (this *Proc) Construct(backlog int) {
	this.ProcC = make(chan ProcF, backlog)
}

// fire and forget call
func (this *Proc) Async(closure ProcF) { this.ProcC <- closure }

// wait for service to invoke
func (this *Proc) Call(closure ProcF) (err error) {
	doneC := make(chan struct{}, 1)
	this.ProcC <- func() error {
		defer func() { doneC <- struct{}{} }()
		return closure()
	}
	<-doneC
	return
}

func (this *Proc) Close() { close(this.ProcC) }

// semi-example of a possible use
func (this *Proc) InvokeUntilError() (err error) {
	for f := range this.ProcC {
		err = f()
		if err != nil {
			return
		}
	}
	return
}

// the same pattern can be applied to a service that already has an any chan.
//
// is it just me, or is this impossible to do with go generics?
type ProcAny struct {
	ProcC chan any // add closures to invoke to this
}

func (this *ProcAny) Construct(c chan any) { this.ProcC = c }

func (this *ProcAny) Async(closure ProcF) { this.ProcC <- closure }

func (this *ProcAny) Call(closure ProcF) (err error) {
	doneC := make(chan struct{}, 1)
	this.ProcC <- func() error {
		defer func() { doneC <- struct{}{} }()
		return closure()
	}
	<-doneC
	return
}
