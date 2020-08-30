package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
)

func main() {
	if err := _main(); err != nil {
		fmt.Fprintf(os.Stderr, "error : %v\n", err)
	}
}

func _main() error {
	var addr string
	var debug bool
	flag.StringVar(&addr, "addr", "127.0.0.1:1178", "Address to listen")
	flag.BoolVar(&debug, "debug", false, "Enable debug mode")
	flag.Parse()

	var dict *Dictionary
	if flag.NArg() > 0 {
		var err error
		dict, err = OpenDictionary(flag.Args()...)
		if err != nil {
			log.Print("error : ", err)
		}
	}

	s := NewServer(dict)
	s.Debug = debug

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	defer close(ch)

	go func() {
		<-ch
		s.Shutdown()
	}()

	if err := s.Listen(addr); err != nil {
		return err
	}

	return nil
}
