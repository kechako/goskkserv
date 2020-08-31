package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/unicode"
)

type ServerEncoding string

const (
	UTF8     ServerEncoding = "utf-8"
	EUCJP    ServerEncoding = "euc-jp"
	ShiftJIS ServerEncoding = "sjis"
)

func ParseServerEncoding(e string) (ServerEncoding, error) {
	se := ServerEncoding(e)
	switch se {
	case UTF8, EUCJP, ShiftJIS:
		return se, nil
	}

	return "", errors.New("invalid encoding")
}

type Server struct {
	Dict  *Dictionary
	Debug bool

	enc encoding.Encoding

	listener   net.Listener
	activeConn map[*net.Conn]struct{}
	wg         sync.WaitGroup
	exit       func()

	loge *log.Logger
	logi *log.Logger
	logd *log.Logger
}

func NewServer(dict *Dictionary) *Server {
	if dict == nil {
		dict = EmptyDictionary()
	}

	return &Server{
		Dict:       dict,
		enc:        unicode.UTF8,
		activeConn: make(map[*net.Conn]struct{}),
		loge:       log.New(os.Stderr, "[ERROR] ", log.Ldate|log.Lmicroseconds|log.Lmsgprefix),
		logi:       log.New(os.Stdout, "[INFO ] ", log.Ldate|log.Lmicroseconds|log.Lmsgprefix),
		logd:       log.New(os.Stdout, "[DEBUG] ", log.Ldate|log.Lmicroseconds|log.Lmsgprefix),
	}
}

func (s *Server) Shutdown() error {
	if s.listener == nil {
		return nil
	}
	if s.exit != nil {
		s.exit()
	}

	lerr := s.listener.Close()

	for conn := range s.activeConn {
		(*conn).Close()
		delete(s.activeConn, conn)
	}

	return lerr
}

func (s *Server) SetEncoding(e ServerEncoding) {
	switch e {
	case UTF8:
		s.enc = unicode.UTF8
	case EUCJP:
		s.enc = japanese.EUCJP
	case ShiftJIS:
		s.enc = japanese.ShiftJIS
	default:
		panic("invalid encoding")
	}
}

func (s *Server) Listen(addr string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.exit = cancel

	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to resolve address [%s]: %w", addr, err)
	}

	s.infof("listen on [%s]...", tcpAddr)
	l, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return fmt.Errorf("failed to listen TCP [%v]: %w", tcpAddr, err)
	}
	defer l.Close()
	s.listener = l

	var tempDelay time.Duration
loop:
	for {
		c, err := l.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				break loop
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				time.Sleep(tempDelay)
			}
			return err
		}
		tempDelay = 0
		s.activeConn[&c] = struct{}{}
		s.wg.Add(1)
		go s.serve(ctx, c)
	}

	s.wg.Wait()

	return nil
}

const (
	ClientEnd        = '0'
	ClientRequest    = '1'
	ClientVersion    = '2'
	ClientHost       = '3'
	ClientCompletion = '4'

	ServerError    = '0'
	ServerFound    = '1'
	ServerNotFound = '4'
	ServerFull     = '9'
)

func (s *Server) serve(ctx context.Context, conn net.Conn) {
	defer s.wg.Done()
	defer delete(s.activeConn, &conn)
	defer conn.Close()

	s.infof("new client : %s", conn.RemoteAddr())

	w := s.enc.NewEncoder().Writer(conn)
	r := s.enc.NewDecoder().Reader(conn)

	var buf [1024]byte
	var ret bytes.Buffer
	ret.Grow(4096)
loop:
	for {
		ret.Reset()

		n, err := r.Read(buf[:])
		if err != nil {
			select {
			case <-ctx.Done():
				break loop
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				continue
			}
			s.error(err)
			return
		}
		cmd := string(buf[:n])
		switch cmd[0] {
		case ClientEnd:
			s.infof("client end : %s", conn.RemoteAddr())
			break loop
		case ClientRequest:
			i := strings.IndexByte(cmd, ' ')
			if i < 0 {
				i = strings.IndexByte(cmd, '\n')
			}
			if i < 0 {
				i = len(cmd)
			}

			key := cmd[1:i]
			s.debugf("REQUEST: key : %s", key)

			candidates := s.Dict.Search(key)
			if len(candidates) > 0 {
				ret.WriteRune(ServerFound)
				for _, c := range candidates {
					ret.WriteRune('/')
					ret.WriteString(c.String())
				}
				ret.WriteString("/\n")
				s.debugf("REQUEST: candidate: %s", strings.TrimSpace(ret.String()))
			} else {
				ret.WriteRune(ServerNotFound)
				ret.WriteString(cmd[1:])
				s.debug("REQUEST: not found")
			}
		case ClientVersion:
			s.debug("VERSION")
			ret.WriteString("goskkserv-1.0")
		case ClientHost:
			s.debug("HOST")
			ret.WriteString(conn.LocalAddr().String())
		case ClientCompletion:
			s.debug("COMPLETION")
			ret.WriteRune(ServerFound)
			ret.WriteString("//\n")
		default:
			s.infof("UNKNOWN: message from client %s: %c/\"%s\"", conn.RemoteAddr(), cmd[0], cmd)
			continue
		}
		if _, err := w.Write(ret.Bytes()); err != nil {
			s.error(err)
			return
		}
	}
}

func (s *Server) error(v ...interface{}) {
	s.loge.Print(v...)
}

func (s *Server) errorf(format string, v ...interface{}) {
	s.loge.Printf(format, v...)
}

func (s *Server) info(v ...interface{}) {
	s.logi.Print(v...)
}

func (s *Server) infof(format string, v ...interface{}) {
	s.logi.Printf(format, v...)
}

func (s *Server) debug(v ...interface{}) {
	if s.Debug {
		s.logd.Print(v...)
	}
}

func (s *Server) debugf(format string, v ...interface{}) {
	if s.Debug {
		s.logd.Printf(format, v...)
	}
}
