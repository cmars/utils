// +build !windows

// Package sigprof provides signal-triggered profiling.
package sigprof

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"
)

func init() {
	s := newSigprof()
	go s.loop()
}

type stderrWriter struct{}

// Write implements io.Writer.
func (w stderrWriter) Write(p []byte) (int, error) {
	return os.Stderr.Write(p)
}

// Close implements io.Closer.
func (w stderrWriter) Close() error {
	return nil
}

type stdoutWriter struct{}

// Write implements io.Writer.
func (w stdoutWriter) Write(p []byte) (int, error) {
	return os.Stdout.Write(p)
}

// Close implements io.Closer.
func (w stdoutWriter) Close() error {
	return nil
}

type outputType string

const (
	stdoutOutput = outputType("stdout")
	stderrOutput = outputType("stderr")
	fileOutput   = outputType("file")
)

type sigprof struct {
	usr1, usr2 []string
	output     outputType

	writerFactory   func(profile string, output outputType) io.WriteCloser
	profilerFactory func() profiler
	sigChanFactory  func() <-chan (os.Signal)
}

func newSigprof() sigprof {
	s := sigprof{
		writerFactory:   newWriter,
		profilerFactory: newProfiler,
		sigChanFactory:  newSigChan,
	}

	usr1EnvStr := os.Getenv(`SIGPROF_USR1`)
	if usr1EnvStr == "" {
		usr1EnvStr = "goroutine"
	}
	s.usr1 = strings.Split(usr1EnvStr, ",")

	usr2EnvStr := os.Getenv(`SIGPROF_USR2`)
	if usr2EnvStr == "" {
		usr2EnvStr = "heap"
	}
	s.usr2 = strings.Split(usr2EnvStr, ",")

	output := os.Getenv(`SIGPROF_OUT`)
	if output == "" {
		output = "file"
	}
	s.output = outputType(output)

	return s
}

// loop handles signals and writes profiles.
func (s *sigprof) loop() {
	c := s.sigChanFactory()
	for {
		select {
		case sig, ok := <-c:
			if !ok {
				return
			}
			s.profileSignal(sig)
		}
	}
}

func newSigChan() <-chan (os.Signal) {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGUSR1, syscall.SIGUSR2)
	return c
}

// profileSignal writes the profiles for the given signal.
func (s *sigprof) profileSignal(sig os.Signal) {
	var profiles []string
	switch sig {
	case syscall.SIGUSR1:
		profiles = s.usr1
	case syscall.SIGUSR2:
		profiles = s.usr2
	default:
		return
	}

	for _, profile := range profiles {
		w := s.writer(profile)
		s.profile(profile, w)
	}
}

// writer returns an io.WriteCloser to where the profile should be written.
func (s *sigprof) writer(profile string) io.WriteCloser {
	return s.writerFactory(profile, s.output)
}

func newWriter(profile string, output outputType) io.WriteCloser {
	switch output {
	case "file":
		f, err := ioutil.TempFile("", fmt.Sprintf("%s.%s.prof.", filepath.Base(os.Args[0]), profile))
		if err != nil {
			log.Printf("failed to create file for %s profile: %v", profile, err)
			return stderrWriter{}
		}
		log.Printf("writing %s profile to %s", profile, f.Name())
		return f
	case "stdout":
		return stdoutWriter{}
	case "stderr":
		return stderrWriter{}
	default:
		return stderrWriter{}
	}
}

type profiler interface {
	writeProfile(w io.WriteCloser, profileName string) error
}

type pprofiler struct{}

func (p *pprofiler) writeProfile(w io.WriteCloser, profileName string) error {
	if profileName == "cpu" {
		return p.cpuProfile(w)
	}
	prof := pprof.Lookup(profileName)
	if prof == nil {
		return fmt.Errorf("failed to lookup profile %q", profileName)
	}
	return prof.WriteTo(w, 1)
}

var cpuProfileDuration = 30 * time.Second

func (p *pprofiler) cpuProfile(w io.WriteCloser) error {
	err := pprof.StartCPUProfile(w)
	if err != nil {
		return fmt.Errorf("failed to start CPU profiling: %v", err)
	}
	go func() {
		time.Sleep(cpuProfileDuration)
		log.Println("cpu profile complete")
		pprof.StopCPUProfile()
		err := w.Close()
		if err != nil {
			log.Println("error closing file: %v", err)
		}
	}()
	return nil
}

func newProfiler() profiler {
	return &pprofiler{}
}

func (s *sigprof) profile(profileName string, w io.WriteCloser) {
	p := s.profilerFactory()
	err := p.writeProfile(w, profileName)
	if err != nil {
		log.Printf("failed to write %s profile: %v", profileName, err)
		if f, ok := w.(*os.File); ok {
			err = f.Close()
			if err != nil {
				log.Printf("error closing file: %v", err)
			}
			err = os.Remove(f.Name())
			if err != nil {
				log.Printf("cleanup error removing file: %v", err)
			}
			return
		}
	}
	if profileName != "cpu" {
		w.Close()
	}
}
