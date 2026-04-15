package server

import (
	gosasl "github.com/emersion/go-sasl"
	"github.com/xdg-go/scram"

	mssasl "github.com/foxcpp/go-sieve/managesieve/sasl"
)

func PlainAuth(fn func(identity, username, password string) (string, error)) SASLServerFactory {
	return func(conn *Conn) (gosasl.Server, error) {
		return gosasl.NewPlainServer(func(identity, username, password string) error {
			owner, err := fn(identity, username, password)
			if err != nil {
				return err
			}
			return conn.Authorize(owner)
		}), nil
	}
}

func SCRAMSHA1Auth(lookup scram.CredentialLookup) SASLServerFactory {
	return func(conn *Conn) (gosasl.Server, error) {
		srv, err := mssasl.NewSCRAMSHA1Server(lookup)
		if err != nil {
			return nil, err
		}
		return &authorizingSCRAMServer{srv: srv, conn: conn}, nil
	}
}

type authorizingServer struct {
	gosasl.Server
	conn *Conn
}

func (s *authorizingServer) Next(resp []byte) ([]byte, bool, error) {
	challenge, done, err := s.Server.Next(resp)
	if err != nil || !done {
		return challenge, done, err
	}
	return challenge, done, s.conn.Authorize("")
}

type authorizingSCRAMServer struct {
	srv  *mssasl.SCRAMSHA1Server
	conn *Conn
}

func (s *authorizingSCRAMServer) Next(resp []byte) ([]byte, bool, error) {
	challenge, done, err := s.srv.Next(resp)
	if err != nil || !done {
		return challenge, done, err
	}
	return challenge, done, s.conn.Authorize(s.srv.Username())
}
