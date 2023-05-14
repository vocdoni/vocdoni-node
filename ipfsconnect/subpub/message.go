package subpub

import (
	"bufio"
	"fmt"

	"git.sr.ht/~sircmpwn/go-bare"
)

// SendMessage encrypts and writes a message on the readwriter buffer.
func (ps *SubPub) SendMessage(w *bufio.Writer, msg []byte) error {
	msg = ps.encrypt(msg)
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

// ReadMessage reads a message from the readwriter buffer.
func (ps *SubPub) ReadMessage(r *bufio.Reader) (*Message, error) {
	message := new(Message)
	if err := bare.UnmarshalReader(r, message); err != nil {
		return nil, fmt.Errorf("error unmarshaling: %w", err)
	}
	if len(message.Data) == 0 {
		return nil, fmt.Errorf("no data could be read")
	}
	var ok bool
	message.Data, ok = ps.decrypt(message.Data)
	if !ok {
		return nil, fmt.Errorf("cannot decrypt message")
	}
	return message, nil
}
