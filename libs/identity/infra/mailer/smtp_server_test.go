package mailer_test

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"testing"
)

type smtpAuthAttempt struct {
	Mechanism string
	Username  string
	Password  string
}

type capturedSMTPMessage struct {
	MailFrom string
	RcptTo   []string
	RawData  []byte
}

type fakeSMTPServer struct {
	ln   net.Listener
	host string
	port int

	mu               sync.Mutex
	connections      int
	authAttempts     []smtpAuthAttempt
	messages         []capturedSMTPMessage
	activeConns      map[net.Conn]struct{}
	advertiseAuth    bool
	rejectAuthMsg    string
	rejectRcptMsg    string
	rejectDataCmdMsg string
	rejectDataEndMsg string
	stallOnConnect   <-chan struct{}
	stallOnData      <-chan struct{}

	wg sync.WaitGroup
}

func startFakeSMTPServer(t *testing.T) *fakeSMTPServer {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake SMTP server: %v", err)
	}
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		_ = ln.Close()
		t.Fatalf("unexpected listener addr type %T", ln.Addr())
	}

	s := &fakeSMTPServer{
		ln:            ln,
		host:          tcpAddr.IP.String(),
		port:          tcpAddr.Port,
		activeConns:   make(map[net.Conn]struct{}),
		advertiseAuth: true,
	}

	s.wg.Add(1)
	go s.acceptLoop()

	t.Cleanup(func() {
		s.close()
	})
	return s
}

func (s *fakeSMTPServer) close() {
	_ = s.ln.Close()
	s.mu.Lock()
	for conn := range s.activeConns {
		_ = conn.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *fakeSMTPServer) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.connections++
		s.activeConns[conn] = struct{}{}
		s.mu.Unlock()

		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			defer func() {
				s.mu.Lock()
				delete(s.activeConns, c)
				s.mu.Unlock()
				_ = c.Close()
			}()
			s.handleConn(c)
		}(conn)
	}
}

func (s *fakeSMTPServer) handleConn(conn net.Conn) {
	s.mu.Lock()
	stallConnect := s.stallOnConnect
	s.mu.Unlock()
	if stallConnect != nil {
		<-stallConnect
		return
	}

	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	tp := textproto.NewReader(rw.Reader)

	if err := writeLine(rw, "220 iris-gate.test ESMTP"); err != nil {
		return
	}

	var current capturedSMTPMessage
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO "):
			s.mu.Lock()
			advAuth := s.advertiseAuth
			s.mu.Unlock()
			if advAuth {
				if err := writeLine(rw, "250-iris-gate.test"); err != nil {
					return
				}
				if err := writeLine(rw, "250 AUTH PLAIN"); err != nil {
					return
				}
			} else {
				if err := writeLine(rw, "250 iris-gate.test"); err != nil {
					return
				}
			}
		case strings.HasPrefix(upper, "HELO "):
			if err := writeLine(rw, "250 iris-gate.test"); err != nil {
				return
			}
		case strings.HasPrefix(upper, "AUTH "):
			if !s.handleAuth(rw, line) {
				return
			}
		case strings.HasPrefix(upper, "MAIL FROM:"):
			current = capturedSMTPMessage{
				MailFrom: extractBracketed(line[len("MAIL FROM:"):]),
			}
			if err := writeLine(rw, "250 OK"); err != nil {
				return
			}
		case strings.HasPrefix(upper, "RCPT TO:"):
			s.mu.Lock()
			rejectMsg := s.rejectRcptMsg
			s.mu.Unlock()
			if rejectMsg != "" {
				if err := writeLine(rw, "550 "+rejectMsg); err != nil {
					return
				}
				continue
			}
			current.RcptTo = append(current.RcptTo, extractBracketed(line[len("RCPT TO:"):]))
			if err := writeLine(rw, "250 OK"); err != nil {
				return
			}
		case upper == "DATA":
			s.mu.Lock()
			rejectCmd := s.rejectDataCmdMsg
			rejectEnd := s.rejectDataEndMsg
			stallData := s.stallOnData
			s.mu.Unlock()
			if stallData != nil {
				<-stallData
				return
			}
			if rejectCmd != "" {
				if err := writeLine(rw, "554 "+rejectCmd); err != nil {
					return
				}
				continue
			}
			if err := writeLine(rw, "354 End data with <CR><LF>.<CR><LF>"); err != nil {
				return
			}
			body, err := io.ReadAll(tp.DotReader())
			if err != nil {
				return
			}
			if rejectEnd != "" {
				if err := writeLine(rw, "554 "+rejectEnd); err != nil {
					return
				}
				continue
			}
			current.RawData = body
			s.mu.Lock()
			s.messages = append(s.messages, current)
			s.mu.Unlock()
			if err := writeLine(rw, "250 OK"); err != nil {
				return
			}
		case upper == "RSET":
			current = capturedSMTPMessage{}
			if err := writeLine(rw, "250 OK"); err != nil {
				return
			}
		case upper == "NOOP":
			if err := writeLine(rw, "250 OK"); err != nil {
				return
			}
		case upper == "QUIT":
			_ = writeLine(rw, "221 Bye")
			return
		default:
			if err := writeLine(rw, "500 unrecognized command"); err != nil {
				return
			}
		}
	}
}

func (s *fakeSMTPServer) handleAuth(rw *bufio.ReadWriter, line string) bool {
	parts := strings.Fields(line)
	attempt := smtpAuthAttempt{}
	if len(parts) >= 2 {
		attempt.Mechanism = strings.ToUpper(parts[1])
	}
	if len(parts) >= 3 && attempt.Mechanism == "PLAIN" {
		if decoded, err := base64.StdEncoding.DecodeString(parts[2]); err == nil {
			fields := strings.Split(string(decoded), "\x00")
			if len(fields) == 3 {
				attempt.Username = fields[1]
				attempt.Password = fields[2]
			}
		}
	}

	s.mu.Lock()
	s.authAttempts = append(s.authAttempts, attempt)
	rejectMsg := s.rejectAuthMsg
	s.mu.Unlock()

	if rejectMsg != "" {
		return writeLine(rw, "535 "+rejectMsg) == nil
	}
	return writeLine(rw, "235 Authentication succeeded") == nil
}

func (s *fakeSMTPServer) snapshot() (int, []smtpAuthAttempt, []capturedSMTPMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	authCopy := make([]smtpAuthAttempt, len(s.authAttempts))
	copy(authCopy, s.authAttempts)
	msgCopy := make([]capturedSMTPMessage, len(s.messages))
	copy(msgCopy, s.messages)
	return s.connections, authCopy, msgCopy
}

func (s *fakeSMTPServer) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connections = 0
	s.authAttempts = nil
	s.messages = nil
	s.rejectAuthMsg = ""
	s.rejectRcptMsg = ""
	s.rejectDataCmdMsg = ""
	s.rejectDataEndMsg = ""
	s.stallOnConnect = nil
	s.stallOnData = nil
}

func writeLine(rw *bufio.ReadWriter, line string) error {
	if _, err := fmt.Fprintf(rw, "%s\r\n", line); err != nil {
		return err
	}
	return rw.Flush()
}

func extractBracketed(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if start := strings.IndexByte(trimmed, '<'); start >= 0 {
		if end := strings.IndexByte(trimmed[start+1:], '>'); end >= 0 {
			return trimmed[start+1 : start+1+end]
		}
	}
	return trimmed
}
