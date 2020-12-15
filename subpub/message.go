package subpub

import (
	"bufio"
)

// SendMessage encrypts and writes a message on the readwriter buffer
func (ps *SubPub) SendMessage(w *bufio.Writer, msg []byte) error {
	if !ps.Private {
		msg = []byte(ps.encrypt(msg))
	}
	if _, err := w.Write(append(msg, byte(delimiter))); err != nil {
		return err
	}
	return w.Flush()
}
