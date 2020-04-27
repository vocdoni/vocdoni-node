package subpub

import (
	"bufio"

	"gitlab.com/vocdoni/go-dvote/log"
)

// SendMessage encrypts and writes a message on the readwriter buffer
func (ps *SubPub) SendMessage(w *bufio.Writer, msg []byte) error {
	log.Debugf("sending message: %s", msg)
	if !ps.Private {
		msg = []byte(ps.encrypt(msg)) // TO-DO find a better way to encapsulate data! byte -> b64 -> byte is not the best...
	}
	if _, err := w.Write(append(msg, byte(delimiter))); err != nil {
		return err
	}
	return w.Flush()
}
