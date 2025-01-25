package usftp

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"flag"
	"io"
	"io/fs"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync"
	"testing"
	"testing/quick"
	"time"

	"github.com/pkg/sftp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	// assert that *fsClient_ implements FS
	_ FS = new(fsClient_)

	// assert that *File implements io.ReadWriteCloser
	_ io.ReadWriteCloser = new(File)
)

const (
	readOnly_                = true
	readWrite_               = false
	nodelay_   time.Duration = 0

	debuglevel = "ERROR" // set to "DEBUG" for debugging
)

func skipIfWindows(t testing.TB) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows")
	}
}

var (
	testGoServer_ bool // if true, test using Go sftp server, not OS one
	//testIntegration = flag.Bool("integration", true,
	//	"perform integration tests against sftp server process")

	testSftpServer_ string // path to OS sftp server
)

func TestMain(m *testing.M) {

	flag.BoolVar(&testGoServer_, "use-go-server", true,
		"test against Go sftp server instead of OS sftp server")

	lookSftpServer := []string{
		"/usr/libexec/openssh/sftp-server",
		"/usr/libexec/sftp-server",
		"/usr/lib/openssh/sftp-server",
		"/usr/lib/ssh/sftp-server",
		`C:\Program Files\Git\usr\lib\ssh\sftp-server.exe`,
	}
	sftpServer, _ := exec.LookPath("sftp-server")
	if 0 == len(sftpServer) {
		for _, location := range lookSftpServer {
			if _, err := os.Stat(location); err == nil {
				sftpServer = location
				break
			}
		}
	}
	flag.StringVar(&testSftpServer_, "sftp", sftpServer,
		"location of the OS sftp server binary")

	flag.Parse()

	os.Exit(m.Run())
}

type delayedWrite struct {
	t time.Time
	b []byte
}

// delayedWriter wraps a writer and artificially delays the write. This is
// meant to mimic connections with various latencies. Error's returned from the
// underlying writer will panic so this should only be used over reliable
// connections.
type delayedWriter struct {
	closed chan struct{}

	mu      sync.Mutex
	ch      chan delayedWrite
	closing chan struct{}
}

func newDelayedWriter(w io.WriteCloser, delay time.Duration) io.WriteCloser {
	dw := &delayedWriter{
		ch:      make(chan delayedWrite, 128),
		closed:  make(chan struct{}),
		closing: make(chan struct{}),
	}

	go func() {
		defer close(dw.closed)
		defer w.Close()

		for writeMsg := range dw.ch {
			time.Sleep(time.Until(writeMsg.t.Add(delay)))

			n, err := w.Write(writeMsg.b)
			if err != nil {
				panic("write error")
			}

			if n < len(writeMsg.b) {
				panic("showrt write")
			}
		}
	}()

	return dw
}

func (dw *delayedWriter) Write(b []byte) (int, error) {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	write := delayedWrite{
		t: time.Now(),
		b: append([]byte(nil), b...),
	}

	select {
	case <-dw.closing:
		return 0, errors.New("delayedWriter is closing")
	case dw.ch <- write:
	}

	return len(b), nil
}

func (dw *delayedWriter) Close() error {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	select {
	case <-dw.closing:
	default:
		close(dw.ch)
		close(dw.closing)
	}

	<-dw.closed
	return nil
}

// netPipe provides a pair of io.ReadWriteClosers connected to each other.
// The functions is identical to os.Pipe with the exception that netPipe
// provides the Read/Close guarantees that os.File derrived pipes do not.
func netPipe(t testing.TB) (io.ReadWriteCloser, io.ReadWriteCloser) {
	type result struct {
		net.Conn
		error
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	closeListener := make(chan struct{}, 1)
	closeListener <- struct{}{}

	ch := make(chan result, 1)
	go func() {
		conn, err := l.Accept()
		ch <- result{conn, err}

		if _, ok := <-closeListener; ok {
			err = l.Close()
			if err != nil {
				t.Error(err)
			}
			close(closeListener)
		}
	}()

	c1, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		if _, ok := <-closeListener; ok {
			l.Close()
			close(closeListener)
		}
		t.Fatal(err)
	}

	r := <-ch
	if r.error != nil {
		t.Fatal(err)
	}

	return c1, r.Conn
}

func testClientGoSvr(
	t testing.TB,
	readonly bool,
	delay time.Duration,
	opts ...ClientOption,
) (*Client, *exec.Cmd) {

	c1, c2 := netPipe(t)

	options := []sftp.ServerOption{sftp.WithDebug(os.Stderr)}
	if readonly {
		options = append(options, sftp.ReadOnly())
	}

	server, err := sftp.NewServer(c1, options...)
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve()

	var wr io.WriteCloser = c2
	if delay > nodelay_ {
		wr = newDelayedWriter(wr, delay)
	}

	client, err := NewClientPipe(c2, wr, opts...)
	if err != nil {
		t.Fatal(err)
	}

	// dummy command...
	return client, exec.Command("true")
}

// testClient returns a *Client connected to a locally running sftp-server
// the *exec.Cmd returned must be defer Wait'd.
func testClient(
	t testing.TB,
	readonly bool,
	delay time.Duration,
	opts ...ClientOption,
) (*Client, *exec.Cmd) {
	//if !*testIntegration {
	//	t.Skip("skipping integration test")
	//}

	if testGoServer_ {
		return testClientGoSvr(t, readonly, delay, opts...)
	}

	// log to stderr, read only
	cmd := exec.Command(testSftpServer_, "-e", "-R", "-l", debuglevel)
	if !readonly { // log to stderr
		cmd = exec.Command(testSftpServer_, "-e", "-l", debuglevel)
	}

	cmd.Stderr = os.Stdout

	pw, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	if delay > nodelay_ {
		pw = newDelayedWriter(pw, delay)
	}

	pr, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		t.Skipf("could not start sftp-server process: %v", err)
	}

	sftp, err := NewClientPipe(pr, pw, opts...)
	if err != nil {
		t.Fatal(err)
	}

	return sftp, cmd
}

func TestNewClient(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()

	if err := sftp.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClientLstat(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := os.CreateTemp("", "sftptest-lstat")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	want, err := os.Lstat(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	got, err := sftp.Lstat(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	if !sameFile(want, got.AsFileInfo(f.Name())) {
		t.Fatalf("Lstat(%q): want %#v, got %#v", f.Name(), want, got)
	}
}

func TestClientLstatIsNotExist(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := os.CreateTemp("", "sftptest-lstatisnotexist")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	os.Remove(f.Name())

	if _, err := sftp.Lstat(f.Name()); !os.IsNotExist(err) {
		t.Errorf("os.IsNotExist(%v) = false, want true", err)
	}
}

func TestClientMkdir(t *testing.T) {
	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	dir, err := os.MkdirTemp("", "sftptest-mkdir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	sub := path.Join(dir, "mkdir1")
	if err := sftp.Mkdir(sub); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(sub); err != nil {
		t.Fatal(err)
	}
}
func TestClientMkdirAll(t *testing.T) {
	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	dir, err := os.MkdirTemp("", "sftptest-mkdirall")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	sub := path.Join(dir, "mkdir1", "mkdir2", "mkdir3")
	if err := sftp.MkdirAll(sub); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(sub)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatalf("Expected mkdirall to create dir at: %s", sub)
	}
}

func TestClientOpen(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := os.CreateTemp("", "sftptest-open")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	got, err := sftp.OpenRead(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if err := got.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClientOpenIsNotExist(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	if _, err := sftp.OpenRead("/doesnt/exist"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("os.IsNotExist(%v) = false, want true", err)
	}
}

func TestClientStatIsNotExist(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	if _, err := sftp.Stat("/doesnt/exist"); !os.IsNotExist(err) {
		t.Errorf("os.IsNotExist(%v) = false, want true", err)
	}
}

const seekBytes = 128 * 1024

type seek struct {
	offset int64
}

func (s seek) Generate(r *rand.Rand, _ int) reflect.Value {
	s.offset = int64(r.Int31n(seekBytes))
	return reflect.ValueOf(s)
}

func (s seek) set(t *testing.T, r io.ReadSeeker) {
	if _, err := r.Seek(s.offset, io.SeekStart); err != nil {
		t.Fatalf("error while seeking with %+v: %v", s, err)
	}
}

func (s seek) current(t *testing.T, r io.ReadSeeker) {
	const mid = seekBytes / 2

	skip := s.offset / 2
	if s.offset > mid {
		skip = -skip
	}

	if _, err := r.Seek(mid, io.SeekStart); err != nil {
		t.Fatalf("error seeking to midpoint with %+v: %v", s, err)
	}
	if _, err := r.Seek(skip, io.SeekCurrent); err != nil {
		t.Fatalf("error seeking from %d with %+v: %v", mid, s, err)
	}
}

func (s seek) end(t *testing.T, r io.ReadSeeker) {
	if _, err := r.Seek(-s.offset, io.SeekEnd); err != nil {
		t.Fatalf("error seeking from end with %+v: %v", s, err)
	}
}

func TestClientSeek(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	fOS, err := os.CreateTemp("", "sftptest-seek")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(fOS.Name())
	defer fOS.Close()

	fSFTP, err := sftp.OpenRead(fOS.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer fSFTP.Close()

	writeN(t, fOS, seekBytes)

	if err := quick.CheckEqual(
		func(s seek) (string, int64) { s.set(t, fOS); return readHash(t, fOS) },
		func(s seek) (string, int64) { s.set(t, fSFTP); return readHash(t, fSFTP) },
		nil,
	); err != nil {
		t.Errorf("Seek: expected equal absolute seeks: %v", err)
	}

	if err := quick.CheckEqual(
		func(s seek) (string, int64) { s.current(t, fOS); return readHash(t, fOS) },
		func(s seek) (string, int64) { s.current(t, fSFTP); return readHash(t, fSFTP) },
		nil,
	); err != nil {
		t.Errorf("Seek: expected equal seeks from middle: %v", err)
	}

	if err := quick.CheckEqual(
		func(s seek) (string, int64) { s.end(t, fOS); return readHash(t, fOS) },
		func(s seek) (string, int64) { s.end(t, fSFTP); return readHash(t, fSFTP) },
		nil,
	); err != nil {
		t.Errorf("Seek: expected equal seeks from end: %v", err)
	}
}

func TestClientCreate(t *testing.T) {
	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := os.CreateTemp("", "sftptest-create")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	defer os.Remove(f.Name())

	f2, err := sftp.Create(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()
}

func TestClientAppend(t *testing.T) {
	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := os.CreateTemp("", "sftptest-append")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	defer os.Remove(f.Name())

	f2, err := sftp.Open(f.Name(), os.O_RDWR|os.O_APPEND)
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()
}

func TestClientCreateFailed(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := os.CreateTemp("", "sftptest-createfailed")
	require.NoError(t, err)

	defer f.Close()
	defer os.Remove(f.Name())

	f2, err := sftp.Create(f.Name())
	require.True(t, errors.Is(err, fs.ErrPermission))
	if err == nil {
		f2.Close()
	}
}

func TestClientFileName(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := os.CreateTemp("", "sftptest-filename")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f2, err := sftp.OpenRead(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()

	if got, want := f2.Name(), f.Name(); got != want {
		t.Fatalf("Name: got %q want %q", want, got)
	}
}

func TestClientFileStat(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := os.CreateTemp("", "sftptest-filestat")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	want, err := os.Lstat(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	f2, err := sftp.OpenRead(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()

	got, err := f2.Stat()
	if err != nil {
		t.Fatal(err)
	}

	if !sameFile(want, got.AsFileInfo(f2.Name())) {
		t.Fatalf("Lstat(%q): want %#v, got %#v", f.Name(), want, got)
	}
}

func TestClientStatLink(t *testing.T) {
	skipIfWindows(t) // Windows does not support links.

	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := os.CreateTemp("", "sftptest-statlink")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	realName := f.Name()
	linkName := f.Name() + ".softlink"

	// create a symlink that points at sftptest
	if err := os.Symlink(realName, linkName); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(linkName)

	// compare Lstat of links
	wantLstat, err := os.Lstat(linkName)
	if err != nil {
		t.Fatal(err)
	}
	wantStat, err := os.Stat(linkName)
	if err != nil {
		t.Fatal(err)
	}

	gotLstat, err := sftp.Lstat(linkName)
	if err != nil {
		t.Fatal(err)
	}
	gotStat, err := sftp.Stat(linkName)
	if err != nil {
		t.Fatal(err)
	}

	// check that stat is not lstat from os package
	if sameFile(wantLstat, wantStat) {
		t.Fatalf("Lstat / Stat(%q): both %#v %#v", f.Name(), wantLstat, wantStat)
	}

	// compare Lstat of links
	if !sameFile(wantLstat, gotLstat.AsFileInfo(linkName)) {
		t.Fatalf("Lstat(%q): want %#v, got %#v", f.Name(), wantLstat, gotLstat)
	}

	// compare Stat of links
	if !sameFile(wantStat, gotStat.AsFileInfo(linkName)) {
		t.Fatalf("Stat(%q): want %#v, got %#v", f.Name(), wantStat, gotStat)
	}

	// check that stat is not lstat
	if sameFile(gotLstat.AsFileInfo(linkName), gotStat.AsFileInfo(linkName)) {
		t.Fatalf("Lstat / Stat(%q): both %#v %#v", f.Name(), gotLstat, gotStat)
	}
}

func TestClientRemove(t *testing.T) {
	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := os.CreateTemp("", "sftptest-remove")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	if err := sftp.Remove(f.Name()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(f.Name()); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestClientRemoveAll(t *testing.T) {
	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "sftptest-removeAll")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a directory tree
	dir1, err := os.MkdirTemp(tempDir, "foo")
	if err != nil {
		t.Fatal(err)
	}
	dir2, err := os.MkdirTemp(dir1, "bar")
	if err != nil {
		t.Fatal(err)
	}

	// Create some files within the directory tree
	file1 := tempDir + "/file1.txt"
	file2 := dir1 + "/file2.txt"
	file3 := dir2 + "/file3.txt"
	err = os.WriteFile(file1, []byte("File 1"), 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	err = os.WriteFile(file2, []byte("File 2"), 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	err = os.WriteFile(file3, []byte("File 3"), 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Call the function to delete the files recursively
	err = sftp.RemoveAll(tempDir)
	if err != nil {
		t.Fatalf("Failed to delete files recursively: %v", err)
	}

	// Check if the directories and files have been deleted
	if _, err := os.Stat(dir1); !os.IsNotExist(err) {
		t.Errorf("Directory %s still exists", dir1)
	}
	if _, err := os.Stat(dir2); !os.IsNotExist(err) {
		t.Errorf("Directory %s still exists", dir2)
	}
	if _, err := os.Stat(file1); !os.IsNotExist(err) {
		t.Errorf("File %s still exists", file1)
	}
	if _, err := os.Stat(file2); !os.IsNotExist(err) {
		t.Errorf("File %s still exists", file2)
	}
	if _, err := os.Stat(file3); !os.IsNotExist(err) {
		t.Errorf("File %s still exists", file3)
	}
}

func TestClientRemoveDir(t *testing.T) {
	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	dir, err := os.MkdirTemp("", "sftptest-removedir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if err := sftp.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestClientRemoveFailed(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := os.CreateTemp("", "sftptest-removefailed")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	if err := sftp.Remove(f.Name()); err == nil {
		t.Fatalf("Remove(%v): want: permission denied, got %v", f.Name(), err)
	}
	if _, err := os.Lstat(f.Name()); err != nil {
		t.Fatal(err)
	}
}

func TestClientRename(t *testing.T) {
	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	dir, err := os.MkdirTemp("", "sftptest-rename")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	f, err := os.Create(filepath.Join(dir, "old"))
	require.NoError(t, err)
	f.Close()

	f2 := filepath.Join(dir, "new")
	if err := sftp.Rename(f.Name(), f2); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(f.Name()); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if _, err := os.Lstat(f2); err != nil {
		t.Fatal(err)
	}
}

func TestClientPosixRename(t *testing.T) {
	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	dir, err := os.MkdirTemp("", "sftptest-posixrename")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	f, err := os.Create(filepath.Join(dir, "old"))
	require.NoError(t, err)
	f.Close()

	f2 := filepath.Join(dir, "new")
	if err := sftp.PosixRename(f.Name(), f2); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(f.Name()); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if _, err := os.Lstat(f2); err != nil {
		t.Fatal(err)
	}
}

func TestClientGetwd(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	lwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rwd, err := sftp.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(rwd) {
		t.Fatalf("Getwd: wanted absolute path, got %q", rwd)
	}
	if filepath.ToSlash(lwd) != filepath.ToSlash(rwd) {
		t.Fatalf("Getwd: want %q, got %q", lwd, rwd)
	}
}

func TestClientReadLink(t *testing.T) {
	if runtime.GOOS == "windows" && testGoServer_ {
		// os.Symlink requires privilege escalation.
		t.Skip()
	}

	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	dir, err := os.MkdirTemp("", "sftptest-readlink")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	f, err := os.Create(filepath.Join(dir, "file"))
	require.NoError(t, err)
	f.Close()

	f2 := filepath.Join(dir, "symlink")
	if err := os.Symlink(f.Name(), f2); err != nil {
		t.Fatal(err)
	}
	if rl, err := sftp.ReadLink(f2); err != nil {
		t.Fatal(err)
	} else if rl != f.Name() {
		t.Fatalf("unexpected link target: %v, not %v", rl, f.Name())
	}
}

func TestClientLink(t *testing.T) {
	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	dir, err := os.MkdirTemp("", "sftptest-link")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	f, err := os.Create(filepath.Join(dir, "file"))
	require.NoError(t, err)
	data := []byte("linktest")
	_, err = f.Write(data)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}

	f2 := filepath.Join(dir, "link")
	if err := sftp.Link(f.Name(), f2); err != nil {
		t.Fatal(err)
	}
	if st2, err := sftp.Stat(f2); err != nil {
		t.Fatal(err)
	} else if int(st2.Size) != len(data) {
		t.Fatalf("unexpected link size: %v, not %v", st2.Size, len(data))
	}
}

func TestClientSymlink(t *testing.T) {
	if runtime.GOOS == "windows" && testGoServer_ {
		// os.Symlink requires privilege escalation.
		t.Skip()
	}

	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	dir, err := os.MkdirTemp("", "sftptest-symlink")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	f, err := os.Create(filepath.Join(dir, "file"))
	require.NoError(t, err)
	f.Close()

	f2 := filepath.Join(dir, "symlink")
	if err := sftp.Symlink(f.Name(), f2); err != nil {
		t.Fatal(err)
	}
	if rl, err := sftp.ReadLink(f2); err != nil {
		t.Fatal(err)
	} else if rl != f.Name() {
		t.Fatalf("unexpected link target: %v, not %v", rl, f.Name())
	}
}

func TestClientChmod(t *testing.T) {
	skipIfWindows(t) // No UNIX permissions.
	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := os.CreateTemp("", "sftptest-chmod")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	if err := sftp.Chmod(f.Name(), 0531); err != nil {
		t.Fatal(err)
	}
	if stat, err := os.Stat(f.Name()); err != nil {
		t.Fatal(err)
	} else if stat.Mode()&os.ModePerm != 0531 {
		t.Fatalf("invalid perm %o\n", stat.Mode())
	}

	sf, err := sftp.OpenRead(f.Name())
	require.NoError(t, err)
	require.NoError(t, sf.Chmod(0500))
	sf.Close()

	stat, err := os.Stat(f.Name())
	require.NoError(t, err)
	require.EqualValues(t, 0500, stat.Mode())
}

func TestClientChmodReadonly(t *testing.T) {
	skipIfWindows(t) // No UNIX permissions.
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := os.CreateTemp("", "sftptest-chmodreadonly")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	if err := sftp.Chmod(f.Name(), 0531); err == nil {
		t.Fatal("expected error")
	}
}

func TestClientSetuid(t *testing.T) {
	skipIfWindows(t) // No UNIX permissions.
	if testGoServer_ {
		t.Skipf("skipping (using Go server)")
	}

	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := os.CreateTemp("", "sftptest-setuid")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	const allPerm = os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky |
		os.FileMode(s_ISUID|s_ISGID|s_ISVTX)

	for _, c := range []struct {
		goPerm    os.FileMode
		posixPerm uint32
	}{
		{os.ModeSetuid, s_ISUID},
		{os.ModeSetgid, s_ISGID},
		{os.ModeSticky, s_ISVTX},
		{os.ModeSetuid | os.ModeSticky, s_ISUID | s_ISVTX},
	} {
		goPerm := 0700 | c.goPerm
		posixPerm := 0700 | c.posixPerm

		err = sftp.Chmod(f.Name(), goPerm)
		require.NoError(t, err)

		info, err := sftp.Stat(f.Name())
		require.NoError(t, err)
		require.Equal(t, goPerm, info.OsFileMode()&allPerm)

		err = sftp.Chmod(f.Name(), 0700) // Reset funny bits.
		require.NoError(t, err)

		// For historical reasons, we also support literal POSIX mode bits in
		// Chmod. Stat should still translate these to Go os.FileMode bits.
		err = sftp.Chmod(f.Name(), os.FileMode(posixPerm))
		require.NoError(t, err)

		info, err = sftp.Stat(f.Name())
		require.NoError(t, err)
		require.Equal(t, goPerm, info.OsFileMode()&allPerm)
	}
}

func TestClientChown(t *testing.T) {
	skipIfWindows(t) // No UNIX permissions.
	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	usr, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	if usr.Uid != "0" {
		t.Log("must be root to run chown tests")
		t.Skip()
	}

	chownto, err := user.Lookup("daemon") // seems common-ish...
	if err != nil {
		t.Fatal(err)
	}
	toUID, err := strconv.Atoi(chownto.Uid)
	if err != nil {
		t.Fatal(err)
	}
	toGID, err := strconv.Atoi(chownto.Gid)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.CreateTemp("", "sftptest-chown")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	before, err := exec.Command("ls", "-nl", f.Name()).Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := sftp.Chown(f.Name(), toUID, toGID); err != nil {
		t.Fatal(err)
	}
	after, err := exec.Command("ls", "-nl", f.Name()).Output()
	if err != nil {
		t.Fatal(err)
	}

	spaceRegex := regexp.MustCompile(`\s+`)

	beforeWords := spaceRegex.Split(string(before), -1)
	if beforeWords[2] != "0" {
		t.Fatalf("bad previous user? should be root")
	}
	afterWords := spaceRegex.Split(string(after), -1)
	if afterWords[2] != chownto.Uid || afterWords[3] != chownto.Gid {
		t.Fatalf("bad chown: %#v", afterWords)
	}
	t.Logf("before: %v", string(before))
	t.Logf(" after: %v", string(after))
}

func TestClientChownReadonly(t *testing.T) {
	skipIfWindows(t) // No UNIX permissions.
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	usr, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	if usr.Uid != "0" {
		t.Log("must be root to run chown tests")
		t.Skip()
	}

	chownto, err := user.Lookup("daemon") // seems common-ish...
	if err != nil {
		t.Fatal(err)
	}
	toUID, err := strconv.Atoi(chownto.Uid)
	if err != nil {
		t.Fatal(err)
	}
	toGID, err := strconv.Atoi(chownto.Gid)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.CreateTemp("", "sftptest-chownreadonly")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	if err := sftp.Chown(f.Name(), toUID, toGID); err == nil {
		t.Fatal("expected error")
	}
}

func TestClientChtimes(t *testing.T) {
	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := os.CreateTemp("", "sftptest-chtimes")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	atime := time.Date(2013, 2, 23, 13, 24, 35, 0, time.UTC)
	mtime := time.Date(1985, 6, 12, 6, 6, 6, 0, time.UTC)
	if err := sftp.Chtimes(f.Name(), atime, mtime); err != nil {
		t.Fatal(err)
	}
	if stat, err := os.Stat(f.Name()); err != nil {
		t.Fatal(err)
	} else if stat.ModTime().Sub(mtime) != 0 {
		t.Fatalf("incorrect mtime: %v vs %v", stat.ModTime(), mtime)
	}
}

func TestClientChtimesReadonly(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := os.CreateTemp("", "sftptest-chtimesreadonly")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	atime := time.Date(2013, 2, 23, 13, 24, 35, 0, time.UTC)
	mtime := time.Date(1985, 6, 12, 6, 6, 6, 0, time.UTC)
	if err := sftp.Chtimes(f.Name(), atime, mtime); err == nil {
		t.Fatal("expected error")
	}
}

func TestClientTruncate(t *testing.T) {
	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := os.CreateTemp("", "sftptest-truncate")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	fname := f.Name()

	if n, err := f.Write([]byte("hello world")); n != 11 || err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := sftp.Truncate(fname, 5); err != nil {
		t.Fatal(err)
	}
	if stat, err := os.Stat(fname); err != nil {
		t.Fatal(err)
	} else if stat.Size() != 5 {
		t.Fatalf("unexpected size: %d", stat.Size())
	}
}

func TestClientTruncateReadonly(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := os.CreateTemp("", "sftptest-truncreadonly")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	fname := f.Name()

	if n, err := f.Write([]byte("hello world")); n != 11 || err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := sftp.Truncate(fname, 5); err == nil {
		t.Fatal("expected error")
	}
	if stat, err := os.Stat(fname); err != nil {
		t.Fatal(err)
	} else if stat.Size() != 11 {
		t.Fatalf("unexpected size: %d", stat.Size())
	}
}

func sameFile(want, got os.FileInfo) bool {
	_, wantName := filepath.Split(want.Name())
	_, gotName := filepath.Split(got.Name())
	return wantName == gotName &&
		want.Size() == got.Size()
}

func TestClientReadSimple(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	d, err := os.MkdirTemp("", "sftptest-readsimple")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)

	f, err := os.CreateTemp(d, "read-test")
	if err != nil {
		t.Fatal(err)
	}
	fname := f.Name()
	f.Write([]byte("hello"))
	f.Close()

	f2, err := sftp.OpenRead(fname)
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()
	stuff := make([]byte, 32)
	n, err := f2.Read(stuff)
	if err != nil && err != io.EOF {
		t.Fatalf("err: %v", err)
	}
	if n != 5 {
		t.Fatalf("n should be 5, is %v", n)
	}
	if string(stuff[0:5]) != "hello" {
		t.Fatalf("invalid contents")
	}
}

func TestClientReadSequential(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	d, err := os.MkdirTemp("", "sftptest-readsequential")
	require.NoError(t, err)

	defer os.RemoveAll(d)

	f, err := os.CreateTemp(d, "read-sequential-test")
	require.NoError(t, err)
	fname := f.Name()
	content := []byte("hello world")
	f.Write(content)
	f.Close()

	for _, maxPktSize := range []int{1, 2, 3, 4} {
		sftp.maxPacket = maxPktSize

		sftpFile, err := sftp.OpenRead(fname)
		require.NoError(t, err)

		stuff := make([]byte, 32)
		n, err := sftpFile.Read(stuff)
		require.ErrorIs(t, err, io.EOF)
		require.Equal(t, len(content), n)
		require.Equal(t, content, stuff[0:len(content)])

		err = sftpFile.Close()
		require.NoError(t, err)

		sftpFile, err = sftp.OpenRead(fname)
		require.NoError(t, err)

		stuff = make([]byte, 5)
		n, err = sftpFile.Read(stuff)
		require.NoError(t, err)
		require.Equal(t, len(stuff), n)
		require.Equal(t, content[:len(stuff)], stuff)

		err = sftpFile.Close()
		require.NoError(t, err)

		// now read from a offset
		off := int64(3)
		sftpFile, err = sftp.OpenRead(fname)
		require.NoError(t, err)

		stuff = make([]byte, 5)
		n, err = sftpFile.ReadAt(stuff, off)
		require.NoError(t, err)
		require.Equal(t, len(stuff), n)
		require.Equal(t, content[off:off+int64(len(stuff))], stuff)

		err = sftpFile.Close()
		require.NoError(t, err)
	}
}

/*
// this writer requires maxPacket = 3 and always returns an error for the second write call
type lastChunkErrSequentialWriter struct {
	counter int
}

func (w *lastChunkErrSequentialWriter) Write(b []byte) (int, error) {
	w.counter++
	if w.counter == 1 {
		if len(b) != 3 {
			return 0, errors.New("this writer requires maxPacket = 3, please set MaxPacketChecked(3)")
		}
		return len(b), nil
	}
	return 1, errors.New("this writer fails after the first write")
}

func TestClientWriteSequentialWriterErr(t *testing.T) {
	client, cmd := testClient(t, readOnly_, nodelay_, MaxPacketChecked(3))
	defer cmd.Wait()
	defer client.Close()

	d, err := os.MkdirTemp("", "sftptest-writesequential-writeerr")
	require.NoError(t, err)

	defer os.RemoveAll(d)

	f, err := os.CreateTemp(d, "write-sequential-writeerr-test")
	require.NoError(t, err)
	fname := f.Name()
	_, err = f.Write([]byte("12345"))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	sftpFile, err := client.Open(fname)
	require.NoError(t, err)
	defer sftpFile.Close()

	w := &lastChunkErrSequentialWriter{}
	written, err := sftpFile.writeToSequential(w)
	assert.Error(t, err)
	expected := int64(4)
	if written != expected {
		t.Errorf("sftpFile.Write() = %d, but expected %d", written, expected)
	}
	assert.Equal(t, 2, w.counter)
}
*/

func TestClientReadDir(t *testing.T) {
	sftp1, cmd1 := testClient(t, readOnly_, nodelay_)
	sftp2, cmd2 := testClientGoSvr(t, readOnly_, nodelay_)
	defer cmd1.Wait()
	defer cmd2.Wait()
	defer sftp1.Close()
	defer sftp2.Close()

	dir := os.TempDir()

	d, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	osfiles, err := d.Readdir(4096)
	if err != nil {
		t.Fatal(err)
	}

	sftp1Files, err := sftp1.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	sftp2Files, err := sftp2.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	osFilesByName := map[string]os.FileInfo{}
	for _, f := range osfiles {
		osFilesByName[f.Name()] = f
	}
	sftp1FilesByName := map[string]*File{}
	for _, f := range sftp1Files {
		sftp1FilesByName[f.BaseName()] = f
	}
	sftp2FilesByName := map[string]*File{}
	for _, f := range sftp2Files {
		sftp2FilesByName[f.BaseName()] = f
	}

	if len(osFilesByName) != len(sftp1FilesByName) || len(sftp1FilesByName) != len(sftp2FilesByName) {
		t.Fatalf("os gives %v, sftp1 gives %v, sftp2 gives %v", len(osFilesByName), len(sftp1FilesByName), len(sftp2FilesByName))
	}

	for name, osF := range osFilesByName {
		sftp1F, ok := sftp1FilesByName[name]
		if !ok {
			t.Fatalf("%v present in os but not sftp1", name)
		}
		sftp2F, ok := sftp2FilesByName[name]
		if !ok {
			t.Fatalf("%v present in os but not sftp2", name)
		}

		//t.Logf("%v: %v %v %v", name, osF, sftp1F, sftp2F)
		if osF.Size() != int64(sftp1F.Size()) || sftp1F.Size() != sftp2F.Size() {
			t.Fatalf("size %d %d %d", osF.Size(), sftp1F.Size(), sftp2F.Size())
		}
		if osF.IsDir() != sftp1F.IsDir() || sftp1F.IsDir() != sftp2F.IsDir() {
			t.Fatalf("isdir %v %v %v", osF.IsDir(), sftp1F.IsDir(), sftp2F.IsDir())
		}
		if osF.ModTime().Sub(sftp1F.ModTime()) > time.Second || sftp1F.ModTime() != sftp2F.ModTime() {
			t.Fatalf("modtime %v %v %v", osF.ModTime(), sftp1F.ModTime(), sftp2F.ModTime())
		}
		if osF.Mode() != sftp1F.OsFileMode() || sftp1F.Mode() != sftp2F.Mode() {
			t.Fatalf("mode %x %x %x", osF.Mode(), sftp1F.OsFileMode(), sftp2F.OsFileMode())
		}
	}
}

var clientReadTests = []struct {
	n int64
}{
	{0},
	{1},
	{1000},
	{1024},
	{1025},
	{2048},
	{4096},
	{1 << 12},
	{1 << 13},
	{1 << 14},
	{1 << 15},
	{1 << 16},
	{1 << 17},
	{1 << 18},
	{1 << 19},
	{1 << 20},
}

func TestClientReadHash(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	d, err := os.MkdirTemp("", "sftptest-read")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)

	for _, tt := range clientReadTests {
		f, err := os.CreateTemp(d, "read-test")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		hash := writeN(t, f, tt.n)
		f2, err := sftp.OpenRead(f.Name())
		if err != nil {
			t.Fatal(err)
		}
		defer f2.Close()
		hash2, n := readHash(t, f2)
		if hash != hash2 || tt.n != n {
			t.Errorf("Read: hash: want: %q, got %q, read: want: %v, got %v", hash, hash2, tt.n, n)
		}
	}
}

// readHash reads r until EOF returning the number of bytes read
// and the hash of the contents.
func readHash(t *testing.T, r io.Reader) (string, int64) {
	h := sha1.New()
	read, err := io.Copy(h, r)
	if err != nil {
		t.Fatal(err, string(debug.Stack()))
	}
	return string(h.Sum(nil)), read
}

// writeN writes n bytes of random data to w and returns the
// hash of that data.
func writeN(t *testing.T, w io.Writer, n int64) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	h := sha1.New()

	mw := io.MultiWriter(w, h)

	written, err := io.CopyN(mw, r, n)
	if err != nil {
		t.Fatal(err)
	}
	if written != n {
		t.Fatalf("CopyN(%v): wrote: %v", n, written)
	}
	return string(h.Sum(nil))
}

var clientWriteTests = []struct {
	n     int
	total int64 // cumulative file size
}{
	{0, 0},
	{1, 1},
	{0, 1},
	{999, 1000},
	{24, 1024},
	{1023, 2047},
	{2048, 4095},
	{1 << 12, 8191},
	{1 << 13, 16383},
	{1 << 14, 32767},
	{1 << 15, 65535},
	{1 << 16, 131071},
	{1 << 17, 262143},
	{1 << 18, 524287},
	{1 << 19, 1048575},
	{1 << 20, 2097151},
	{1 << 21, 4194303},
}

func TestClientWrite(t *testing.T) {
	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	d, err := os.MkdirTemp("", "sftptest-write")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)

	f := path.Join(d, "writeTest")
	w, err := sftp.Create(f)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	for _, tt := range clientWriteTests {
		got, err := w.Write(make([]byte, tt.n))
		if err != nil {
			t.Fatal(err)
		}
		if got != tt.n {
			t.Errorf("Write(%v): wrote: want: %v, got %v", tt.n, tt.n, got)
		}
		fi, err := os.Stat(f)
		if err != nil {
			t.Fatal(err)
		}
		if total := fi.Size(); total != tt.total {
			t.Errorf("Write(%v): size: want: %v, got %v", tt.n, tt.total, total)
		}
	}
}

// ReadFrom is basically Write with io.Reader as the arg
func TestClientReadFrom(t *testing.T) {
	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	d, err := os.MkdirTemp("", "sftptest-readfrom")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)

	f := path.Join(d, "writeTest")
	w, err := sftp.Create(f)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	for _, tt := range clientWriteTests {

		got, err := w.ReadFrom(bytes.NewReader(make([]byte, tt.n)))
		if err != nil {
			t.Fatal(err)
		}
		if got != int64(tt.n) {
			t.Fatalf("ReadFrom(len=%d): transferred: %d", tt.n, got)
		}
		stat, err := w.Stat()
		if err != nil {
			t.Fatalf("sftp stat: %s", err)
		}
		fi, err := os.Stat(f)
		if int64(stat.Size) != tt.total {
			t.Fatalf("ReadFrom(%d) expected sftp stat size %d, but is %d, diff=%d",
				tt.n, tt.total, stat.Size, tt.total-int64(stat.Size))
		}
		if err != nil {
			t.Fatalf("stat: %s", err)
		}
		if total := fi.Size(); total != tt.total {
			t.Fatalf("ReadFrom(%d) expected total=%d, but is %d", tt.n, tt.total, total)
		}
	}
}

/*
// A sizedReader is a Reader with a completely arbitrary Size.
type sizedReader struct {
	io.Reader
	size int
}

func (r *sizedReader) Size() int { return r.size }

// Test File.ReadFrom's handling of a Reader's Size:
// it should be used as a heuristic for determining concurrency only.
func TestClientReadFromSizeMismatch(t *testing.T) {
	const (
		packetSize = 1024
		filesize   = 4 * packetSize
	)

	sftp, cmd := testClient(t, readWrite_, nodelay_, MaxPacketChecked(packetSize), UseConcurrentWrites(true))
	defer cmd.Wait()
	defer sftp.Close()

	d, err := os.MkdirTemp("", "sftptest-readfrom-size-mismatch")
	if err != nil {
		t.Fatal("cannot create temp dir:", err)
	}
	defer os.RemoveAll(d)

	buf := make([]byte, filesize)

	for i, reportedSize := range []int{
		-1, filesize - 100, filesize, filesize + 100,
	} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			r := &sizedReader{Reader: bytes.NewReader(buf), size: reportedSize}

			f := path.Join(d, fmt.Sprint(i))
			w, err := sftp.Create(f)
			if err != nil {
				t.Fatal("unexpected error:", err)
			}
			defer w.Close()

			n, err := w.ReadFrom(r)
			assert.EqualValues(t, filesize, n)

			fi, err := os.Stat(f)
			if err != nil {
				t.Fatal("unexpected error:", err)
			}
			assert.EqualValues(t, filesize, fi.Size())
		})
	}
}
*/

/*
// Issue #145 in github
// Deadlock in ReadFrom when network drops after 1 good packet.
// Deadlock would occur anytime desiredInFlight-inFlight==2 and 2 errors
// occurred in a row. The channel to report the errors only had a buffer
// of 1 and 2 would be sent.
var errFakeNet = errors.New("Fake network issue")

func TestClientReadFromDeadlock(t *testing.T) {
	for i := 0; i < 5; i++ {
		clientWriteDeadlock(t, i, func(f *File) {
			b := make([]byte, 32768*4)
			content := bytes.NewReader(b)
			_, err := f.ReadFrom(content)
			if !errors.Is(err, errFakeNet) {
				t.Fatal("Didn't receive correct error:", err)
			}
		})
	}
}

// Write has exact same problem
func TestClientWriteDeadlock(t *testing.T) {
	for i := 0; i < 5; i++ {
		clientWriteDeadlock(t, i, func(f *File) {
			b := make([]byte, 32768*4)

			_, err := f.Write(b)
			if !errors.Is(err, errFakeNet) {
				t.Fatal("Didn't receive correct error:", err)
			}
		})
	}
}

type timeBombWriter struct {
	count int
	w     io.WriteCloser
}

func (w *timeBombWriter) Write(b []byte) (int, error) {
	if w.count < 1 {
		return 0, errFakeNet
	}

	w.count--
	return w.w.Write(b)
}

func (w *timeBombWriter) Close() error {
	return w.w.Close()
}

// shared body for both previous tests
func clientWriteDeadlock(t *testing.T, N int, badfunc func(*File)) {
	if !testGoServer_ {
		t.Skipf("skipping without -testserver")
	}

	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	d, err := os.MkdirTemp("", "sftptest-writedeadlock")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)

	f := path.Join(d, "writeTest")
	w, err := sftp.Create(f)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// Override the clienConn Writer with a failing version
	// Replicates network error/drop part way through (after N good writes)
	wrap := sftp.conn.w
	sftp.conn.w = &timeBombWriter{
		count: N,
		w:     wrap,
	}

	// this locked (before the fix)
	badfunc(w)
}

// Read/WriteTo has this issue as well
func TestClientReadDeadlock(t *testing.T) {
	for i := 0; i < 3; i++ {
		clientReadDeadlock(t, i, func(f *File) {
			b := make([]byte, 32768*4)

			_, err := f.Read(b)
			if !errors.Is(err, errFakeNet) {
				t.Fatal("Didn't receive correct error:", err)
			}
		})
	}
}

func TestClientWriteToDeadlock(t *testing.T) {
	for i := 0; i < 3; i++ {
		clientReadDeadlock(t, i, func(f *File) {
			b := make([]byte, 32768*4)

			buf := bytes.NewBuffer(b)

			_, err := f.WriteTo(buf)
			if !errors.Is(err, errFakeNet) {
				t.Fatal("Didn't receive correct error:", err)
			}
		})
	}
}

func clientReadDeadlock(t *testing.T, N int, badfunc func(*File)) {
	if !testGoServer_ {
		t.Skipf("skipping without -testserver")
	}
	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	d, err := os.MkdirTemp("", "sftptest-readdeadlock")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)

	f := path.Join(d, "writeTest")

	w, err := sftp.Create(f)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// write the data for the read tests
	b := make([]byte, 32768*4)
	w.Write(b)

	// open new copy of file for read tests
	r, err := sftp.OpenRead(f)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	// Override the clienConn Writer with a failing version
	// Replicates network error/drop part way through (after N good writes)
	wrap := sftp.conn.w
	sftp.conn.w = &timeBombWriter{
		count: N,
		w:     wrap,
	}

	// this locked (before the fix)
	badfunc(r)
}
*/

func TestClientSyncGo(t *testing.T) {
	if !testGoServer_ {
		t.Skipf("skipping without -testserver")
	}
	err := testClientSync(t)

	// Since Server does not support the fsync extension, we can only
	// check that we get the right error.
	require.Error(t, err)

	switch err := err.(type) {
	case *StatusError:
		assert.Equal(t, ErrSSHFxOpUnsupported, err.FxCode())
	default:
		t.Error(err)
	}
}

func TestClientSyncSFTP(t *testing.T) {
	if testGoServer_ {
		t.Skipf("skipping (using Go server)")
	}
	err := testClientSync(t)
	assert.NoError(t, err)
}

func testClientSync(t *testing.T) error {
	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	d, err := os.MkdirTemp("", "sftptest.sync")
	require.NoError(t, err)
	defer os.RemoveAll(d)

	f := path.Join(d, "syncTest")
	w, err := sftp.Create(f)
	require.NoError(t, err)
	defer w.Close()

	return w.Sync()
}

/* TODO: convert this to use client.WalkDir and standard go fs

// taken from github.com/kr/fs/walk_test.go

type Node struct {
	name    string
	entries []*Node // nil if the entry is a file
	mark    int
}

var tree = &Node{
	"testdata",
	[]*Node{
		{"a", nil, 0},
		{"b", []*Node{}, 0},
		{"c", nil, 0},
		{
			"d",
			[]*Node{
				{"x", nil, 0},
				{"y", []*Node{}, 0},
				{
					"z",
					[]*Node{
						{"u", nil, 0},
						{"v", nil, 0},
					},
					0,
				},
			},
			0,
		},
	},
	0,
}

func walkTree(n *Node, path string, f func(path string, n *Node)) {
	f(path, n)
	for _, e := range n.entries {
		walkTree(e, filepath.Join(path, e.name), f)
	}
}

func makeTree(t *testing.T) {
	walkTree(tree, tree.name, func(path string, n *Node) {
		if n.entries == nil {
			fd, err := os.Create(path)
			if err != nil {
				t.Errorf("makeTree: %v", err)
				return
			}
			fd.Close()
		} else {
			os.Mkdir(path, 0770)
		}
	})
}

func markTree(n *Node) { walkTree(n, "", func(path string, n *Node) { n.mark++ }) }

func checkMarks(t *testing.T, report bool) {
	walkTree(tree, tree.name, func(path string, n *Node) {
		if n.mark != 1 && report {
			t.Errorf("node %s mark = %d; expected 1", path, n.mark)
		}
		n.mark = 0
	})
}

// Assumes that each node name is unique. Good enough for a test.
// If clear is true, any incoming error is cleared before return. The errors
// are always accumulated, though.
func mark(path string, info os.FileInfo, err error, errors *[]error, clear bool) error {
	if err != nil {
		*errors = append(*errors, err)
		if clear {
			return nil
		}
		return err
	}
	name := info.Name()
	walkTree(tree, tree.name, func(path string, n *Node) {
		if n.name == name {
			n.mark++
		}
	})
	return nil
}

func TestClientWalk(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	makeTree(t)
	errors := make([]error, 0, 10)
	clear := true
	markFn := func(walker *fs.Walker) error {
		for walker.Step() {
			err := mark(walker.Path(), walker.Stat(), walker.Err(), &errors, clear)
			if err != nil {
				return err
			}
		}
		return nil
	}
	// Expect no errors.
	err := markFn(sftp.Walk(tree.name))
	if err != nil {
		t.Fatalf("no error expected, found: %s", err)
	}
	if len(errors) != 0 {
		t.Fatalf("unexpected errors: %s", errors)
	}
	checkMarks(t, true)
	errors = errors[0:0]

	// Test permission errors.  Only possible if we're not root
	// and only on some file systems (AFS, FAT).  To avoid errors during
	// all.bash on those file systems, skip during go test -short.
	if os.Getuid() > 0 && !testing.Short() {
		// introduce 2 errors: chmod top-level directories to 0
		os.Chmod(filepath.Join(tree.name, tree.entries[1].name), 0)
		os.Chmod(filepath.Join(tree.name, tree.entries[3].name), 0)

		// 3) capture errors, expect two.
		// mark respective subtrees manually
		markTree(tree.entries[1])
		markTree(tree.entries[3])
		// correct double-marking of directory itself
		tree.entries[1].mark--
		tree.entries[3].mark--
		err := markFn(sftp.Walk(tree.name))
		if err != nil {
			t.Fatalf("expected no error return from Walk, got %s", err)
		}
		if len(errors) != 2 {
			t.Errorf("expected 2 errors, got %d: %s", len(errors), errors)
		}
		// the inaccessible subtrees were marked manually
		checkMarks(t, true)
		errors = errors[0:0]

		// 4) capture errors, stop after first error.
		// mark respective subtrees manually
		markTree(tree.entries[1])
		markTree(tree.entries[3])
		// correct double-marking of directory itself
		tree.entries[1].mark--
		tree.entries[3].mark--
		clear = false // error will stop processing
		err = markFn(sftp.Walk(tree.name))
		if err == nil {
			t.Fatalf("expected error return from Walk")
		}
		if len(errors) != 1 {
			t.Errorf("expected 1 error, got %d: %s", len(errors), errors)
		}
		// the inaccessible subtrees were marked manually
		checkMarks(t, false)
		errors = errors[0:0]

		// restore permissions
		os.Chmod(filepath.Join(tree.name, tree.entries[1].name), 0770)
		os.Chmod(filepath.Join(tree.name, tree.entries[3].name), 0770)
	}

	// cleanup
	if err := os.RemoveAll(tree.name); err != nil {
		t.Errorf("removeTree: %v", err)
	}
}
*/

type MatchTest struct {
	pattern, s string
	match      bool
	err        error
}

var matchTests = []MatchTest{
	{"abc", "abc", true, nil},
	{"*", "abc", true, nil},
	{"*c", "abc", true, nil},
	{"a*", "a", true, nil},
	{"a*", "abc", true, nil},
	{"a*", "ab/c", false, nil},
	{"a*/b", "abc/b", true, nil},
	{"a*/b", "a/c/b", false, nil},
	{"a*b*c*d*e*/f", "axbxcxdxe/f", true, nil},
	{"a*b*c*d*e*/f", "axbxcxdxexxx/f", true, nil},
	{"a*b*c*d*e*/f", "axbxcxdxe/xxx/f", false, nil},
	{"a*b*c*d*e*/f", "axbxcxdxexxx/fff", false, nil},
	{"a*b?c*x", "abxbbxdbxebxczzx", true, nil},
	{"a*b?c*x", "abxbbxdbxebxczzy", false, nil},
	{"ab[c]", "abc", true, nil},
	{"ab[b-d]", "abc", true, nil},
	{"ab[e-g]", "abc", false, nil},
	{"ab[^c]", "abc", false, nil},
	{"ab[^b-d]", "abc", false, nil},
	{"ab[^e-g]", "abc", true, nil},
	{"a\\*b", "a*b", true, nil},
	{"a\\*b", "ab", false, nil},
	{"a?b", "a☺b", true, nil},
	{"a[^a]b", "a☺b", true, nil},
	{"a???b", "a☺b", false, nil},
	{"a[^a][^a][^a]b", "a☺b", false, nil},
	{"[a-ζ]*", "α", true, nil},
	{"*[a-ζ]", "A", false, nil},
	{"a?b", "a/b", false, nil},
	{"a*b", "a/b", false, nil},
	{"[\\]a]", "]", true, nil},
	{"[\\-]", "-", true, nil},
	{"[x\\-]", "x", true, nil},
	{"[x\\-]", "-", true, nil},
	{"[x\\-]", "z", false, nil},
	{"[\\-x]", "x", true, nil},
	{"[\\-x]", "-", true, nil},
	{"[\\-x]", "a", false, nil},
	{"[]a]", "]", false, ErrBadPattern},
	{"[-]", "-", false, ErrBadPattern},
	{"[x-]", "x", false, ErrBadPattern},
	{"[x-]", "-", false, ErrBadPattern},
	{"[x-]", "z", false, ErrBadPattern},
	{"[-x]", "x", false, ErrBadPattern},
	{"[-x]", "-", false, ErrBadPattern},
	{"[-x]", "a", false, ErrBadPattern},
	{"\\", "a", false, ErrBadPattern},
	{"[a-b-c]", "a", false, ErrBadPattern},
	{"[", "a", false, ErrBadPattern},
	{"[^", "a", false, ErrBadPattern},
	{"[^bc", "a", false, ErrBadPattern},
	{"a[", "ab", false, ErrBadPattern},
	{"*x", "xxx", true, nil},

	// The following test behaves differently on Go 1.15.3 and Go tip as
	// https://github.com/golang/go/commit/b5ddc42b465dd5b9532ee336d98343d81a6d35b2
	// (pre-Go 1.16). TODO: reevaluate when Go 1.16 is released.
	//{"a[", "a", false, nil},
}

func errp(e error) string {
	if e == nil {
		return "<nil>"
	}
	return e.Error()
}

// contains returns true if vector contains the string s.
func contains(vector []string, s string) bool {
	for _, elem := range vector {
		if elem == s {
			return true
		}
	}
	return false
}

var globTests = []struct {
	pattern, result string
}{
	{"match.go", "match.go"},
	{"mat?h.go", "match.go"},
	{"ma*ch.go", "match.go"},
	{`\m\a\t\c\h\.\g\o`, "match.go"},
	{"../*/match.go", "../usftp/match.go"},
}

//type globTest struct {
//	pattern string
//	matches []string
//}
//
//func (test *globTest) buildWant(root string) []string {
//	var want []string
//	for _, m := range test.matches {
//		want = append(want, root+filepath.FromSlash(m))
//	}
//	sort.Strings(want)
//	return want
//}

func TestMatch(t *testing.T) {
	for _, tt := range matchTests {
		pattern := tt.pattern
		s := tt.s
		ok, err := path.Match(pattern, s)
		if ok != tt.match || err != tt.err {
			t.Errorf("Match(%#q, %#q) = %v, %q want %v, %q", pattern, s, ok, errp(err), tt.match, errp(tt.err))
		}
	}
}

func TestGlob(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	for _, tt := range globTests {
		pattern := tt.pattern
		result := tt.result
		matches, err := sftp.Glob(pattern)
		if err != nil {
			t.Errorf("Glob error for %q: %s", pattern, err)
			continue
		}
		if !contains(matches, result) {
			t.Errorf("Glob(%#q) = %#v want %v", pattern, matches, result)
		}
	}
	for _, pattern := range []string{"no_match", "../*/no_match"} {
		matches, err := sftp.Glob(pattern)
		if err != nil {
			t.Errorf("Glob error for %q: %s", pattern, err)
			continue
		}
		if len(matches) != 0 {
			t.Errorf("Glob(%#q) = %#v want []", pattern, matches)
		}
	}
}

func TestGlobError(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	_, err := sftp.Glob("[7]")
	if err != nil {
		t.Error("expected error for bad pattern; got none")
	}
}

func TestGlobUNC(t *testing.T) {
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()
	// Just make sure this runs without crashing for now.
	// See issue 15879.
	sftp.Glob(`\\?\C:\*`)
}

// sftp/issue/42, abrupt server hangup would result in client hangs.
func TestServerRoughDisconnect(t *testing.T) {
	skipIfWindows(t)
	/*
		if testGoServer_ {
			t.Skipf("skipping (using Go server)")
		}
	*/
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := sftp.OpenRead("/dev/zero")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	go func() {
		time.Sleep(100 * time.Millisecond)
		cmd.Process.Kill()
	}()

	_, err = io.Copy(io.Discard, f)
	assert.Error(t, err)
}

// sftp/issue/181, abrupt server hangup would result in client hangs.
// due to broadcastErr filling up the request channel
// this reproduces it about 50% of the time
func TestServerRoughDisconnect2(t *testing.T) {
	skipIfWindows(t)
	if testGoServer_ {
		t.Skipf("skipping (using Go server)")
	}
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := sftp.OpenRead("/dev/zero")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b := make([]byte, 32768*100)
	go func() {
		time.Sleep(1 * time.Millisecond)
		cmd.Process.Kill()
	}()
	for {
		_, err = f.Read(b)
		if err != nil {
			break
		}
	}
}

// sftp/issue/234 - abrupt shutdown during ReadFrom hangs client
func TestServerRoughDisconnect3(t *testing.T) {
	skipIfWindows(t)
	if testGoServer_ {
		t.Skipf("skipping (using Go server)")
	}

	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	dest, err := sftp.Open("/dev/null", os.O_RDWR)
	if err != nil {
		t.Fatal(err)
	}
	defer dest.Close()

	src, err := os.Open("/dev/zero")
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	go func() {
		time.Sleep(10 * time.Millisecond)
		cmd.Process.Kill()
	}()

	_, err = io.Copy(dest, src)
	assert.Error(t, err)
}

// sftp/issue/234 - also affected Write
func TestServerRoughDisconnect4(t *testing.T) {
	skipIfWindows(t)
	if testGoServer_ {
		t.Skipf("skipping (using Go server)")
	}
	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	dest, err := sftp.Open("/dev/null", os.O_RDWR)
	if err != nil {
		t.Fatal(err)
	}
	defer dest.Close()

	src, err := os.Open("/dev/zero")
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	go func() {
		time.Sleep(10 * time.Millisecond)
		cmd.Process.Kill()
	}()

	b := make([]byte, 32768*200)
	src.Read(b)
	for {
		_, err = dest.Write(b)
		if err != nil {
			assert.NotEqual(t, io.EOF, err)
			break
		}
	}

	_, err = io.Copy(dest, src)
	assert.Error(t, err)
}

// sftp/issue/390 - server disconnect should not cause io.EOF or
// io.ErrUnexpectedEOF in sftp.File.Read, because those confuse io.ReadFull.
func TestServerRoughDisconnectEOF(t *testing.T) {
	skipIfWindows(t)
	/*
		if testGoServer_ {
			t.Skipf("skipping (using Go server)")
		}
	*/
	sftp, cmd := testClient(t, readOnly_, nodelay_)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := sftp.OpenRead("/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	go func() {
		time.Sleep(100 * time.Millisecond)
		cmd.Process.Kill()
	}()

	_, err = io.ReadFull(f, make([]byte, 10))
	assert.Error(t, err)
	assert.NotEqual(t, io.ErrUnexpectedEOF, err)
}

// sftp/issue/26 writing to a read only file caused client to loop.
func TestClientWriteToROFile(t *testing.T) {
	skipIfWindows(t)

	sftp, cmd := testClient(t, readWrite_, nodelay_)
	defer cmd.Wait()

	defer func() {
		err := sftp.Close()
		assert.NoError(t, err)
	}()

	// TODO (puellanivis): /dev/zero is not actually a read-only file.
	// So, this test works purely by accident.
	f, err := sftp.OpenRead("/dev/zero")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	_, err = f.Write([]byte("hello"))
	if err == nil {
		t.Fatal("expected error, got", err)
	}
}

type sink struct{}

func (*sink) Close() error                { return nil }
func (*sink) Write(p []byte) (int, error) { return len(p), nil }

func TestClientZeroLengthPacket(t *testing.T) {
	// Packet length zero (never valid). This used to crash the client.
	packet := []byte{0, 0, 0, 0}

	r := bytes.NewReader(packet)
	c, err := NewClientPipe(r, &sink{})
	if err == nil {
		t.Error("expected an error, got nil")
	}
	if c != nil {
		c.Close()
	}
}

func TestClientShortPacket(t *testing.T) {
	// init packet too short.
	packet := []byte{0, 0, 0, 1, 2}

	r := bytes.NewReader(packet)
	_, err := NewClientPipe(r, &sink{})
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected error: %v, got: %v", io.ErrUnexpectedEOF, err)
	}
}

// Issue #418: panic in clientConn.recv when the sid is incomplete.
func TestClientNoSid(t *testing.T) {
	buff := make([]byte, 4096)
	stream := new(bytes.Buffer)
	sendPacket(stream, buff, &sshFxVersionPacket{Version: sftpProtocolVersion})

	c, err := NewClientPipe(stream, &sink{})
	if err != nil {
		t.Fatal(err)
	}

	// Next packet has the sid cut short after two bytes.
	stream.Write([]byte{0, 0, 0, 10, 0, 0})

	_, err = c.Stat("anything")
	if nil == err {
		t.Fatal("expected error, got", err)
	}
}
