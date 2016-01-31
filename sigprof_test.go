// +build !windows

package sigprof

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"syscall"
	"testing"
	"time"
)

type bufferCloser struct {
	*bytes.Buffer
}

func (bufferCloser) Close() error { return nil }

type testProfiler struct{}

func (testProfiler) writeProfile(w io.WriteCloser, profileName string) error {
	fmt.Fprintf(w, "test %s\n", profileName)
	return nil
}

func TestStubs(t *testing.T) {
	outputs := map[string]*bytes.Buffer{}
	s := sigprof{
		usr1:   []string{"foo", "bar"},
		usr2:   []string{"baz", "quux"},
		output: "orange",
		sigChanFactory: func() <-chan (os.Signal) {
			c := make(chan os.Signal)
			go func() {
				c <- syscall.SIGUSR1
				c <- syscall.SIGUSR2
				c <- syscall.SIGHUP
				close(c)
			}()
			return c
		},
		writerFactory: func(profile string, out outputType) io.WriteCloser {
			if out != "orange" {
				t.Fatalf("unexpected output %q", out)
			}
			var buf bytes.Buffer
			outputs[profile] = &buf
			return bufferCloser{&buf}
		},
		profilerFactory: func() profiler {
			return testProfiler{}
		},
	}

	s.loop()

	if len(outputs) != 4 {
		t.Errorf("unexpected outputs len=%d", len(outputs))
	}
	for _, profile := range []string{"foo", "bar", "baz", "quux"} {
		buf, ok := outputs[profile]
		if !ok {
			t.Errorf("missing expected profile %q", profile)
		}
		if buf.String() != "test "+profile+"\n" {
			t.Errorf("unexpected profiler contents: %q", buf.String())
		}
	}
}

func TestPprof(t *testing.T) {
	outputs := []*bytes.Buffer{}
	s := sigprof{
		usr1:   []string{"goroutine"},
		usr2:   []string{"heap"},
		output: "file",
		writerFactory: func(profile string, out outputType) io.WriteCloser {
			var buf bytes.Buffer
			outputs = append(outputs, &buf)
			return bufferCloser{&buf}
		},
		sigChanFactory: func() <-chan os.Signal {
			ch := make(chan os.Signal)
			go func() {
				for i := 0; i < 100; i++ {
					ch <- syscall.SIGUSR1
					ch <- syscall.SIGUSR2
				}
				close(ch)
			}()
			return ch
		},
		profilerFactory: newProfiler,
	}

	s.loop()

	if len(outputs) != 200 {
		t.Errorf("unexpected number of profiles: %d", len(outputs))
	}

	var nHeap, nGoroutine int
	for _, output := range outputs {
		if strings.Contains(output.String(), "goroutine profile") {
			nGoroutine++
		} else if strings.Contains(output.String(), "heap profile") {
			nHeap++
		}
	}
	if nGoroutine != 100 {
		t.Errorf("unexpected goroutine profile count: %d", nGoroutine)
	}
	if nHeap != 100 {
		t.Errorf("unexpected heap profile count: %d", nHeap)
	}
}

func TestCPU(t *testing.T) {
	origDuration := cpuProfileDuration
	cpuProfileDuration = 0
	defer func() {
		cpuProfileDuration = origDuration
	}()

	outputs := []*bytes.Buffer{}
	s := sigprof{
		usr1:   []string{"cpu"},
		output: "file",
		writerFactory: func(profile string, out outputType) io.WriteCloser {
			var buf bytes.Buffer
			outputs = append(outputs, &buf)
			return bufferCloser{&buf}
		},
		sigChanFactory: func() <-chan os.Signal {
			ch := make(chan os.Signal)
			go func() {
				ch <- syscall.SIGUSR1
				close(ch)
			}()
			return ch
		},
		profilerFactory: newProfiler,
	}

	s.loop()

	// Try to start another CPU profile. This will fail if the above triggered
	// profile is still running.
	var profCompleted bool
	for i := 0; i < 5; i++ {
		err := pprof.StartCPUProfile(ioutil.Discard)
		if err == nil {
			pprof.StopCPUProfile()
			profCompleted = true
			break
		}
		t.Logf("cpu profile likely still running, sleeping %ds", i)
		time.Sleep(time.Duration(i) * time.Second)
	}
	if !profCompleted {
		t.Fatal("timeout waiting for cpu profile to complete")
	}

	if len(outputs) != 1 {
		t.Errorf("unexpected number of profiles: %d", len(outputs))
	}
	if outputs[0].Len() == 0 {
		t.Errorf("empty profile contents")
	}
}

func TestWriter(t *testing.T) {
	stdout := newWriter("blips", "stdout")
	if _, ok := stdout.(stdoutWriter); !ok {
		t.Errorf("stdout: got a %T instead", stdout)
	}
	stderr := newWriter("blops", "stderr")
	if _, ok := stderr.(stderrWriter); !ok {
		t.Errorf("stderr: got a %T instead", stderr)
	}
	whatever := newWriter("blups", "whatever")
	if _, ok := whatever.(stderrWriter); !ok {
		t.Errorf("default: got a %T instead", whatever)
	}
	file := newWriter("nitpicks", "file")
	if f, ok := file.(*os.File); !ok {
		t.Errorf("file: got a %T instead", file)
	} else {
		defer os.Remove(f.Name())
		defer file.Close()
		if !strings.Contains(filepath.Base(f.Name()), "nitpicks.prof.") {
			t.Errorf("file: unexpected file name %q", f.Name())
		}
	}
}

type errorProfiler struct{}

func (errorProfiler) writeProfile(w io.WriteCloser, profileName string) error {
	return errors.New("testing write failed")
}

func TestWriteProfilerErr(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	fileName := f.Name()
	defer os.Remove(fileName)

	s := sigprof{
		profilerFactory: func() profiler {
			return errorProfiler{}
		},
	}
	s.profile("blops", f)
	if _, err := os.Stat(fileName); !os.IsNotExist(err) {
		t.Fatal("profiler error failed to clean up output file")
	}
	if err := f.Close(); err == nil {
		t.Fatal("profiler left file open")
	}
}
