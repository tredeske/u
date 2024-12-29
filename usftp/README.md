usftp
-----

This started out as [https://pkg.go.dev/github.com/pkg/sftp](https://pkg.go.dev/github.com/pkg/sftp), and would not exist without the great work done there.

The goals for this package are:
* lower memory usage
* higher throughput
* fewer round trips (less latency)

This package is greatly cut down from the original, providing only the client
portion, and focusing on those goals.

The interface is very similar to the original, except:
* The go stdlib fs is supported instead of github.com/kr/fs
* Asynchronous operations are supported
* ReadDir produces a `[]*File`, not `[]os.FileInfo`
   * Avoids having to issue a duplicate Stat when using each File
* File
   * May be open or closed
   * Caches file attributes
   * When created from ReadDir, already has attributes
   * Operations that require size attribute do not cause a Stat by default
   * ReadFrom does not require use with certain kinds of io.Readers

usage and examples
------------------

See [https://pkg.go.dev/github.com/pkg/sftp](https://pkg.go.dev/github.com/pkg/sftp) for
examples and usage.



contributing
------------

We welcome pull requests, bug fixes and issue reports.

Before proposing a large change, first please discuss your change by raising an
issue.

For API/code bugs, please include a small, self contained code example to
reproduce the issue. For pull requests, remember test coverage.

We try to handle issues and pull requests with a 0 open philosophy. That means
we will try to address the submission as soon as possible and will work toward
a resolution. If progress can no longer be made (eg. unreproducible bug) or
stops (eg. unresponsive submitter), we will close the bug.

Thanks.
