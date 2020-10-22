package skkserv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/kechako/goskkserv/dict"
	"github.com/kechako/goskkserv/log"
)

type Server struct {
	Dictionary *dict.Dictionary
	Encoding   Encoding
	Logger     log.Logger

	listener   net.Listener
	activeConn map[*net.Conn]struct{}
	wg         sync.WaitGroup
	exit       func()
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
		s.setActiveConn(conn, false)
	}

	return lerr
}

func (s *Server) Listen(addr string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.exit = cancel

	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to resolve address [%s]: %w", addr, err)
	}

	s.logger().Infof("listen on [%s]...", tcpAddr)
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
		s.setActiveConn(&c, true)
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
	defer s.setActiveConn(&conn, false)
	defer conn.Close()

	s.logger().Infof("new client : %s", conn.RemoteAddr())

	encoding := s.Encoding.encoding()
	w := encoding.NewEncoder().Writer(conn)
	r := encoding.NewDecoder().Reader(conn)

	dictionary := s.dict()

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
			if errors.Is(err, io.EOF) {
				// client closed
				break loop
			}
			s.logger().Error("failed to read request data: ", err)
			return
		}
		cmd := string(buf[:n])
		switch cmd[0] {
		case ClientEnd:
			s.logger().Infof("client end : %s", conn.RemoteAddr())
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
			s.logger().Debugf("REQUEST: key : %s", key)

			candidates := dictionary.Search(key)
			if len(candidates) > 0 {
				ret.WriteRune(ServerFound)
				for _, c := range candidates {
					ret.WriteRune('/')
					ret.WriteString(c.String())
				}
				ret.WriteString("/\n")
				s.logger().Debugf("REQUEST: candidate: %s", strings.TrimSpace(ret.String()))
			} else {
				ret.WriteRune(ServerNotFound)
				ret.WriteString(cmd[1:])
				s.logger().Debug("REQUEST: not found")
			}
		case ClientVersion:
			s.logger().Debug("VERSION")
			ret.WriteString("goskkserv-1.0")
		case ClientHost:
			s.logger().Debug("HOST")
			ret.WriteString(conn.LocalAddr().String())
		case ClientCompletion:
			s.logger().Debug("COMPLETION")
			ret.WriteRune(ServerFound)
			ret.WriteString("//\n")
		default:
			s.logger().Infof("UNKNOWN: message from client %s: %c/\"%s\"", conn.RemoteAddr(), cmd[0], cmd)
			continue
		}
		if _, err := w.Write(ret.Bytes()); err != nil {
			s.logger().Error(err)
			return
		}
	}
}

func (s *Server) setActiveConn(conn *net.Conn, set bool) {
	if s.activeConn == nil {
		s.activeConn = make(map[*net.Conn]struct{})
	}

	if set {
		s.activeConn[conn] = struct{}{}
	} else {
		delete(s.activeConn, conn)
	}
}

func (s *Server) dict() *dict.Dictionary {
	if s.Dictionary != nil {
		return s.Dictionary
	}

	return &dict.Dictionary{}
}

var nopLogger = log.NewNop()

func (s *Server) logger() log.Logger {
	if s.Logger != nil {
		return s.Logger
	}

	return nopLogger
}
