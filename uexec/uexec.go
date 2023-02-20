package uexec

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/ustrings"
)

const (
	STDIN  = 0
	STDOUT = 1
	STDERR = 2
)

// Run command, tossing out stdout, capturing stderr, returning error if
// non-zero exit status.
func Sh(args ...string) (err error) {
	c := Child{Args: args}
	err = c.ShToNull()
	if err != nil {
		err = uerr.Chainf(err, "stderr: %s", c.GetStderr())
	}
	return
}

// A child process to be run
type Child struct {
	Args     []string    // [0] is FULL path to command (see exec.LookPath)
	Dir      string      //
	ChildIo  [3]*os.File // child's stdin, stdout, stderr
	ParentIo [3]*os.File // parent's connection to child's stdin, stdout, stderr
	Env      []string    //
	Process  *os.Process
	State    *os.ProcessState   // set when process completes
	Context  context.Context    //
	Cancel   context.CancelFunc //
	errC     chan struct{}      // signal capturing stdout done
	errOut   bytes.Buffer       // captured stderr
}

// create a child
func NewChild(args ...string) (this *Child) {
	return &Child{Args: args}
}

// By default, all env vars are inherited, but you need to all this if you
// set specific env vars and also want to inherit.
//
// if plus is non-empty, then also add these vars
func (this *Child) InheritEnv(plus map[string]string) *Child {
	this.Env = os.Environ()
	for k, v := range plus {
		this.Env = append(this.Env, k+"="+v)
	}
	return this
}

// Add these env variables.  If InheritEnv not called, then no inheritence.
func (this *Child) SetEnv(env map[string]string) *Child {
	for k, v := range env {
		this.Env = append(this.Env, k+"="+v)
	}
	return this
}

// add an env variable.  If InheritEnv not called, then no inheritance of others.
func (this *Child) SetEnvVar(key, value string) *Child {
	this.Env = append(this.Env, key+"="+value)
	return this
}

// change the args
func (this *Child) SetArgs(args ...string) *Child {
	this.Args = args
	return this
}

// Set a timeout
func (this *Child) SetTimeout(t time.Duration) *Child {
	// if 0, then is is probably configured to not have a timeout
	if 0 != t {
		if nil == this.Context {
			this.Context = context.Background()
		}
		this.Context, this.Cancel = context.WithTimeout(this.Context, t)
	}
	return this
}

// set the dir child will exec in
func (this *Child) AtDir(dir string) *Child {
	this.Dir = dir
	return this
}

var devNull_ *os.File = func() *os.File {
	f, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		panic(err)
	}
	return f
}()

// get a string representing the executed command
func (this *Child) CommandLine() string {
	return strings.Join(this.Args, " ")
}

// map the specified fd to /dev/null
func (this *Child) SetDevNull(io int) (err error) {
	switch io {
	case STDIN, STDOUT, STDERR:
		this.ChildIo[io] = devNull_
	default:
		err = errors.New("io must be 0, 1, 2")
	}
	return
}

// map the specified fd to a pipe
func (this *Child) AddPipe(io int) (err error) {
	r, w, err := os.Pipe()
	if err != nil {
		return
	}
	switch io {
	case STDIN:
		this.ChildIo[STDIN] = r
		this.ParentIo[STDIN] = w
	case STDOUT, STDERR:
		this.ChildIo[io] = w
		this.ParentIo[io] = r
	default:
		r.Close()
		w.Close()
		err = errors.New("io must be 0, 1, 2")
	}
	return
}

// cause stdout to be dropped and stderr to be read as stdout
//
// This will change the output that ShTo functions operate on so that they
// operate on stderr instead of stdout.
//
// This will also disable stderr capturing.
func (this *Child) StderrOnly() *Child {
	this.SetDevNull(STDOUT)
	this.AddPipe(STDERR)
	this.ParentIo[STDOUT] = this.ParentIo[STDERR]
	return this
}

// cause stdout and stderr to be combined
func (this *Child) CombineOutput() *Child {
	this.AddPipe(STDOUT)
	this.DupStdout()
	return this
}

// cause stdout and stderr to go to same place by duping stdOUT to stderr
// (stdout must already be set)
func (this *Child) DupStdout() {
	this.ChildIo[STDERR] = this.ChildIo[STDOUT]
	this.ParentIo[STDERR] = this.ParentIo[STDOUT]
}

// cause stderr to be captured when the child runs
func (this *Child) CaptureStderr() (err error) {
	err = this.AddPipe(STDERR)
	if nil == err {
		stderr := this.ParentIo[STDERR]
		errC := make(chan struct{})
		this.errC = errC
		if 0 == this.errOut.Cap() {
			this.errOut.Grow(1024) // see bytes.MinRead
		} else if 0 != this.errOut.Len() {
			this.errOut.Reset()
		}
		go func() {
			nread, err := this.errOut.ReadFrom(stderr)
			if 0 == nread && nil != err {
				err = uerr.Chainf(err,
					"Unable to read stderr (this=%#v) (stderr=%#v)", this, stderr)
				this.errOut.WriteString(err.Error())
			}
			close(errC)
		}()
	}
	return
}

// if stderr capturing is enabled, then return captured stderr output
func (this *Child) GetStderr() (rv string) {
	/*
		if nil == this.State {
			panic("cannot get stderr if process not done")
			//} else if nil == this.errC {
			//	panic("cannot get stderr if stderr capturing not set")
		}
	*/
	if 0 != this.errOut.Len() {
		rv = ustrings.UnsafeBytesToString(this.errOut.Bytes())
	}
	return
}

// close all the child side fd's related to this
func (this *Child) closeChildIo() {
	for i, f := range this.ChildIo {
		if nil != f && devNull_ != f {
			f.Close()
		}
		this.ChildIo[i] = nil
	}
}

// close all the parent side fd's related to this
func (this *Child) CloseParentIo() {
	for i, f := range this.ParentIo {
		if nil != f && devNull_ != f {
			f.Close()
		}
		this.ParentIo[i] = nil
	}
}

// close all the fd's related to this
func (this *Child) Close() {
	this.closeChildIo()
	this.CloseParentIo()
	cancel := this.Cancel
	if nil != cancel {
		cancel()
	}
}

// prep this for reuse
func (this *Child) Reset() *Child {
	this.Close()
	this.Context = nil
	this.Process = nil
	this.State = nil
	this.Dir = ""
	this.errC = nil
	this.errOut.Reset()
	return this
}

// Shell out and run external command, redirecting output to capture lines
// of output and appending them to lines.
//
// By default, stderr will be captured separately, but see CombineOutput.
//
// # If lines is nil, then a new slice is created
//
// # If splitF is nil, then default to terminate lines on newline
//
// If reduceF is !nil, then it will be applied to each line to determine
// inclusion into the result array.  Empty lineOut will not be included.
// If reduceF sets an error, then processing stops immediately after appending
// lineOut.
//
// # The new lines slice with appended lines is returned
//
// If command exits with a non zero status, an error is returned
func (this *Child) ShToArray(
	lines []string,
	splitF bufio.SplitFunc,
	reduceF func(lineIn string) (lineOut string, err error),
) (rv []string, err error) {

	err = this.ShToFunc(

		func(stdout *os.File) (err error) {
			scanner := bufio.NewScanner(stdout)
			if nil != splitF {
				scanner.Split(splitF)
			}
			for scanner.Scan() {
				line := scanner.Text()
				if 0 != len(line) && nil != reduceF {
					line, err = reduceF(line)
				}
				if 0 != len(line) {
					lines = append(lines, line)
				}
				if err != nil {
					break
				}
			}
			return
		})

	rv = lines
	return
}

const ErrBufferOverflow = uerr.Const("Command output overflowed buffer")

// read child's stdout until EOF or bb full, returning a slice of the output
//
// By default, stderr will be captured separately, but see CombineOutput.
//
// if command exits with a non zero status, an error is returned
func (this *Child) ShToBytes(bb []byte) (rv []byte, err error) {

	err = this.ShToFunc(

		func(stdout *os.File) (err error) {
			nread, err := io.ReadFull(stdout, bb)
			if err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					rv = bb[:nread]
					err = nil
				}
			} else {
				err = ErrBufferOverflow
			}
			return
		})

	return
}

// Append stdout output to provided buffer.
//
// By default, stderr will be captured separately, but see CombineOutput.
//
// if command exits with a non zero status, an error is returned
func (this *Child) ShToBuff(bb *bytes.Buffer) (err error) {

	err = this.ShToFunc(

		func(stdout *os.File) (err error) {
			nread, err := bb.ReadFrom(stdout)
			if err != nil && 0 != nread {
				err = nil
			}
			return
		})

	return
}

// Run command capturing output in string
func (this *Child) ShToString() (rv string, err error) {
	var bb bytes.Buffer
	err = this.ShToBuff(&bb)
	if 0 != bb.Len() {
		rv = ustrings.UnsafeBytesToString(bb.Bytes())
	}
	return
}

// shell out and run external command, redirecting output to be read by f()
// if command exits with a non zero status, an error is returned
//
// By default, stderr will be captured separately, but see CombineOutput.
//
// stderr output will NOT be added to the returned error.  Use GetStderr
// if stderr capturing is set.
//
// if StderrOnly() is set, then the stdout output will be ignored and
// stderr will be passed to f().
func (this *Child) ShToFunc(f func(stdout *os.File) error) (err error) {

	if nil == this.ChildIo[STDERR] {
		err = this.CaptureStderr()
		if err != nil {
			this.Close()
			return
		}
	}
	if nil == this.ChildIo[STDOUT] {
		err = this.AddPipe(STDOUT)
		if err != nil {
			this.Close()
			return
		}
	}
	if nil == this.ChildIo[STDIN] {
		this.SetDevNull(STDIN)
	}

	err = this.Start()
	if err != nil {
		this.Close()
		return
	}

	if nil != f {
		err = f(this.ParentIo[STDOUT])
	}

	werr := this.Wait()
	if werr != nil {
		this.Close()
		if nil == err {
			err = werr
		}
	}

	if nil == err {
		err = this.exitError()
	}
	return
}

// shell out and run external command, capturing stderr if necessary,
// discarding stdout output.
//
// if command exits with a non zero status, an error is returned
func (this *Child) ShToNull() (err error) {

	if nil == this.ChildIo[STDOUT] {
		this.SetDevNull(STDOUT)
	}
	return this.ShToFunc(nil)
}

// start a command concurrently
func (this *Child) Start() (err error) {
	if nil != this.Process {
		return errors.New("uexec: already started")
	}
	this.State = nil

	cmd := this.Args[0]
	if '/' != cmd[0] && '.' != cmd[0] {
		cmd, err = exec.LookPath(cmd)
		if err != nil {
			return
		}
	}
	proc, err := os.StartProcess(cmd, this.Args,
		&os.ProcAttr{
			Dir:   this.Dir,
			Files: this.ChildIo[:],
			Env:   this.Env,
		})
	this.closeChildIo() // we no longer need these - they're the childs

	ctx := this.Context
	if nil != proc && nil != ctx {
		go func() {
			<-ctx.Done()
			if nil == this.State {
				//log.Printf("killing %#v", proc)
				proc.Kill()
			}
		}()
	}
	this.Process = proc
	return
}

// wait for a Start()ed command to finish
func (this *Child) Wait() (err error) {
	if nil != this.State {
		return
	} else if nil == this.Process {
		return errors.New("uexec: not started")
	}
	this.State, err = this.Process.Wait()
	if nil != this.errC {
		<-this.errC
	}
	this.CloseParentIo()
	cancel := this.Cancel
	if nil != cancel {
		cancel()
	}
	return
}

// get the exit status of the just completed command and if it is not 0 (zero)
// then create a human readable error message
func (this *Child) exitError() (err error) {
	exitStatus, err := this.Status()
	if nil == err && 0 != exitStatus {
		err = fmt.Errorf("Command failed. exit=%d\n\tcmd=%s",
			exitStatus, this.CommandLine())
	}
	return
}

// get the exit status of the completed command
func (this *Child) Status() (exitCode int, err error) {
	if nil == this.State {
		err = errors.New("uexec: not waited for")
		return
	}

	// This works on both Unix and Windows. Although package
	// syscall is generally platform dependent, WaitStatus is
	// defined for both Unix and Windows and in both cases has
	// an ExitStatus() method with the same signature.
	if status, ok := this.State.Sys().(syscall.WaitStatus); ok {
		exitCode = status.ExitStatus()
	} else {
		err = errors.New("uexec: unable to obtain exit code")
	}
	return
}
