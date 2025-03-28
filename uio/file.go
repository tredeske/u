package uio

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tredeske/u/uerr"
)

// get the absolute/canonical path, or panic
func MustAbsPath(f string) string {
	rv, err := filepath.Abs(f)
	if err != nil {
		panic(err)
	}
	return rv
}

// Remove all files starting at rmF.  If rmF is a dir, then recursively
// remove all files.
//
// attempt to prevent rm of "/" or any path such as /usr, /opt, ...
func FileRemoveAll(rmF string) (err error) {

	if 0 == len(rmF) {
		return
	}
	var abs string
	abs, err = filepath.Abs(rmF)
	if err != nil {
		return
	}
	abs = filepath.Clean(abs)
	if 2 > strings.Count(abs, "/") {
		err = fmt.Errorf("Invalid path to remove: '%s'", abs)
		return
	}

	err = os.RemoveAll(abs)
	if nil == err {
		return //////////////////// success
	}

	//
	// it may be that there is a dir that is not rwx for us, so walk the
	// tree and repair perms, then retry
	//
	// we only attempt to fix the owner rwx because if we are not the owner
	// of the file, we can't change the perms anyway
	//
	// we cannot use filepath.Walk as it bails out early if perms not correct
	//

	walkErr := Walk(abs,
		func(path string, fi os.FileInfo, walkErr error) (err error) {
			if nil != fi && fi.IsDir() {
				perms := fi.Mode().Perm()
				if 0700 != (perms & 0700) {
					err = os.Chmod(path, perms|0700)
				}
			}
			return
		})
	if walkErr != nil {
		err = uerr.Chainf(err, "Unable to fix perms: %s", walkErr)
		return
	}

	err = os.RemoveAll(abs)
	return
}

// split the dirname, filename base, and filename extension parts
// /path/to/foobar.tar.gz results in "/path/to/", "foobar", ".tar.gz"
func FilenameParts(filename string) (dir, base, ext string) {
	dir, base = filepath.Split(filename)
	ext = filepath.Ext(base)
	if 0 != len(ext) && len(ext) < 6 {
		base = base[:len(base)-len(ext)]
		ext2 := filepath.Ext(base)
		if ext2 == ".tar" {
			ext = ext2 + ext
			base = base[:len(base)-len(ext2)]
		}
	}
	return
}

// attempt to create a hard link to 'copy' the file.  if that fails, then
// perform an actual copy
func FileLinkOrCopy(src, dst string) (err error) {
	err = os.Link(src, dst)
	if err != nil { // on failure, try fileCopy instead
		err = FileCopy(src, dst)
	}
	return err
}

// copy src file to dst file
func FileCopy(src, dst string) (err error) {

	//
	// 1st try to copy using mmap (much more efficient)
	//
	srcM, srcInfo, err := MapFile(src)
	if nil == err {
		err = os.WriteFile(dst, srcM, srcInfo.Mode())
		srcM.Close()
		return
	}

	//
	// fallback to standard
	//
	srcF, err := os.Open(src)
	if err != nil {
		return uerr.Chainf(err, "Opening %s for copying to %s", src, dst)
	}
	defer srcF.Close()

	stat, err := srcF.Stat()
	if err != nil {
		return
	}

	_, err = CopyBufferToFile(srcF, src, dst, stat.Size(), nil)
	return
}

// copy bytes from src io.Reader to dst file.
// if srcSz is a positive number, ensure srcSz bytes were copied
func CopyToFile(src io.Reader, srcName, dst string, srcSz int64,
) (amount int64, err error) {

	return CopyBufferToFile(src, srcName, dst, srcSz, nil)
}

// copy bytes from src io.Reader to dst file using provided buffer.
// if srcSz is a positive number, ensure srcSz bytes were copied
// if no provided buffer, then use a default buffer
func CopyBufferToFile(src io.Reader, srcName, dst string, srcSz int64, buf []byte,
) (amount int64, err error) {

	dstF, err := os.Create(dst)
	if err != nil {
		err = uerr.Chainf(err, "Creating %s to copy %s", dst, srcName)
		return
	}
	defer func() {
		cerr := dstF.Close()
		if nil != cerr && err == nil {
			err = uerr.Chainf(cerr, "Closing %s for copying from %s", dst, srcName)
		}
	}()

	amount, err = CopyBufferTo(dstF, src, srcSz, buf)
	if err != nil {
		err = uerr.Chainf(err, "Copying %s to %s", srcName, dst)
	}
	return
}

// hard link or copy file to specified directory, returning new name
func FileLinkOrCopyTo(file, dir string) (dst string, err error) {
	dst = path.Join(dir, path.Base(file))
	err = FileLinkOrCopy(file, dst)
	return
}

// move file to specified directory, returning new name
//
// if dir is on different disk, then a copy is performed, and the original
// file is removed upon success.  in the case of copy, the new file is created
// with a hidden filename, then renamed.
func FileMoveOrCopyTo(file, dstDir string) (dst string, err error) {
	dst, err = FileMoveTo(file, dstDir)
	if err != nil && 0 != len(dst) { // didn't work - try copy
		base := path.Base(file)
		dstHidden := path.Join(dstDir, "."+base)
		err = FileCopy(file, dstHidden)
		if nil == err {
			err = os.Rename(dstHidden, dst)
			FileRemoveAll(file)
		}
	}
	return
}

// move file to specified directory, returning new name
//
// file and directory must be on same disk
func FileMoveTo(file, dstDir string) (dst string, err error) {
	if 0 == len(dstDir) {
		err = errors.New("dstDir not provided")
		return
	}
	srcDir, srcF := path.Split(file) // contains trailing slash
	dst = path.Join(dstDir, srcF)
	if srcDir != dstDir && dstDir != srcDir[:len(srcDir)-1] {
		err = os.Rename(file, dst)
	}
	return
}

// symlink file to specified directory, returning new name
func FileSymlinkTo(file, dir string) (dst string, err error) {
	dst = path.Join(dir, path.Base(file))
	err = os.Symlink(file, dst)
	return
}

// can we stat the file?
func FileExists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}

// get the size of the file, if it is a regular file
func FileSize(file string) (int64, error) {
	fi, err := os.Stat(file)
	if err != nil {
		return 0, err
		//} else if fi.IsDir() {
		//	return DiskUsage(file)
	} else {
		return fi.Size(), nil
	}
}

// get file mod time
func FileMTime(file string) (time.Time, error) {
	fi, err := os.Stat(file)
	if err != nil {
		return time.Unix(0, 0), err
	} else {
		return fi.ModTime(), nil
	}
}

// get file owner
func FileUid(file string) (int, error) {
	fi, err := os.Stat(file)
	if err != nil {
		return 0, err
	}
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errors.New("was not a stat struct")
	}
	return int(stat.Uid), nil
}

// get file owner username
func FileUser(file string) (string, error) {
	uid, err := FileUid(file)
	if err != nil {
		return "", err
	}
	user, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return "", err
	}
	return user.Username, nil
}

// update mod time of file, or create it
func FileTouch(file string, t time.Time) error {
	return os.Chtimes(file, t, t)
}

// compute the md5 of the file
func FileMd5(file string) (sum []byte, sz int, err error) {
	content, _, err := MapFile(file)
	if err != nil {
		return
	}
	sz = len(content)
	md5sum := md5.Sum(content)
	sum = md5sum[:]
	//sum = md5.Sum(content)[:]
	err = content.Close()
	//return md5sum[:], sz, err
	//return md5sum, sz, err
	return
}

// watch a file.  if there is a change, then call onChange with fi set.
// if there is an error watching the file, call onChange with err set
func FileWatch(
	file string,
	period time.Duration,
	onChange func(changedFile string, changedInfo os.FileInfo, err error),
) {

	updated := time.Now()
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	for {
		<-ticker.C // wait til time mark
		stat, err := os.Stat(file)
		if err != nil {
			onChange(file, stat, err)
		} else if stat.ModTime().After(updated) {
			updated = stat.ModTime()
			onChange(file, stat, err)
		}
	}
}

// Open file for create, run the filler, then close the file
// Ensures file closed properly
func FileCreate(name string, filler func(*os.File) error) (err error) {
	f, err := os.Create(name)
	if err != nil {
		return
	}
	defer func() {
		if nil != f {
			f.Close()
		}
	}()

	if nil != filler {
		err = filler(f)
		if err != nil {
			return
		}
	}

	err = f.Close()
	f = nil
	return
}
