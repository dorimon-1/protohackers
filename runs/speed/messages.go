package main

func (s *Session) SendError(msg string) error {
	defer s.Cancel()

	buf := CreateString(msg)
	if err := s.SendMessage(ERROR, buf); err != nil {
		return err
	}

	return s.Ctx.Err()
}
