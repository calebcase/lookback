package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
)

type Ring struct {
	size   int
	buffer []byte
}

func NewRing(size int) *Ring {
	return &Ring{
		size:   size,
		buffer: make([]byte, 0, size),
	}
}

func (r *Ring) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	if len(p) > cap(r.buffer) {
		return 0, errors.New("Cannot write buffer larger than ring.")
	}

	if len(r.buffer)+len(p) > r.size {
		r.buffer = r.buffer[len(p):]
	}

	r.buffer = append(r.buffer, p...)

	return len(p), nil
}

func (r *Ring) Dump() []byte {
	buffer := r.buffer
	r.buffer = make([]byte, 0, r.size)

	return buffer
}

type State struct {
	size int
	file *os.File

	Path   string
	Output bool
	Ring   *Ring

	sync.Mutex
}

func NewState(size int, path string) *State {
	return &State{
		size: size,
		Path: path,
		Ring: NewRing(size),
	}
}

func (s *State) Start() {
	s.Lock()
	defer s.Unlock()

	file, err := os.Create(s.Path)
	if err != nil {
		log.Fatal(err)
	}
	s.file = file

	s.file.Write(s.Ring.Dump())
	s.Output = true
}

func (s *State) Stop() {
	s.Lock()
	defer s.Unlock()

	s.file.Close()
	s.Output = false
}

func (s *State) Write(p []byte) (n int, err error) {
	s.Lock()
	defer s.Unlock()

	if s.Output {
		return s.file.Write(p)
	} else {
		return s.Ring.Write(p)
	}
}

func main() {
	output_path := os.Args[1]

	signals := make(chan os.Signal, 1)

	watching := []os.Signal{
		syscall.SIGUSR1,
		syscall.SIGUSR2,
	}
	signal.Notify(signals, watching...)

	sample_size, err := strconv.Atoi(os.Args[2])
	if err != nil {
		log.Fatal(err)
	}

	ring_size, err := strconv.Atoi(os.Args[3])
	if err != nil {
		log.Fatal(err)
	}

	state := NewState(ring_size, output_path)

	go func() {
		for {
			sig := <-signals
			switch sig {
			case syscall.SIGUSR1:
				state.Start()
			case syscall.SIGUSR2:
				state.Stop()
			}
		}
	}()

	buf := make([]byte, sample_size)
	for {
		n, err := io.ReadAtLeast(os.Stdin, buf, len(buf))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		n, err = state.Write(buf)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if n != sample_size {
			os.Exit(1)
		}
	}
}
