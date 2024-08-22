package electionprice

import (
	"fmt"
	"math"
	"os"
	"testing"
	"text/tabwriter"
)

func TestPrice(t *testing.T) {
	calculator := NewElectionPriceCalculator(DefaultElectionPriceFactors)
	calculator.SetBasePrice(100)
	calculator.SetCapacity(1000)

	testParams := ElectionParameters{
		MaxCensusSize:    500,
		ElectionDuration: 1000,
		EncryptedVotes:   true,
		AnonymousVotes:   true,
		MaxVoteOverwrite: 1,
	}

	// Compute the price with the test parameters
	price := calculator.Price(&testParams)

	// Check for underflows or overflows
	if price == 0 || price == math.MaxUint64 {
		t.Error("Underflow or overflow detected in price calculation.")
	}

	// Modify the parameters to double the duration
	testParams.ElectionDuration *= 2
	modifiedPrice := calculator.Price(&testParams)
	if modifiedPrice <= price {
		t.Error("Price did not increase after doubling the election duration.")
	}

	// Modify the parameters to halve the maxCensusSize
	testParams.MaxCensusSize /= 2
	modifiedPrice2 := calculator.Price(&testParams)
	if modifiedPrice2 >= modifiedPrice {
		t.Error("Price did not decrease after halving the maxCensusSize.")
	}

	// Modify the parameters to disable encryption
	testParams.EncryptedVotes = false
	modifiedPrice3 := calculator.Price(&testParams)
	if modifiedPrice3 >= modifiedPrice2 {
		t.Error("Price did not decrease after disabling EncryptedVotes.")
	}

	// Modify the parameters to disable anonymous votes
	testParams.AnonymousVotes = false
	modifiedPrice4 := calculator.Price(&testParams)
	if modifiedPrice4 >= modifiedPrice3 {
		t.Error("Price did not decrease after disabling AnonymousVotes.")
	}

	// Modify the parameters to increase the maxVoteOverwrite
	testParams.MaxVoteOverwrite *= 2
	modifiedPrice5 := calculator.Price(&testParams)
	if modifiedPrice5 <= modifiedPrice4 {
		t.Error("Price did not increase after increasing the maxVoteOverwrite.")
	}
}

func TestCalculator_Disabled(t *testing.T) {
	c := NewElectionPriceCalculator(DefaultElectionPriceFactors)
	c.SetBasePrice(0)

	params := &ElectionParameters{
		MaxCensusSize:    500,
		ElectionDuration: 600,
		EncryptedVotes:   false,
		AnonymousVotes:   false,
		MaxVoteOverwrite: 2,
	}

	price := c.Price(params)
	if price != 0 {
		t.Errorf("expected 0, got %d", price)
	}
}

func TestCalculatorPriceWithZeroValues(t *testing.T) {
	calculator := NewElectionPriceCalculator(DefaultElectionPriceFactors)
	calculator.SetBasePrice(100)
	calculator.SetCapacity(1000)

	params := &ElectionParameters{
		MaxCensusSize:    0,
		ElectionDuration: 0,
		EncryptedVotes:   true,
		AnonymousVotes:   true,
		MaxVoteOverwrite: 5,
	}

	price := calculator.Price(params)

	if price < 100 {
		t.Errorf("Price should not be less than base price, got %d", price)
	}

	// The prices shouldn't change if MaxCensusSize and ElectionDuration are zero
	params2 := &ElectionParameters{
		MaxCensusSize:    10,
		ElectionDuration: 0,
		EncryptedVotes:   true,
		AnonymousVotes:   true,
		MaxVoteOverwrite: 5,
	}
	price2 := calculator.Price(params2)

	if price != price2 {
		t.Errorf("Prices should not differ when MaxCensusSize changes from 0 to non-zero, got %d and %d", price, price2)
	}

	params3 := &ElectionParameters{
		MaxCensusSize:    0,
		ElectionDuration: 10,
		EncryptedVotes:   true,
		AnonymousVotes:   true,
		MaxVoteOverwrite: 5,
	}
	price3 := calculator.Price(params3)

	if price != price3 {
		t.Errorf("Prices should not differ when ElectionDuration changes from 0 to non-zero, got %d and %d", price, price3)
	}
}

func TestCalculatorPriceTable(_ *testing.T) {
	c := NewElectionPriceCalculator(DefaultElectionPriceFactors)
	c.SetCapacity(1000)
	c.SetBasePrice(10)
	hour := uint32(340)

	maxCensusSizes := []uint64{
		100, 200, 500, 1000, 1500, 2000, 2500, 3000, 5000, 10000, 20000,
		50000, 100000, 200000, 500000, 800000, 1000000,
	}
	electionDurations := []uint32{
		1 * hour, 12 * hour, 24 * hour, 24 * hour * 2, 24 * hour * 5,
		24 * hour * 7, 24 * hour * 15,
	}
	encryptedVotes := true
	anonymousVotes := true
	maxVoteOverwrite := uint32(0)

	w := new(tabwriter.Writer)
	// Format in tab-separated columns with a tab stop of 8.
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)
	fmt.Fprintln(w, "CensusSize\t\tDuration\t\tPrice\t\tPricePerVoter")

	for _, maxCensusSize := range maxCensusSizes {
		for _, electionDuration := range electionDurations {
			params := &ElectionParameters{
				MaxCensusSize:    maxCensusSize,
				ElectionDuration: electionDuration,
				EncryptedVotes:   encryptedVotes,
				AnonymousVotes:   anonymousVotes,
				MaxVoteOverwrite: maxVoteOverwrite,
			}
			price := c.Price(params)
			pricePerVoter := float64(price) / float64(maxCensusSize)
			fmt.Fprintf(w, "%d\t\t%d\t\t%d\t\t%.2f\n", maxCensusSize, electionDuration, price, pricePerVoter)
		}
	}

	fmt.Fprintln(w)
	w.Flush()
}
