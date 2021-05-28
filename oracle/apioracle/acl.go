package apioracle

import (
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

var ErrMaxProposalsReached = fmt.Errorf("max proposals per address reached")

type proposalACL struct {
	store        sync.Map
	maxProposals uint
	purgePeriod  time.Duration
}

type proposalCounter struct {
	creationTime  time.Time
	creationCount uint
}

func NewProposalACL(purgePeriod time.Duration, maxProposals uint) *proposalACL {
	pACL := &proposalACL{
		purgePeriod:  purgePeriod,
		maxProposals: maxProposals,
	}
	go func() {
		for {
			time.Sleep(aclPurgePeriod)
			n := pACL.purge()
			if n > 0 {
				log.Infof("purged %d ACL entries", n)
			}
			log.Infof("proposal ACL size: %d", pACL.count())
		}
	}()
	return pACL
}

func (p *proposalACL) id(holder, contract []byte) string {
	return fmt.Sprintf("%s%s", holder, contract)
}

func (p *proposalACL) purge() int {
	rmList := []string{}
	p.store.Range(func(key, value interface{}) bool {
		pc := value.(*proposalCounter)
		if time.Since(pc.creationTime) > p.purgePeriod {
			rmList = append(rmList, key.(string))
		}
		return true
	})
	for _, id := range rmList {
		p.store.Delete(id)
	}
	return len(rmList)
}

func (p *proposalACL) count() int {
	count := 0
	p.store.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

func (p *proposalACL) add(holder, contract []byte) error {
	if holder == nil || len(contract) != types.EntityIDsize ||
		len(holder) < common.AddressLength {
		return fmt.Errorf("holder or contract address, wrong format")
	}
	id := p.id(holder, contract)
	val, ok := p.store.Load(id)
	if !ok {
		p.store.Store(id, &proposalCounter{
			creationTime:  time.Now(),
			creationCount: 1,
		})
		return nil
	}
	pc := val.(*proposalCounter)
	if pc.creationCount >= p.maxProposals {
		return ErrMaxProposalsReached
	}
	pc.creationCount++
	p.store.Store(id, pc)
	return nil
}
