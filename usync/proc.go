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
// service that there's a problem.
//
// Async calls do not return immediately to the caller, and only have a svcErr
// return.
//
// Sync calls wait for the service to perform the invocation, and return a callErr
// in addition to svcErr.  The callErr is passed to the caller.
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
//		err = proc.Call(func() (callErr, svcErr error) {
//			// ... do something with arg in Svc context
//			// ... do something with state in Svc context
//			return callErr,  // tell caller invoke succeeded or failed
//				svcErr       // tell service call succeeded or failed
//		})
//		return
//	}
type Proc struct {
	ProcC chan func() error // queue of closures to invoke
}

func NewProc(backlog int) *Proc {
	return &Proc{ProcC: make(chan func() error, backlog)}
}

func (this *Proc) Construct(backlog int) {
	this.ProcC = make(chan func() error, backlog)
}

func (this *Proc) Async(closure func() (svcErr error)) { this.ProcC <- closure }

func (this *Proc) Call(closure func() (callErr, svcErr error)) (err error) {
	respC := make(chan error, 1)
	this.ProcC <- func() error {
		var callErr, svcErr error
		defer func() {
			respC <- callErr
		}()
		callErr, svcErr = closure()
		return svcErr
	}
	return <-respC
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

func (this *ProcAny) Async(closure func() (svcErr error)) { this.ProcC <- closure }

func (this *ProcAny) Call(closure func() (callErr, svcErr error)) (err error) {
	respC := make(chan error, 1)
	this.ProcC <- func() error {
		var callErr, svcErr error
		defer func() {
			respC <- callErr
		}()
		callErr, svcErr = closure()
		return svcErr
	}
	return <-respC
}
