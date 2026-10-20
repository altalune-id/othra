package client

import "time"

func (s *Service) SetNow(now func() time.Time) { s.now = now }

func (s *Service) SetNewSecret(fn func() (plaintext, hash, hint string, err error)) { s.newSecret = fn }
