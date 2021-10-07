package subpub

import (
	"bufio"

	"git.sr.ht/~sircmpwn/go-bare"
)

// SendMessage encrypts and writes a message on the readwriter buffer
func (ps *SubPub) SendMessage(w *bufio.Writer, msg []byte) error {
	if !ps.Private {
		msg = ps.encrypt(msg)
	}
	message := &Message{Data: msg}
	data, err := bare.Marshal(message)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return w.Flush()
}
