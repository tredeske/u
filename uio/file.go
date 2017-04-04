package uio

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tredeske/u/uerr"

	"gopkg.in/yaml.v2"
)

//
// load the YAML file into target, which may be a ptr to map or ptr to struct
//
func YamlLoad(file string, target interface{}) error {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(content, target)
}

//
// store contents of source (a map or a struct) into file as YAML
//
func YamlStore(file string, source interface{}) (err error) {
	data, err := yaml.Marshal(source)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, data, 0664)
}

//
// store contents of source (a map or a struct) into file as JSON
//
func JsonStore(file string, it interface{}) (err error) {
	return FileCreate(file,
		func(f *os.File) error {
			return json.NewEncoder(f).Encode(it)
		})
}

//
// load the JSON file into target, which may be a ptr to map or ptr to struct
//
func JsonLoad(file string, target interface{}) (err error) {
	jsonF, err := os.Open(file)
	if nil == err {
		err = json.NewDecoder(jsonF).Decode(target)
		jsonF.Close()
	}
	return
}

//
// load the JSON file into target, which may be a ptr to map or ptr to struct
// if file does not exist, then leave target unchanged and do not error
//
func JsonLoadIfExists(file string, target interface{}) (err error) {
	err = JsonLoad(file, target)
	if err != nil {
		perr, ok := err.(*os.PathError)
		if ok && perr.Op == "open" { // ignore could not open
			err = nil
		}
	}
	return
}

//
// store contents of source (a map or a struct) into string as JSON
//
func JsonString(source interface{}) string {
	//jsonString, err := json.Marshal(source)
	bytes, err := json.MarshalIndent(source, "", " ")
	if err != nil {
		return err.Error()
	}
	return string(bytes)
}

//
// compute the amount of space used by the files rooted at dir
//
func DiskUsage(dir string) (used int64, err error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return used, err
	}
	for _, f := range files {
		if f.IsDir() {
			subUsed, err := DiskUsage(path.Join(dir, f.Name()))
			if err != nil {
				return used, err
			}
			used += subUsed
		} else if f.Mode().IsRegular() {
			used += f.Size()
		}
	}
	return used, nil
}

//
// copy src dir to dst dir
//
func DirCopy(src, dst string) (err error) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return
	} else if !srcInfo.IsDir() {
		err = fmt.Errorf("%s is not a dir", src)
		return
	}

	err = os.MkdirAll(dst, srcInfo.Mode())
	if err != nil {
		return
	}

	files, err := ioutil.ReadDir(src)
	for _, file := range files {

		srcPath := path.Join(src, file.Name())
		dstPath := path.Join(dst, file.Name())

		if file.IsDir() {
			err = DirCopy(srcPath, dstPath)
		} else {
			err = FileCopy(srcPath, dstPath)
		}
		if err != nil {
			break
		}
	}
	return
}

//
// Get the names of files contained by dir.  If max is specified, only
// return up to max names.
//
func DirFilenames(dir string, max ...int) (files []string, err error) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	n := -1
	if 0 != len(max) {
		n = max[0]
	}
	files, err = f.Readdirnames(n)
	return
}

//
// Is the dir empty?
//
func DirEmpty(dir string) bool {
	files, _ := DirFilenames(dir, 1)
	return 1 != len(files)
}

//
// get the absolute/canonical path, or panic
//
func MustAbsPath(f string) string {
	rv, err := filepath.Abs(f)
	if err != nil {
		panic(err)
	}
	return rv
}

//
// attempt to prevent rm of "/" or any path such as /usr, /opt, ...
//
func FileRemoveAll(f string) (err error) {

	if 0 != len(f) {
		var abs string
		abs, err = filepath.Abs(f)
		if err != nil {
			return
		}
		abs = filepath.Clean(abs)
		if 2 > strings.Count(abs, "/") {
			err = fmt.Errorf("Invalid path to remove: '%s'", abs)
			return
		}

		err = os.RemoveAll(abs)
	}
	return
}

//
// split the dirname, filename base, and filename extension parts
// /path/to/foobar.tar.gz results in "/path/to/", "foobar", ".tar.gz"
//
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

//
// attempt to create a hard link to 'copy' the file.  if that fails, then
// perform an actual copy
//
func FileLinkOrCopy(src, dst string) (err error) {
	err = os.Link(src, dst)
	if err != nil { // on failure, try fileCopy instead
		err = FileCopy(src, dst)
	}
	return err
}

//
// copy src file to dst file
//
func FileCopy(src, dst string) (err error) {

	//
	// 1st try to copy using mmap (much more efficient)
	//
	srcM, srcInfo, err := MapFile(src)
	if nil == err {
		err = ioutil.WriteFile(dst, srcM, srcInfo.Mode())
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

//
// copy bytes from src io.Reader to dst file.
// if srcSz is a positive number, ensure srcSz bytes were copied
//
func CopyToFile(src io.Reader, srcName, dst string, srcSz int64,
) (amount int64, err error) {

	return CopyBufferToFile(src, srcName, dst, srcSz, nil)
}

//
// copy bytes from src io.Reader to dst file using provided buffer.
// if srcSz is a positive number, ensure srcSz bytes were copied
//
func CopyBufferToFile(src io.Reader, srcName, dst string, srcSz int64, buf []byte,
) (amount int64, err error) {

	dstF, err := os.Create(dst)
	if err != nil {
		err = uerr.Chainf(err, "Creating %s to copy %s", dst, srcName)
		return
	}
	var b *Buffer
	if 0 == len(buf) {
		b = DefaultPool.Get()
		buf = b.B()
	}
	defer func() {
		if nil != b {
			b.Return()
		}
		cerr := dstF.Close()
		if nil != cerr && err == nil {
			err = uerr.Chainf(cerr, "Closing %s for copying from %s", dst, srcName)
		}
	}()

	amount, err = io.CopyBuffer(dstF, src, buf)
	if err != nil {
		err = uerr.Chainf(err, "Copying %s to %s", srcName, dst)
	} else if 0 < srcSz && amount != srcSz {
		err = fmt.Errorf("Copy of %s to %s failed: missing bytes: "+
			"srcSize=%d, got=%d", srcName, dst, srcSz, amount)
	}
	return
}

//
// hard link or copy file to specified directory, returning new name
//
func FileLinkOrCopyTo(file, dir string) (dst string, err error) {
	dst = path.Join(dir, path.Base(file))
	err = FileLinkOrCopy(file, dst)
	return
}

//
// move file to specified directory, returning new name
//
func FileMoveTo(file, dstDir string) (dst string, err error) {
	if 0 == len(dstDir) {
		err = errors.New("dstDir not provided")
		return
	}
	srcDir, srcF := path.Split(file)
	dst = path.Join(dstDir, srcF)
	if srcDir != dstDir {
		err = os.Rename(file, dst)
	}
	return
}

//
// symlink file to specified directory, returning new name
//
func FileSymlinkTo(file, dir string) (dst string, err error) {
	dst = path.Join(dir, path.Base(file))
	err = os.Symlink(file, dst)
	return
}

//
// can we stat the file?
//
func FileExists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}

//
// get the size of the file, or the files contained by file if file is a dir
//
func FileSize(file string) (int64, error) {
	fi, err := os.Stat(file)
	if err != nil {
		return 0, err
	} else if fi.IsDir() {
		return DiskUsage(file)
	} else {
		return fi.Size(), nil
	}
}

//
// get file mod time
//
func FileMTime(file string) (time.Time, error) {
	fi, err := os.Stat(file)
	if err != nil {
		return time.Unix(0, 0), err
	} else {
		return fi.ModTime(), nil
	}
}

//
// get file owner
//
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

//
// get file owner username
//
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

//
// is it a directory?
//
func FileIsDir(file string) bool {
	fi, err := os.Stat(file)
	return nil == err && fi.IsDir()
}

//
// update mod time of file, or create it
//
func FileTouch(file string, t time.Time) error {
	return os.Chtimes(file, t, t)
}

//
// compute the md5 of the file
//
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

type file_by_mtime []os.FileInfo

func (f file_by_mtime) Len() int      { return len(f) }
func (f file_by_mtime) Swap(i, j int) { f[i], f[j] = f[j], f[i] }
func (f file_by_mtime) Less(i, j int) bool {
	return f[i].ModTime().Before(f[j].ModTime())
}

// Sorted by mtime (oldest to youngest)
func SortByModTime(files []os.FileInfo) {
	if 1 < len(files) {
		sort.Sort(file_by_mtime(files))
	}
}

// Get listing of dir, sorted by mtime (oldest to youngest)
func FilesByModTime(dir string) (files []os.FileInfo, err error) {
	if dirF, err := os.Open(dir); err != nil {
		return files, err
	} else {
		defer dirF.Close()
		if files, err = dirF.Readdir(0); err != nil {
			return files, err
		}
		SortByModTime(files)
	}
	return
}

// watch a file.  if there is a change, then call onChange with fi set.
// if there is an error watching the file, call onChange with err set
func FileWatch(
	file string,
	period time.Duration,
	onChange func(file string, fi os.FileInfo, err error),
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

//
// Open file for create, run the filler, then close the file
// Ensures file closed properly
//
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

//
// Get the max allowed number of open files
//
func FdsMax() (currMax, allowedMax int, err error) {
	rlim := syscall.Rlimit{}
	err = syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlim)
	if err != nil {
		return
	}
	currMax = int(rlim.Cur)
	allowedMax = int(rlim.Max)
	return
}

//
// Return current valid file descriptors, up to maxFd.
//
// If maxFd is non-positive, then determine from rlimit
//
func FdsOpen(maxFd int) (fds []int, err error) {
	if 0 >= maxFd {
		maxFd, _, err = FdsMax()
		if err != nil {
			return
		}
	}
	stat := syscall.Stat_t{}
	for i := 0; i < maxFd; i++ {
		statErr := syscall.Fstat(i, &stat)
		if statErr != nil {
			errno, ok := statErr.(syscall.Errno)
			if ok {
				switch errno {
				case syscall.EBADF: // nothing to do
				default:
					log.Printf("Unable to fstat %d: %#v", i, statErr)
				}
			} else {
				log.Printf("Unable to fstat %d: %#v", i, statErr)
			}
		} else {
			fds = append(fds, i)
		}
	}
	return
}
