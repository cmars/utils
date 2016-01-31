[![Build Status](https://travis-ci.org/cmars/sigprof.svg?branch=master)](https://travis-ci.org/cmars/sigprof)

# sigprof
Golang package for inspecting running processes. Similar to [net/http/pprof](https://golang.org/pkg/net/http/pprof/) but using `USR1` and `USR2` signals instead of HTTP server routes.

# Usage
Link the package:

```go
import _ "github.com/cmars/sigprof"
```

Send the `USR1` or `USR2` signal to inspect the process.

```bash
kill -USR1 <golang process pid>
```

or

```bash
killall -USR2 <executable name>
```

By default, `sigprof` will save results to temp files.

```bash
go tool pprof /tmp/<executable name>.<profile type>.prof.<unique integer>
```

# Configuration

`sigprof` loads its configuration from the following environment variables.

* `SIGPROF_USR1` - Profile executed on the `USR1` signal. Default: `goroutine`
* `SIGPROF_USR2` - Profile executed on the `USR2` signal. Default: `heap`
* `SIGPROF_OUT` - Specify the output location, either `file`, `stderr`, or
  `stdout`. Default: `file`. Using anything other than `file` with `cpu`
  profiles is not recommended since it is a binary format.
