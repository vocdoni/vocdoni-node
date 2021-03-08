package scrutinizer

import (
	"time"

	"github.com/timshannon/badgerhold/v3"
	"go.vocdoni.io/proto/build/go/models"
)

type Process struct {
	ID            []byte `badgerhold:"key"`
	EntityID      []byte `badgerhold:"index"`
	StartBlock    uint32
	EndBlock      uint32 `badgerhold:"index"`
	ResultsHeight uint32 `badgerhold:"index"`
	CensusRoot    []byte
	CensusURI     string
	CensusOrigin  int32
	Status        int32  `badgerhold:"index"`
	Namespace     uint32 `badgerhold:"index"`
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

type Entity struct {
	ID           []byte `badgerhold:"key"`
	CreationTime time.Time
}

type Results struct {
	ProcessID  []byte `badgerhold:"key"`
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
