package scrutinizer

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/timshannon/badgerhold/v3"
	"go.vocdoni.io/proto/build/go/models"
)

type Process struct {
	ID            []byte `badgerholdKey:"ID"`
	EntityID      []byte `badgerholdIndex:"EntityID"`
	StartBlock    uint32
	EndBlock      uint32 `badgerholdIndex:"EndBlock"`
	Rheight       uint32 `badgerholdIndex:"Rheight"`
	CensusRoot    []byte
	CensusURI     string
	CensusOrigin  int32
	Status        int32  `badgerholdIndex:"Status"`
	Namespace     uint32 `badgerholdIndex:"Namespace"`
	Envelope      *models.EnvelopeType
	Mode          *models.ProcessMode
	VoteOpts      *models.ProcessVoteOptions
	PrivateKeys   []string
	PublicKeys    []string
	QuestionIndex uint32
	CreationTime  time.Time
	HaveResults   bool
	FinalResults  bool
}

func (p Process) String() string {
	v := reflect.ValueOf(p)
	t := v.Type()
	var b strings.Builder
	b.WriteString("{")
	for i := 0; i < t.NumField(); i++ {
		fv := v.Field(i)
		if fv.IsZero() {
			// omit zero values
			continue
		}
		if b.Len() > 1 {
			b.WriteString(" ")
		}
		ft := t.Field(i)
		b.WriteString(ft.Name)
		b.WriteString(":")
		if ft.Type.Kind() == reflect.Slice && ft.Type.Elem().Kind() == reflect.Uint8 {
			// print []byte as hexadecimal
			fmt.Fprintf(&b, "%x", fv.Bytes())
		} else {
			fv = reflect.Indirect(fv) // print *T as T
			fmt.Fprintf(&b, "%v", fv.Interface())
		}
	}
	b.WriteString("}")
	return b.String()
}

type Entity struct {
	ID           []byte `badgerholdKey:"ID"`
	CreationTime time.Time
}

type Results struct {
	ProcessID  []byte `badgerholdKey:"ProcessID"`
	Votes      []*models.QuestionResult
	Signatures [][]byte
}

func InitDB(dataDir string) (*badgerhold.Store, error) {
	options := badgerhold.DefaultOptions
	options.Dir = dataDir
	options.ValueDir = dataDir
	// TO-DO set custom logger
	return badgerhold.Open(options)
}
