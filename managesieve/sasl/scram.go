package sasl

import (
	"fmt"

	gosasl "github.com/emersion/go-sasl"
	"github.com/xdg-go/scram"
)

const MechSCRAMSHA1 = "SCRAM-SHA-1"

func NewSCRAMSHA1Client(username, password, authzID string) (gosasl.Client, error) {
	c, err := scram.SHA1.NewClient(username, password, authzID)
	if err != nil {
		return nil, err
	}
	return &scramClient{mech: MechSCRAMSHA1, conv: c.NewConversation()}, nil
}

func NewSCRAMSHA1Server(lookup scram.CredentialLookup) (*SCRAMSHA1Server, error) {
	srv, err := scram.SHA1.NewServer(lookup)
	if err != nil {
		return nil, err
	}
	return &SCRAMSHA1Server{conv: srv.NewConversation()}, nil
}

func CredentialsSCRAMSHA1(username, password string, factors scram.KeyFactors) (scram.StoredCredentials, error) {
	c, err := scram.SHA1.NewClient(username, password, "")
	if err != nil {
		return scram.StoredCredentials{}, err
	}
	creds, err := c.GetStoredCredentialsWithError(factors)
	if err != nil {
		return scram.StoredCredentials{}, err
	}
	return creds, nil
}

type scramClient struct {
	mech    string
	conv    *scram.ClientConversation
	started bool
}

func (c *scramClient) Start() (mech string, ir []byte, err error) {
	out, err := c.conv.Step("")
	if err != nil {
		return "", nil, err
	}
	c.started = true
	return c.mech, []byte(out), nil
}

func (c *scramClient) Next(challenge []byte) (response []byte, err error) {
	if !c.started {
		return nil, fmt.Errorf("managesieve/sasl: SCRAM client not started")
	}
	out, err := c.conv.Step(string(challenge))
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}

type SCRAMSHA1Server struct {
	conv *scram.ServerConversation
}

func (s *SCRAMSHA1Server) Next(response []byte) (challenge []byte, done bool, err error) {
	out, err := s.conv.Step(string(response))
	if err != nil {
		return nil, false, err
	}
	if s.conv.Done() && !s.conv.Valid() {
		return nil, true, fmt.Errorf("managesieve/sasl: SCRAM validation failed")
	}
	return []byte(out), s.conv.Done(), nil
}

func (s *SCRAMSHA1Server) Username() string {
	return s.conv.Username()
}
