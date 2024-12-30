usftp
-----

An SFTP library, as specified by
[https://filezilla-project.org/specs/draft-ietf-secsh-filexfer-02.txt](https://filezilla-project.org/specs/draft-ietf-secsh-filexfer-02.txt)

This started out as [https://pkg.go.dev/github.com/pkg/sftp](https://pkg.go.dev/github.com/pkg/sftp),
and would not exist without the great work done there.

The goals for this package are:
* lower memory usage
* higher throughput
* fewer round trips (less latency)
* enable less abstracted access

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

Performance
-----------

While we still have work to do, performance is significantly improved.
* reducing allocations in most cases
* increasing throughput in most cases
* significantly reduced impact of latency

### usftp benchmark
```
go test -bench . -benchmem ./usftp/
```
```
goos: linux
goarch: amd64
pkg: github.com/tredeske/u/usftp
cpu: Intel(R) Xeon(R) Platinum 8488C
BenchmarkRead1k-8                      	       7	 162961489 ns/op	  64.35 MB/s	 7947804 B/op	  117844 allocs/op
BenchmarkRead16k-8                     	      88	  14949862 ns/op	 701.40 MB/s	 6047386 B/op	    7730 allocs/op
BenchmarkRead32k-8                     	     121	   9902739 ns/op	1058.89 MB/s	 6661250 B/op	    3910 allocs/op
BenchmarkRead128k-8                    	     194	   5980805 ns/op	1753.26 MB/s	 6775944 B/op	    2731 allocs/op
BenchmarkRead512k-8                    	     283	   4255154 ns/op	2464.28 MB/s	 7266450 B/op	    2554 allocs/op
BenchmarkRead1MiB-8                    	     298	   4029469 ns/op	2602.30 MB/s	 7923765 B/op	    2645 allocs/op
BenchmarkRead4MiB-8                    	     248	   4810317 ns/op	2179.87 MB/s	10559765 B/op	    3323 allocs/op
BenchmarkRead4MiBDelay10Msec-8         	      20	  56504192 ns/op	 185.58 MB/s	10581938 B/op	    3313 allocs/op
BenchmarkRead4MiBDelay50Msec-8         	       4	 256977088 ns/op	  40.80 MB/s	10571968 B/op	    3189 allocs/op
BenchmarkRead4MiBDelay150Msec-8        	       2	 758676654 ns/op	  13.82 MB/s	10574524 B/op	    3204 allocs/op
BenchmarkWrite1k-8                     	       3	 368453770 ns/op	  28.46 MB/s	16285938 B/op	  174152 allocs/op
BenchmarkWrite16k-8                    	      38	  31136818 ns/op	 336.77 MB/s	12081080 B/op	   11436 allocs/op
BenchmarkWrite32k-8                    	      61	  19519946 ns/op	 537.19 MB/s	13251158 B/op	    5781 allocs/op
BenchmarkWrite128k-8                   	      85	  12500312 ns/op	 838.85 MB/s	13237210 B/op	    4817 allocs/op
BenchmarkWrite512k-8                   	     134	   8924411 ns/op	1174.97 MB/s	13232805 B/op	    4554 allocs/op
BenchmarkWrite1MiB-8                   	     146	   8169570 ns/op	1283.53 MB/s	13231710 B/op	    4505 allocs/op
BenchmarkWrite4MiB-8                   	     157	   7649872 ns/op	1370.73 MB/s	13231273 B/op	    4475 allocs/op
BenchmarkWrite4MiBDelay10Msec-8        	      14	  78914710 ns/op	 132.88 MB/s	23728936 B/op	    4973 allocs/op
BenchmarkWrite4MiBDelay50Msec-8        	       3	 360331604 ns/op	  29.10 MB/s	23727800 B/op	    4794 allocs/op
BenchmarkWrite4MiBDelay150Msec-8       	       1	1060637846 ns/op	   9.89 MB/s	23736016 B/op	    4948 allocs/op
BenchmarkReadFrom1k-8                  	     152	   7716155 ns/op	1358.95 MB/s	13236754 B/op	    4632 allocs/op
BenchmarkReadFrom16k-8                 	     152	   7743238 ns/op	1354.20 MB/s	13236591 B/op	    4631 allocs/op
BenchmarkReadFrom32k-8                 	     153	   7694715 ns/op	1362.74 MB/s	13236852 B/op	    4639 allocs/op
BenchmarkReadFrom128k-8                	     152	   7742448 ns/op	1354.34 MB/s	13236355 B/op	    4625 allocs/op
BenchmarkReadFrom512k-8                	     151	   7740086 ns/op	1354.75 MB/s	13236765 B/op	    4635 allocs/op
BenchmarkReadFrom1MiB-8                	     152	   8205981 ns/op	1277.83 MB/s	13237490 B/op	    4657 allocs/op
BenchmarkReadFrom4MiB-8                	     151	   7894978 ns/op	1328.17 MB/s	13237073 B/op	    4648 allocs/op
BenchmarkReadFrom4MiBDelay10Msec-8     	      30	  39165915 ns/op	 267.73 MB/s	23740898 B/op	    4596 allocs/op
BenchmarkReadFrom4MiBDelay50Msec-8     	       7	 160743979 ns/op	  65.23 MB/s	23737406 B/op	    4223 allocs/op
BenchmarkReadFrom4MiBDelay150Msec-8    	       3	 461692848 ns/op	  22.71 MB/s	23733821 B/op	    4142 allocs/op
BenchmarkWriteTo1k-8                   	     193	   6021044 ns/op	1741.54 MB/s	13198142 B/op	    4098 allocs/op
BenchmarkWriteTo16k-8                  	     194	   5951886 ns/op	1761.77 MB/s	13198049 B/op	    4095 allocs/op
BenchmarkWriteTo32k-8                  	     192	   6067440 ns/op	1728.22 MB/s	13198018 B/op	    4093 allocs/op
BenchmarkWriteTo128k-8                 	     196	   6028461 ns/op	1739.40 MB/s	13197939 B/op	    4092 allocs/op
BenchmarkWriteTo512k-8                 	     196	   6090812 ns/op	1721.59 MB/s	13198032 B/op	    4095 allocs/op
BenchmarkWriteTo1MiB-8                 	     192	   6058390 ns/op	1730.80 MB/s	13198301 B/op	    4103 allocs/op
BenchmarkWriteTo4MiB-8                 	     195	   6070520 ns/op	1727.35 MB/s	13198060 B/op	    4096 allocs/op
BenchmarkWriteTo4MiBDelay10Msec-8      	      25	  47326963 ns/op	 221.56 MB/s	13209201 B/op	    4055 allocs/op
BenchmarkWriteTo4MiBDelay50Msec-8      	       5	 209843273 ns/op	  49.97 MB/s	13210521 B/op	    3883 allocs/op
BenchmarkWriteTo4MiBDelay150Msec-8     	       2	 612475772 ns/op	  17.12 MB/s	13210452 B/op	    3880 allocs/op
BenchmarkCopyDown10MiBDelay10Msec-8    	      22	  51149897 ns/op	 205.00 MB/s	13221540 B/op	    4061 allocs/op
BenchmarkCopyDown10MiBDelay50Msec-8    	       5	 216364325 ns/op	  48.46 MB/s	13209161 B/op	    3829 allocs/op
BenchmarkCopyDown10MiBDelay150Msec-8   	       2	 627764723 ns/op	  16.70 MB/s	13335236 B/op	    3853 allocs/op
BenchmarkCopyUp10MiBDelay10Msec-8      	      26	  43601592 ns/op	 240.49 MB/s	24394394 B/op	    4555 allocs/op
BenchmarkCopyUp10MiBDelay50Msec-8      	       6	 168712101 ns/op	  62.15 MB/s	23779912 B/op	    4269 allocs/op
BenchmarkCopyUp10MiBDelay150Msec-8     	       3	 473988576 ns/op	  22.12 MB/s	29413186 B/op	    4261 allocs/op
```

#### Comparison to Incumbent sftp

We ran benchmarks with count=10 for both usftp and sftp.

The benchmarks are essentially the same, with minor changes due to the minor API
changes.

While benchmarking the incumbent sftp library:
* BenchmarkWrite4MiDelay150Msec completed 1 round before quitting
* BenchmarkReadFrom4MiDelay105Msec completed 7 rounds before quitting
* BenchmarkCopyUp10MiDelay150Msec completed 4 rounds before quitting

The benchstat comparison shows significant improvement on most metrics
```
                            │ /tmp/sftp-bench.txt │         /tmp/usftp-bench.txt          │
                            │       sec/op        │   sec/op     vs base                  │
Read1k-8                            165.8m ± 3%     164.0m ± 2%        ~ (p=0.315 n=10)
Read16k-8                           16.58m ± 1%     15.48m ± 1%   -6.64% (p=0.000 n=10)
Read32k-8                          12.073m ± 1%     9.786m ± 2%  -18.94% (p=0.000 n=10)
Read128k-8                          7.890m ± 1%     6.306m ± 2%  -20.08% (p=0.000 n=10)
Read512k-8                          6.809m ± 1%     4.410m ± 2%  -35.24% (p=0.000 n=10)
Read1MiB-8                          6.636m ± 1%     4.179m ± 3%  -37.02% (p=0.000 n=10)
Read4MiB-8                          6.704m ± 2%     4.922m ± 1%  -26.59% (p=0.000 n=10)
Read4MiBDelay10Msec-8               67.15m ± 0%     56.80m ± 0%  -15.41% (p=0.000 n=10)
Read4MiBDelay50Msec-8               308.0m ± 0%     257.0m ± 0%  -16.56% (p=0.000 n=10)
Read4MiBDelay150Msec-8              908.5m ± 0%     757.8m ± 0%  -16.59% (p=0.000 n=10)
Write1k-8                           382.6m ± 1%     370.5m ± 3%   -3.16% (p=0.009 n=10)
Write16k-8                          31.76m ± 1%     34.26m ± 2%   +7.88% (p=0.000 n=10)
Write32k-8                          19.43m ± 3%     22.13m ± 3%  +13.92% (p=0.000 n=10)
Write128k-8                         19.76m ± 4%     13.86m ± 2%  -29.85% (p=0.000 n=10)
Write512k-8                        19.769m ± 2%     9.893m ± 1%  -49.96% (p=0.000 n=10)
Write1MiB-8                        19.571m ± 4%     9.086m ± 1%  -53.57% (p=0.000 n=10)
Write4MiB-8                        19.491m ± 5%     8.423m ± 2%  -56.78% (p=0.000 n=10)
Write4MiBDelay10Msec-8            3338.30m ± 0%     80.57m ± 0%  -97.59% (p=0.000 n=10)
Write4MiBDelay50Msec-8            16278.6m ± 0%     360.2m ± 0%  -97.79% (p=0.000 n=10)
Write4MiBDelay150Msec-8             48.642 ±  ∞ ¹    1.061 ± 0%        ~ (p=0.182 n=1+10)
ReadFrom1k-8                       20.432m ± 2%     8.334m ± 1%  -59.21% (p=0.000 n=10)
ReadFrom16k-8                      20.150m ± 4%     8.298m ± 1%  -58.82% (p=0.000 n=10)
ReadFrom32k-8                      20.445m ± 2%     8.338m ± 1%  -59.22% (p=0.000 n=10)
ReadFrom128k-8                     19.997m ± 4%     8.332m ± 2%  -58.33% (p=0.000 n=10)
ReadFrom512k-8                     20.159m ± 3%     8.361m ± 1%  -58.53% (p=0.000 n=10)
ReadFrom1MiB-8                     20.061m ± 2%     8.395m ± 1%  -58.15% (p=0.000 n=10)
ReadFrom4MiB-8                     21.335m ± 3%     8.468m ± 1%  -60.31% (p=0.000 n=10)
ReadFrom4MiBDelay10Msec-8         3337.73m ± 0%     40.52m ± 1%  -98.79% (p=0.000 n=10)
ReadFrom4MiBDelay50Msec-8         16274.2m ± 0%     162.3m ± 0%  -99.00% (p=0.000 n=10)
ReadFrom4MiBDelay150Msec-8        48658.2m ± 0%     461.2m ± 0%  -99.05% (p=0.000 n=7+10)
WriteTo1k-8                         8.013m ± 1%     6.147m ± 1%  -23.29% (p=0.000 n=10)
WriteTo16k-8                        7.960m ± 1%     6.113m ± 3%  -23.20% (p=0.000 n=10)
WriteTo32k-8                        7.991m ± 1%     6.182m ± 1%  -22.64% (p=0.000 n=10)
WriteTo128k-8                       7.868m ± 1%     6.168m ± 1%  -21.61% (p=0.000 n=10)
WriteTo512k-8                       7.900m ± 1%     6.102m ± 2%  -22.76% (p=0.000 n=10)
WriteTo1MiB-8                       7.920m ± 2%     6.164m ± 2%  -22.18% (p=0.000 n=10)
WriteTo4MiB-8                       7.900m ± 1%     6.129m ± 1%  -22.42% (p=0.000 n=10)
WriteTo4MiBDelay10Msec-8            98.05m ± 1%     48.28m ± 1%  -50.76% (p=0.000 n=10)
WriteTo4MiBDelay50Msec-8            460.6m ± 0%     209.5m ± 0%  -54.52% (p=0.000 n=10)
WriteTo4MiBDelay150Msec-8          1366.9m ± 0%     612.6m ± 0%  -55.18% (p=0.000 n=10)
CopyDown10MiBDelay10Msec-8         104.38m ± 1%     51.50m ± 1%  -50.66% (p=0.000 n=10)
CopyDown10MiBDelay50Msec-8          472.0m ± 0%     216.6m ± 0%  -54.11% (p=0.000 n=10)
CopyDown10MiBDelay150Msec-8        1393.0m ± 0%     626.7m ± 0%  -55.01% (p=0.000 n=10)
CopyUp10MiBDelay10Msec-8          3369.01m ± 0%     44.07m ± 1%  -98.69% (p=0.000 n=10)
CopyUp10MiBDelay50Msec-8          16274.6m ± 0%     169.7m ± 0%  -98.96% (p=0.000 n=10)
CopyUp10MiBDelay150Msec-8         48550.0m ±  ∞ ¹   474.5m ± 0%  -99.02% (p=0.002 n=4+10)
geomean                             110.9m          32.91m       -70.34%
¹ need >= 6 samples for confidence interval at level 0.95

                            │ /tmp/sftp-bench.txt │            /tmp/usftp-bench.txt             │
                            │         B/s         │      B/s        vs base                     │
Read1k-8                           60.30Mi ± 3%       60.98Mi ± 2%           ~ (p=0.315 n=10)
Read16k-8                          603.0Mi ± 1%       645.9Mi ± 1%      +7.11% (p=0.000 n=10)
Read32k-8                          828.3Mi ± 1%      1021.9Mi ± 2%     +23.37% (p=0.000 n=10)
Read128k-8                         1.238Gi ± 1%       1.549Gi ± 2%     +25.12% (p=0.000 n=10)
Read512k-8                         1.434Gi ± 1%       2.215Gi ± 2%     +54.41% (p=0.000 n=10)
Read1MiB-8                         1.472Gi ± 1%       2.337Gi ± 3%     +58.78% (p=0.000 n=10)
Read4MiB-8                         1.457Gi ± 3%       1.984Gi ± 1%     +36.22% (p=0.000 n=10)
Read4MiBDelay10Msec-8              148.9Mi ± 0%       176.1Mi ± 0%     +18.22% (p=0.000 n=10)
Read4MiBDelay50Msec-8              32.47Mi ± 0%       38.91Mi ± 0%     +19.83% (p=0.000 n=10)
Read4MiBDelay150Msec-8             11.01Mi ± 0%       13.20Mi ± 0%     +19.93% (p=0.000 n=10)
Write1k-8                          26.14Mi ± 1%       26.99Mi ± 3%      +3.27% (p=0.007 n=10)
Write16k-8                         314.8Mi ± 1%       291.9Mi ± 2%      -7.30% (p=0.000 n=10)
Write32k-8                         514.8Mi ± 3%       451.9Mi ± 3%     -12.22% (p=0.000 n=10)
Write128k-8                        506.1Mi ± 4%       721.5Mi ± 2%     +42.56% (p=0.000 n=10)
Write512k-8                        505.9Mi ± 2%      1010.8Mi ± 1%     +99.82% (p=0.000 n=10)
Write1MiB-8                        511.0Mi ± 4%      1100.6Mi ± 1%    +115.38% (p=0.000 n=10)
Write4MiB-8                        513.1Mi ± 5%      1187.2Mi ± 2%    +131.40% (p=0.000 n=10)
Write4MiBDelay10Msec-8             2.995Mi ± 0%     124.121Mi ± 0%   +4044.90% (p=0.000 n=10)
Write4MiBDelay50Msec-8             625.0Ki ± 0%     28432.6Ki ± 0%   +4449.22% (p=0.000 n=10)
Write4MiBDelay150Msec-8            214.8Ki ±  ∞ ¹    9653.3Ki ± 0%           ~ (p=0.182 n=1+10)
ReadFrom1k-8                       489.4Mi ± 2%      1200.0Mi ± 1%    +145.17% (p=0.000 n=10)
ReadFrom16k-8                      496.3Mi ± 4%      1205.1Mi ± 1%    +142.81% (p=0.000 n=10)
ReadFrom32k-8                      489.1Mi ± 2%      1199.4Mi ± 1%    +145.21% (p=0.000 n=10)
ReadFrom128k-8                     500.1Mi ± 4%      1200.2Mi ± 2%    +140.00% (p=0.000 n=10)
ReadFrom512k-8                     496.1Mi ± 3%      1196.1Mi ± 1%    +141.12% (p=0.000 n=10)
ReadFrom1MiB-8                     498.5Mi ± 2%      1191.1Mi ± 1%    +138.95% (p=0.000 n=10)
ReadFrom4MiB-8                     468.7Mi ± 3%      1181.0Mi ± 1%    +151.96% (p=0.000 n=10)
ReadFrom4MiBDelay10Msec-8          2.995Mi ± 0%     246.820Mi ± 1%   +8142.36% (p=0.000 n=10)
ReadFrom4MiBDelay50Msec-8          625.0Ki ± 0%     63110.4Ki ± 0%   +9997.66% (p=0.000 n=10)
ReadFrom4MiBDelay150Msec-8         214.8Ki ± 0%     22202.1Ki ± 0%  +10234.09% (p=0.000 n=7+10)
WriteTo1k-8                        1.219Gi ± 1%       1.589Gi ± 1%     +30.35% (p=0.000 n=10)
WriteTo16k-8                       1.227Gi ± 1%       1.597Gi ± 2%     +30.20% (p=0.000 n=10)
WriteTo32k-8                       1.222Gi ± 1%       1.580Gi ± 2%     +29.26% (p=0.000 n=10)
WriteTo128k-8                      1.241Gi ± 1%       1.583Gi ± 1%     +27.56% (p=0.000 n=10)
WriteTo512k-8                      1.236Gi ± 1%       1.601Gi ± 2%     +29.47% (p=0.000 n=10)
WriteTo1MiB-8                      1.233Gi ± 2%       1.584Gi ± 2%     +28.50% (p=0.000 n=10)
WriteTo4MiB-8                      1.236Gi ± 1%       1.593Gi ± 1%     +28.89% (p=0.000 n=10)
WriteTo4MiBDelay10Msec-8           102.0Mi ± 1%       207.1Mi ± 1%    +103.10% (p=0.000 n=10)
WriteTo4MiBDelay50Msec-8           21.71Mi ± 0%       47.73Mi ± 0%    +119.86% (p=0.000 n=10)
WriteTo4MiBDelay150Msec-8          7.315Mi ± 0%      16.322Mi ± 0%    +123.14% (p=0.000 n=10)
CopyDown10MiBDelay10Msec-8         95.80Mi ± 1%      194.16Mi ± 1%    +102.68% (p=0.000 n=10)
CopyDown10MiBDelay50Msec-8         21.19Mi ± 0%       46.16Mi ± 0%    +117.87% (p=0.000 n=10)
CopyDown10MiBDelay150Msec-8        7.181Mi ± 0%      15.955Mi ± 0%    +122.18% (p=0.000 n=10)
CopyUp10MiBDelay10Msec-8           2.966Mi ± 0%     226.932Mi ± 1%   +7551.29% (p=0.000 n=10)
CopyUp10MiBDelay50Msec-8           625.0Ki ± 0%     60322.3Ki ± 0%   +9551.56% (p=0.000 n=10)
CopyUp10MiBDelay150Msec-8          214.8Ki ±  ∞ ¹   21582.0Ki ± 0%   +9945.45% (p=0.002 n=4+10)
geomean                            90.21Mi            303.9Mi         +236.84%
¹ need >= 6 samples for confidence interval at level 0.95

                            │ /tmp/sftp-bench.txt │          /tmp/usftp-bench.txt           │
                            │        B/op         │     B/op       vs base                  │
Read1k-8                          13.285Mi ± 0%     7.579Mi ± 31%  -42.95% (p=0.000 n=10)
Read16k-8                         11.415Mi ± 0%     5.767Mi ±  0%  -49.48% (p=0.000 n=10)
Read32k-8                         12.644Mi ± 0%     6.352Mi ±  0%  -49.76% (p=0.000 n=10)
Read128k-8                        12.926Mi ± 0%     6.462Mi ±  1%  -50.01% (p=0.000 n=10)
Read512k-8                        13.854Mi ± 0%     6.930Mi ±  0%  -49.98% (p=0.000 n=10)
Read1MiB-8                        15.108Mi ± 0%     7.556Mi ±  0%  -49.98% (p=0.000 n=10)
Read4MiB-8                         20.12Mi ± 0%     10.07Mi ±  0%  -49.94% (p=0.000 n=10)
Read4MiBDelay10Msec-8              20.13Mi ± 0%     10.08Mi ±  0%  -49.91% (p=0.000 n=10)
Read4MiBDelay50Msec-8              20.13Mi ± 0%     10.08Mi ±  0%  -49.92% (p=0.000 n=10)
Read4MiBDelay150Msec-8             20.13Mi ± 0%     10.08Mi ±  1%  -49.93% (p=0.000 n=10)
Write1k-8                          16.80Mi ± 0%     15.53Mi ±  0%   -7.55% (p=0.000 n=10)
Write16k-8                         11.60Mi ± 0%     11.52Mi ±  0%   -0.69% (p=0.000 n=10)
Write32k-8                         12.68Mi ± 0%     12.64Mi ±  0%   -0.32% (p=0.000 n=10)
Write128k-8                        12.64Mi ± 0%     12.62Mi ±  0%   -0.13% (p=0.000 n=10)
Write512k-8                        12.63Mi ± 0%     12.62Mi ±  0%   -0.09% (p=0.000 n=10)
Write1MiB-8                        12.63Mi ± 0%     12.62Mi ±  0%   -0.07% (p=0.000 n=10)
Write4MiB-8                        12.63Mi ± 0%     12.62Mi ±  0%   -0.05% (p=0.000 n=10)
Write4MiBDelay10Msec-8             22.64Mi ± 0%     22.63Mi ±  0%   -0.04% (p=0.000 n=10)
Write4MiBDelay50Msec-8             22.64Mi ± 0%     22.63Mi ±  0%   -0.03% (p=0.000 n=10)
Write4MiBDelay150Msec-8            22.64Mi ±  ∞ ¹   22.64Mi ±  0%        ~ (p=0.364 n=1+10)
ReadFrom1k-8                       12.66Mi ± 0%     12.62Mi ±  0%   -0.28% (p=0.000 n=10)
ReadFrom16k-8                      12.66Mi ± 0%     12.62Mi ±  0%   -0.28% (p=0.000 n=10)
ReadFrom32k-8                      12.66Mi ± 0%     12.62Mi ±  0%   -0.28% (p=0.000 n=10)
ReadFrom128k-8                     12.66Mi ± 0%     12.62Mi ±  0%   -0.28% (p=0.000 n=10)
ReadFrom512k-8                     12.66Mi ± 0%     12.62Mi ±  0%   -0.28% (p=0.000 n=10)
ReadFrom1MiB-8                     12.66Mi ± 0%     12.62Mi ±  0%   -0.28% (p=0.000 n=10)
ReadFrom4MiB-8                     12.66Mi ± 0%     12.62Mi ±  0%   -0.28% (p=0.000 n=10)
ReadFrom4MiBDelay10Msec-8          22.67Mi ± 0%     22.64Mi ±  0%   -0.14% (p=0.000 n=10)
ReadFrom4MiBDelay50Msec-8          22.67Mi ± 0%     22.63Mi ±  0%   -0.15% (p=0.000 n=10)
ReadFrom4MiBDelay150Msec-8         22.67Mi ± 0%     22.64Mi ±  0%   -0.11% (p=0.000 n=7+10)
WriteTo1k-8                        28.04Mi ± 0%     12.59Mi ±  0%  -55.11% (p=0.000 n=10)
WriteTo16k-8                       28.03Mi ± 0%     12.59Mi ±  0%  -55.10% (p=0.000 n=10)
WriteTo32k-8                       28.03Mi ± 0%     12.59Mi ±  0%  -55.10% (p=0.000 n=10)
WriteTo128k-8                      28.03Mi ± 0%     12.59Mi ±  0%  -55.10% (p=0.000 n=10)
WriteTo512k-8                      28.04Mi ± 0%     12.59Mi ±  0%  -55.11% (p=0.000 n=10)
WriteTo1MiB-8                      28.03Mi ± 0%     12.59Mi ±  0%  -55.10% (p=0.000 n=10)
WriteTo4MiB-8                      28.04Mi ± 0%     12.59Mi ±  0%  -55.11% (p=0.000 n=10)
WriteTo4MiBDelay10Msec-8           28.07Mi ± 0%     12.60Mi ±  0%  -55.12% (p=0.000 n=10)
WriteTo4MiBDelay50Msec-8           28.08Mi ± 0%     12.60Mi ±  0%  -55.14% (p=0.000 n=10)
WriteTo4MiBDelay150Msec-8          28.04Mi ± 0%     12.60Mi ±  0%  -55.08% (p=0.000 n=10)
CopyDown10MiBDelay10Msec-8         28.20Mi ± 0%     13.34Mi ±  0%  -52.70% (p=0.000 n=10)
CopyDown10MiBDelay50Msec-8         28.16Mi ± 0%     15.84Mi ± 20%  -43.73% (p=0.000 n=10)
CopyDown10MiBDelay150Msec-8        28.12Mi ± 0%     12.72Mi ± 63%  -54.79% (p=0.000 n=10)
CopyUp10MiBDelay10Msec-8           22.67Mi ± 0%     23.28Mi ±  0%   +2.68% (p=0.001 n=10)
CopyUp10MiBDelay50Msec-8           22.67Mi ± 0%     25.34Mi ± 11%  +11.77% (p=0.000 n=10)
CopyUp10MiBDelay150Msec-8          22.67Mi ±  ∞ ¹   25.38Mi ± 11%  +11.97% (p=0.002 n=4+10)
geomean                            18.67Mi          12.96Mi        -30.55%
¹ need >= 6 samples for confidence interval at level 0.95

                            │ /tmp/sftp-bench.txt │          /tmp/usftp-bench.txt          │
                            │      allocs/op      │  allocs/op    vs base                  │
Read1k-8                            76.85k ± 0%     117.84k ± 0%  +53.33% (p=0.000 n=10)
Read16k-8                           5.133k ± 0%      7.720k ± 0%  +50.39% (p=0.000 n=10)
Read32k-8                           2.600k ± 0%      3.909k ± 0%  +50.35% (p=0.000 n=10)
Read128k-8                          3.594k ± 0%      2.731k ± 0%  -24.01% (p=0.000 n=10)
Read512k-8                          3.644k ± 0%      2.563k ± 0%  -29.66% (p=0.000 n=10)
Read1MiB-8                          3.892k ± 0%      2.651k ± 0%  -31.89% (p=0.000 n=10)
Read4MiB-8                          4.769k ± 1%      3.328k ± 0%  -30.23% (p=0.000 n=10)
Read4MiBDelay10Msec-8               4.978k ± 0%      3.337k ± 0%  -32.96% (p=0.000 n=10)
Read4MiBDelay50Msec-8               4.880k ± 1%      3.149k ± 1%  -35.46% (p=0.000 n=10)
Read4MiBDelay150Msec-8              4.899k ± 1%      3.152k ± 1%  -35.65% (p=0.000 n=10)
Write1k-8                           163.9k ± 0%      174.1k ± 0%   +6.25% (p=0.000 n=10)
Write16k-8                          10.79k ± 0%      11.43k ± 0%   +5.88% (p=0.000 n=10)
Write32k-8                          5.460k ± 0%      5.770k ± 0%   +5.69% (p=0.000 n=10)
Write128k-8                         4.979k ± 0%      4.839k ± 0%   -2.80% (p=0.000 n=10)
Write512k-8                         4.859k ± 0%      4.586k ± 0%   -5.61% (p=0.000 n=10)
Write1MiB-8                         4.840k ± 0%      4.583k ± 1%   -5.30% (p=0.000 n=10)
Write4MiB-8                         4.824k ± 0%      4.622k ± 1%   -4.18% (p=0.000 n=10)
Write4MiBDelay10Msec-8              5.201k ± 0%      4.947k ± 1%   -4.87% (p=0.000 n=10)
Write4MiBDelay50Msec-8              5.197k ± 0%      4.882k ± 1%   -6.06% (p=0.000 n=10)
Write4MiBDelay150Msec-8             5.198k ±  ∞ ¹    4.968k ± 1%        ~ (p=0.182 n=1+10)
ReadFrom1k-8                        4.822k ± 0%      4.657k ± 0%   -3.41% (p=0.000 n=10)
ReadFrom16k-8                       4.822k ± 0%      4.660k ± 0%   -3.36% (p=0.000 n=10)
ReadFrom32k-8                       4.819k ± 0%      4.663k ± 0%   -3.22% (p=0.000 n=10)
ReadFrom128k-8                      4.821k ± 0%      4.666k ± 0%   -3.22% (p=0.000 n=10)
ReadFrom512k-8                      4.822k ± 0%      4.669k ± 0%   -3.16% (p=0.000 n=10)
ReadFrom1MiB-8                      4.822k ± 0%      4.666k ± 0%   -3.23% (p=0.000 n=10)
ReadFrom4MiB-8                      4.821k ± 0%      4.673k ± 0%   -3.07% (p=0.000 n=10)
ReadFrom4MiBDelay10Msec-8           5.199k ± 0%      4.511k ± 1%  -13.23% (p=0.000 n=10)
ReadFrom4MiBDelay50Msec-8           5.197k ± 0%      4.165k ± 1%  -19.86% (p=0.000 n=10)
ReadFrom4MiBDelay150Msec-8          5.196k ± 0%      4.330k ± 2%  -16.67% (p=0.000 n=7+10)
WriteTo1k-8                         7.367k ± 0%      4.111k ± 0%  -44.20% (p=0.000 n=10)
WriteTo16k-8                        7.361k ± 0%      4.107k ± 0%  -44.20% (p=0.000 n=10)
WriteTo32k-8                        7.362k ± 0%      4.106k ± 0%  -44.22% (p=0.000 n=10)
WriteTo128k-8                       7.364k ± 0%      4.106k ± 0%  -44.25% (p=0.000 n=10)
WriteTo512k-8                       7.364k ± 0%      4.106k ± 0%  -44.24% (p=0.000 n=10)
WriteTo1MiB-8                       7.364k ± 0%      4.101k ± 0%  -44.32% (p=0.000 n=10)
WriteTo4MiB-8                       7.367k ± 0%      4.100k ± 0%  -44.34% (p=0.000 n=10)
WriteTo4MiBDelay10Msec-8            7.630k ± 0%      4.100k ± 1%  -46.26% (p=0.000 n=10)
WriteTo4MiBDelay50Msec-8            7.540k ± 1%      3.847k ± 1%  -48.98% (p=0.000 n=10)
WriteTo4MiBDelay150Msec-8           7.600k ± 1%      3.829k ± 1%  -49.61% (p=0.000 n=10)
CopyDown10MiBDelay10Msec-8          7.622k ± 0%      4.053k ± 0%  -46.82% (p=0.000 n=10)
CopyDown10MiBDelay50Msec-8          7.593k ± 1%      3.821k ± 1%  -49.67% (p=0.000 n=10)
CopyDown10MiBDelay150Msec-8         7.649k ± 2%      3.805k ± 1%  -50.25% (p=0.000 n=10)
CopyUp10MiBDelay10Msec-8            5.191k ± 0%      4.528k ± 1%  -12.77% (p=0.000 n=10)
CopyUp10MiBDelay50Msec-8            5.194k ± 0%      4.133k ± 2%  -20.41% (p=0.000 n=10)
CopyUp10MiBDelay150Msec-8           5.193k ±  ∞ ¹    4.269k ± 2%  -17.79% (p=0.002 n=4+10)
geomean                             6.306k           4.965k       -21.26%
¹ need >= 6 samples for confidence interval at level 0.95
```
