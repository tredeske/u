package usftp

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"
)

func benchmarkRead(b *testing.B, bufsize int, delay time.Duration) {
	skipIfWindows(b)
	size := 10*1024*1024 + 123 // ~10MiB

	// open sftp client
	sftp, cmd := testClient(b, readOnly_, delay)
	defer cmd.Wait()
	defer sftp.Close()

	buf := make([]byte, bufsize)

	b.ResetTimer()
	b.SetBytes(int64(size))

	for i := 0; i < b.N; i++ {
		offset := 0

		f2, err := sftp.OpenRead("/dev/zero")
		if err != nil {
			b.Fatal(err)
		}

		for offset < size {
			n, err := io.ReadFull(f2, buf)
			offset += n
			if err == io.ErrUnexpectedEOF && offset != size {
				b.Fatalf("read too few bytes! want: %d, got: %d", size, n)
			}

			if err != nil {
				b.Fatal(err)
			}

			offset += n
		}

		f2.Close()
	}
}

func BenchmarkRead1k(b *testing.B) {
	benchmarkRead(b, 1*1024, nodelay_)
}

func BenchmarkRead16k(b *testing.B) {
	benchmarkRead(b, 16*1024, nodelay_)
}

func BenchmarkRead32k(b *testing.B) {
	benchmarkRead(b, 32*1024, nodelay_)
}

func BenchmarkRead128k(b *testing.B) {
	benchmarkRead(b, 128*1024, nodelay_)
}

func BenchmarkRead512k(b *testing.B) {
	benchmarkRead(b, 512*1024, nodelay_)
}

func BenchmarkRead1MiB(b *testing.B) {
	benchmarkRead(b, 1024*1024, nodelay_)
}

func BenchmarkRead4MiB(b *testing.B) {
	benchmarkRead(b, 4*1024*1024, nodelay_)
}

func BenchmarkRead4MiBDelay10Msec(b *testing.B) {
	benchmarkRead(b, 4*1024*1024, 10*time.Millisecond)
}

func BenchmarkRead4MiBDelay50Msec(b *testing.B) {
	benchmarkRead(b, 4*1024*1024, 50*time.Millisecond)
}

func BenchmarkRead4MiBDelay150Msec(b *testing.B) {
	benchmarkRead(b, 4*1024*1024, 150*time.Millisecond)
}

func benchmarkWrite(b *testing.B, bufsize int, delay time.Duration) {
	size := 10*1024*1024 + 123 // ~10MiB

	// open sftp client
	sftp, cmd := testClient(b, false, delay)
	defer cmd.Wait()
	defer sftp.Close()

	data := make([]byte, size)

	b.ResetTimer()
	b.SetBytes(int64(size))

	for i := 0; i < b.N; i++ {
		offset := 0

		f, err := os.CreateTemp("", "sftptest-benchwrite")
		if err != nil {
			b.Fatal(err)
		}
		defer os.Remove(f.Name()) // actually queue up a series of removes for these files

		f2, err := sftp.Create(f.Name())
		if err != nil {
			b.Fatal(err)
		}

		for offset < size {
			buf := data[offset:]
			if len(buf) > bufsize {
				buf = buf[:bufsize]
			}

			n, err := f2.Write(buf)
			if err != nil {
				b.Fatal(err)
			}

			if offset+n < size && n != bufsize {
				b.Fatalf("wrote too few bytes! want: %d, got: %d", size, n)
			}

			offset += n
		}

		f2.Close()

		fi, err := os.Stat(f.Name())
		if err != nil {
			b.Fatal(err)
		}

		if fi.Size() != int64(size) {
			b.Fatalf("wrong file size: want %d, got %d", size, fi.Size())
		}

		os.Remove(f.Name())
	}
}

func BenchmarkWrite1k(b *testing.B) {
	benchmarkWrite(b, 1*1024, nodelay_)
}

func BenchmarkWrite16k(b *testing.B) {
	benchmarkWrite(b, 16*1024, nodelay_)
}

func BenchmarkWrite32k(b *testing.B) {
	benchmarkWrite(b, 32*1024, nodelay_)
}

func BenchmarkWrite128k(b *testing.B) {
	benchmarkWrite(b, 128*1024, nodelay_)
}

func BenchmarkWrite512k(b *testing.B) {
	benchmarkWrite(b, 512*1024, nodelay_)
}

func BenchmarkWrite1MiB(b *testing.B) {
	benchmarkWrite(b, 1024*1024, nodelay_)
}

func BenchmarkWrite4MiB(b *testing.B) {
	benchmarkWrite(b, 4*1024*1024, nodelay_)
}

func BenchmarkWrite4MiBDelay10Msec(b *testing.B) {
	benchmarkWrite(b, 4*1024*1024, 10*time.Millisecond)
}

func BenchmarkWrite4MiBDelay50Msec(b *testing.B) {
	benchmarkWrite(b, 4*1024*1024, 50*time.Millisecond)
}

func BenchmarkWrite4MiBDelay150Msec(b *testing.B) {
	benchmarkWrite(b, 4*1024*1024, 150*time.Millisecond)
}

func benchmarkReadFrom(b *testing.B, bufsize int, delay time.Duration) {
	size := 10*1024*1024 + 123 // ~10MiB

	// open sftp client
	sftp, cmd := testClient(b, false, delay)
	defer cmd.Wait()
	defer sftp.Close()

	data := make([]byte, size)

	b.ResetTimer()
	b.SetBytes(int64(size))

	for i := 0; i < b.N; i++ {
		f, err := os.CreateTemp("", "sftptest-benchreadfrom")
		if err != nil {
			b.Fatal(err)
		}
		defer os.Remove(f.Name())

		f2, err := sftp.Create(f.Name())
		if err != nil {
			b.Fatal(err)
		}
		defer f2.Close()

		f2.ReadFrom(bytes.NewReader(data))
		f2.Close()

		fi, err := os.Stat(f.Name())
		if err != nil {
			b.Fatal(err)
		}

		if fi.Size() != int64(size) {
			b.Fatalf("wrong file size: want %d, got %d", size, fi.Size())
		}

		os.Remove(f.Name())
	}
}

func BenchmarkReadFrom1k(b *testing.B) {
	benchmarkReadFrom(b, 1*1024, nodelay_)
}

func BenchmarkReadFrom16k(b *testing.B) {
	benchmarkReadFrom(b, 16*1024, nodelay_)
}

func BenchmarkReadFrom32k(b *testing.B) {
	benchmarkReadFrom(b, 32*1024, nodelay_)
}

func BenchmarkReadFrom128k(b *testing.B) {
	benchmarkReadFrom(b, 128*1024, nodelay_)
}

func BenchmarkReadFrom512k(b *testing.B) {
	benchmarkReadFrom(b, 512*1024, nodelay_)
}

func BenchmarkReadFrom1MiB(b *testing.B) {
	benchmarkReadFrom(b, 1024*1024, nodelay_)
}

func BenchmarkReadFrom4MiB(b *testing.B) {
	benchmarkReadFrom(b, 4*1024*1024, nodelay_)
}

func BenchmarkReadFrom4MiBDelay10Msec(b *testing.B) {
	benchmarkReadFrom(b, 4*1024*1024, 10*time.Millisecond)
}

func BenchmarkReadFrom4MiBDelay50Msec(b *testing.B) {
	benchmarkReadFrom(b, 4*1024*1024, 50*time.Millisecond)
}

func BenchmarkReadFrom4MiBDelay150Msec(b *testing.B) {
	benchmarkReadFrom(b, 4*1024*1024, 150*time.Millisecond)
}

func benchmarkWriteTo(b *testing.B, bufsize int, delay time.Duration) {
	size := 10*1024*1024 + 123 // ~10MiB

	// open sftp client
	sftp, cmd := testClient(b, false, delay)
	defer cmd.Wait()
	defer sftp.Close()

	f, err := os.CreateTemp("", "sftptest-benchwriteto")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(f.Name())

	data := make([]byte, size)

	f.Write(data)
	f.Close()

	buf := bytes.NewBuffer(make([]byte, 0, size))

	b.ResetTimer()
	b.SetBytes(int64(size))

	for i := 0; i < b.N; i++ {
		buf.Reset()

		f2, err := sftp.OpenRead(f.Name())
		if err != nil {
			b.Fatal(err)
		}

		f2.WriteTo(buf)
		f2.Close()

		if buf.Len() != size {
			b.Fatalf("wrote buffer size: want %d, got %d", size, buf.Len())
		}
	}
}

func BenchmarkWriteTo1k(b *testing.B) {
	benchmarkWriteTo(b, 1*1024, nodelay_)
}

func BenchmarkWriteTo16k(b *testing.B) {
	benchmarkWriteTo(b, 16*1024, nodelay_)
}

func BenchmarkWriteTo32k(b *testing.B) {
	benchmarkWriteTo(b, 32*1024, nodelay_)
}

func BenchmarkWriteTo128k(b *testing.B) {
	benchmarkWriteTo(b, 128*1024, nodelay_)
}

func BenchmarkWriteTo512k(b *testing.B) {
	benchmarkWriteTo(b, 512*1024, nodelay_)
}

func BenchmarkWriteTo1MiB(b *testing.B) {
	benchmarkWriteTo(b, 1024*1024, nodelay_)
}

func BenchmarkWriteTo4MiB(b *testing.B) {
	benchmarkWriteTo(b, 4*1024*1024, nodelay_)
}

func BenchmarkWriteTo4MiBDelay10Msec(b *testing.B) {
	benchmarkWriteTo(b, 4*1024*1024, 10*time.Millisecond)
}

func BenchmarkWriteTo4MiBDelay50Msec(b *testing.B) {
	benchmarkWriteTo(b, 4*1024*1024, 50*time.Millisecond)
}

func BenchmarkWriteTo4MiBDelay150Msec(b *testing.B) {
	benchmarkWriteTo(b, 4*1024*1024, 150*time.Millisecond)
}

func benchmarkCopyDown(b *testing.B, fileSize int64, delay time.Duration) {
	skipIfWindows(b)
	// Create a temp file and fill it with zero's.
	src, err := os.CreateTemp("", "sftptest-benchcopydown")
	if err != nil {
		b.Fatal(err)
	}
	defer src.Close()
	srcFilename := src.Name()
	defer os.Remove(srcFilename)
	zero, err := os.Open("/dev/zero")
	if err != nil {
		b.Fatal(err)
	}
	n, err := io.Copy(src, io.LimitReader(zero, fileSize))
	if err != nil {
		b.Fatal(err)
	}
	if n < fileSize {
		b.Fatal("short copy")
	}
	zero.Close()
	src.Close()

	sftp, cmd := testClient(b, readOnly_, delay)
	defer cmd.Wait()
	defer sftp.Close()
	b.ResetTimer()
	b.SetBytes(fileSize)

	for i := 0; i < b.N; i++ {
		dst, err := os.CreateTemp("", "sftptest-benchcopydown")
		if err != nil {
			b.Fatal(err)
		}
		defer os.Remove(dst.Name())

		src, err := sftp.OpenRead(srcFilename)
		if err != nil {
			b.Fatal(err)
		}
		defer src.Close()
		n, err := io.Copy(dst, src)
		if err != nil {
			b.Fatalf("copy error: %v", err)
		}
		if n < fileSize {
			b.Fatal("unable to copy all bytes")
		}
		dst.Close()
		fi, err := os.Stat(dst.Name())
		if err != nil {
			b.Fatal(err)
		}

		if fi.Size() != fileSize {
			b.Fatalf("wrong file size: want %d, got %d", fileSize, fi.Size())
		}
		os.Remove(dst.Name())
	}
}

func BenchmarkCopyDown10MiBDelay10Msec(b *testing.B) {
	benchmarkCopyDown(b, 10*1024*1024, 10*time.Millisecond)
}

func BenchmarkCopyDown10MiBDelay50Msec(b *testing.B) {
	benchmarkCopyDown(b, 10*1024*1024, 50*time.Millisecond)
}

func BenchmarkCopyDown10MiBDelay150Msec(b *testing.B) {
	benchmarkCopyDown(b, 10*1024*1024, 150*time.Millisecond)
}

func benchmarkCopyUp(b *testing.B, fileSize int64, delay time.Duration) {
	skipIfWindows(b)
	// Create a temp file and fill it with zero's.
	src, err := os.CreateTemp("", "sftptest-benchcopyup")
	if err != nil {
		b.Fatal(err)
	}
	defer src.Close()
	srcFilename := src.Name()
	defer os.Remove(srcFilename)
	zero, err := os.Open("/dev/zero")
	if err != nil {
		b.Fatal(err)
	}
	n, err := io.Copy(src, io.LimitReader(zero, fileSize))
	if err != nil {
		b.Fatal(err)
	}
	if n < fileSize {
		b.Fatal("short copy")
	}
	zero.Close()
	src.Close()

	sftp, cmd := testClient(b, false, delay)
	defer cmd.Wait()
	defer sftp.Close()

	b.ResetTimer()
	b.SetBytes(fileSize)

	for i := 0; i < b.N; i++ {
		tmp, err := os.CreateTemp("", "sftptest-benchcopyup")
		if err != nil {
			b.Fatal(err)
		}
		tmp.Close()
		defer os.Remove(tmp.Name())

		dst, err := sftp.Create(tmp.Name())
		if err != nil {
			b.Fatal(err)
		}
		defer dst.Close()
		src, err := os.Open(srcFilename)
		if err != nil {
			b.Fatal(err)
		}
		defer src.Close()
		n, err := io.Copy(dst, src)
		if err != nil {
			b.Fatalf("copy error: %v", err)
		}
		if n < fileSize {
			b.Fatal("unable to copy all bytes")
		}

		fi, err := os.Stat(tmp.Name())
		if err != nil {
			b.Fatal(err)
		}

		if fi.Size() != fileSize {
			b.Fatalf("wrong file size: want %d, got %d", fileSize, fi.Size())
		}
		os.Remove(tmp.Name())
	}
}

func BenchmarkCopyUp10MiBDelay10Msec(b *testing.B) {
	benchmarkCopyUp(b, 10*1024*1024, 10*time.Millisecond)
}

func BenchmarkCopyUp10MiBDelay50Msec(b *testing.B) {
	benchmarkCopyUp(b, 10*1024*1024, 50*time.Millisecond)
}

func BenchmarkCopyUp10MiBDelay150Msec(b *testing.B) {
	benchmarkCopyUp(b, 10*1024*1024, 150*time.Millisecond)
}
