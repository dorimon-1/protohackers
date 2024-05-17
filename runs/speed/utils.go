package main

import (
	"log"
)

func (s *Session) SendMessage(msgType MessageType, msg []byte) error {
	if msg == nil {
		msg = make([]byte, 1)
	} else {
		msg = append(msg[:1], msg[0:]...)
	}

	msg[0] = byte(msgType)
	_, err := s.Conn.Write(msg)
	return err

}

func (s *Session) ReadUint8() (uint8, error) {
	byteMsg, err := s.Reader.ReadByte()
	if err != nil {
		return 0x00, err
	}
	return uint8(byteMsg), nil
}

func (s *Session) ReadUint16() (uint16, error) {
	var unsignedInt uint16

	for i := 0; i < 2; i++ {
		b, err := s.Reader.ReadByte()
		if err != nil {
			return 0, err
		}

		unsignedInt = (unsignedInt << 8) | uint16(b)
	}

	return unsignedInt, nil
}

func (s *Session) ReadUint32() (uint32, error) {

	var unsignedInt uint32

	for i := 0; i < 4; i++ {
		b, err := s.Reader.ReadByte()
		if err != nil {
			return 0, err
		}

		unsignedInt = (unsignedInt << 8) | uint32(b)
	}

	return unsignedInt, nil
}

// TODO: Reading a string doesn't work
func (s *Session) ReadString() (string, error) {
	strLen, err := s.Reader.ReadByte()
	if err != nil {
		return "", err
	}

	buf := make([]byte, strLen)

	for i := 0; i < int(strLen); i++ {
		b, err := s.Reader.ReadByte()
		if err != nil {
			return "", err
		}

		buf[i] = b
	}
	s.logf("Got a string: %s", string(buf))
	return string(buf), nil
}

func CreateString(msg string) []byte {
	buf := make([]byte, 1)
	buf[0] = byte(len(msg))

	for _, char := range msg {
		buf = append(buf, byte(char))
	}
	return buf
}

func (s *Session) logf(format string, v ...any) {
	log.SetPrefix(s.Conn.RemoteAddr().String() + string(" "))
	log.Printf(format, v...)
}

func (s *Session) logln(v ...any) {
	log.SetPrefix(s.Conn.RemoteAddr().String() + string(" "))
	log.Println(v...)
}
