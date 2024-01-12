// Package electionprice provides a mechanism for calculating the price of an election based on its characteristics.
//
// The formula used to calculate the price for creating an election on the Vocdoni blockchain is designed to take into
// account various factors that impact the cost and complexity of conducting an election. The price is determined by
// combining several components, each reflecting a specific aspect of the election process.
//
// 1. Base Price: This is a fixed cost that serves as a starting point for the price calculation. It represents the
// minimal price for creating an election, regardless of its size or duration.
//
// 2. Size Price: As the number of voters (maxCensusSize) in an election increases, the resources required to manage
// the election also grow. To account for this, the size price component is directly proportional to the maximum number
// of votes allowed in the election. Additionally, it takes into consideration the blockchain's maximum capacity
// (capacity) and the maximum capacity the blockchain administrators can set (maxCapacity). This ensures that the price
// is adjusted based on the current capacity of the blockchain.
//
// 3. Duration Price: The length of the election (electionDuration) also affects the price, as longer elections occupy
// more resources over time. The duration price component is directly proportional to the election duration and
// inversely proportional to the maximum number of votes. This means that if the election lasts longer, the price
// increases, and if there are more votes in a shorter time, the price also increases to reflect the higher demand for
// resources.
//
// 4. Encrypted Votes: If an election requires encryption for maintaining secrecy until the end (encryptedVotes), it
// demands additional resources and computational effort. Therefore, the encrypted price component is added to the total
// price when this feature is enabled.
//
// 5. Anonymous Votes: Similarly, if an election must be anonymous (anonymousVotes), it requires additional measures to
// ensure voter privacy. As a result, the anonymous price component is added to the total price when this option is
// chosen.
//
// 6. Overwrite Price: Allowing voters to overwrite their votes (maxVoteOverwrite) can increase the complexity of
// managing the election, as it requires additional resources to handle vote updates. The overwrite price component
// accounts for this by being proportional to the maximum number of vote overwrites and the maximum number of votes
// allowed in the election. It also takes into account the blockchain's capacity to ensure the price reflects the
// current resource constraints.
//
// The constant factors in the price formula play a crucial role in determining the price of an election based on its
// characteristics. Each factor is associated with a specific component of the price formula and helps to weigh the
// importance of that component in the final price calculation. The rationale behind these constant factors is to
// provide a flexible mechanism to adjust the pricing model based on the system's needs and requirements.
//
// k1 (Size price factor): This constant factor affects the size price component of the formula. By adjusting k1,
// you can control the impact of the maximum number of votes (maxCensusSize) on the overall price. A higher k1 value
// would make the price increase more rapidly as the election size grows, while a lower k1 value would make the price
// less sensitive to the election size. The rationale behind k1 is to ensure that the pricing model can be adapted to
// accommodate different election sizes while considering the resource requirements.
//
// k2 (Duration price factor): This constant factor influences the duration price component of the formula. By
// adjusting k2, you can control how the duration of the election (electionDuration) affects the price. A higher k2
// value would make the price increase more quickly as the election duration extends, while a lower k2 value would make
// the price less sensitive to the election duration. The rationale behind k2 is to reflect the resource consumption
// over time and ensure that longer elections are priced accordingly.
//
// k3 (Encrypted price factor): This constant factor affects the encrypted price component of the formula. By adjusting
// k3, you can control the additional cost associated with encrypted elections (encryptedVotes). A higher k3 value would
// make the price increase more significantly for elections that require encryption, while a lower k3 value would make
// the price less sensitive to the encryption requirement. The rationale behind k3 is to account for the extra
// computational effort and resources needed to ensure secrecy in encrypted elections.
//
// k4 (Anonymous price factor): This constant factor influences the anonymous price component of the formula. By
// adjusting k4, you can control the additional cost associated with anonymous elections (anonymousVotes). A higher k4
// value would make the price increase more significantly for elections that require anonymity, while a lower k4 value
// would make the price less sensitive to the anonymity requirement. The rationale behind k4 is to account for the extra
// measures and resources needed to ensure voter privacy in anonymous elections.
//
// k5 (Overwrite price factor): This constant factor affects the overwrite price component of the formula. By adjusting
// k5, you can control the additional cost associated with allowing vote overwrites (maxVoteOverwrite). A higher k5
// value would make the price increase more significantly for elections that permit vote overwrites, while a lower k5
// value would make the price less sensitive to the overwrite allowance. The rationale behind k5 is to account for the
// increased complexity and resources needed to manage vote overwrites in the election process.
//
// k6 (Non-linear growth factor): This constant factor determines the rate of price growth for elections with a maximum
// number of votes (maxCensusSize) exceeding the k7 threshold. By adjusting k6, you can control the non-linear growth
// rate of the price for larger elections. A higher k6 value would result in a more rapid increase in the price as the
// election size grows beyond the k7 threshold, while a lower k6 value would result in a slower increase in the price
// for larger elections. The rationale behind k6 is to provide a mechanism for controlling the pricing model's
// sensitivity to large elections. This factor ensures that the price accurately reflects the increased complexity,
// resource consumption, and management effort associated with larger elections, while maintaining a more affordable
// price for smaller elections. By fine-tuning k6, the pricing model can be adapted to balance accessibility for smaller
// elections with the need to cover costs and resource requirements for larger elections.
//
// k7 (Size non-linear trigger): This constant factor represents a threshold value for the maximum number of
// votes (maxCensusSize) in an election. When the election size exceeds k7, the price growth becomes non-linear,
// increasing more rapidly beyond this point. The rationale behind k7 is to create a pricing model that accommodates
// a "freemium" approach, where smaller elections (under the k7 threshold) are priced affordably, while larger elections
// are priced more significantly due to their increased resource requirements and complexity. By adjusting k7, you can
// control the point at which the price transition from linear to non-linear growth occurs. A higher k7 value would
// allow for more affordable pricing for a larger range of election sizes, while a lower k7 value would result in more
// rapid price increases for smaller election sizes. This flexibility enables the pricing model to be tailored to the
// specific needs and goals of the Vocdoni blockchain, ensuring that small elections remain accessible and affordable,
// while larger elections are priced to reflect their higher resource demands.
package electionprice

import (
	"sync"

	"go.vocdoni.io/dvote/types"
)

// ElectionParameters is a struct to group the input parameters for CalculatePrice method.
type ElectionParameters struct {
	MaxCensusSize           uint64 `json:"maxCensusSize"`
	ElectionDuration        uint32 `json:"electionBlocks"`
	ElectionDurationSeconds uint32 `json:"electionDuration"`
	EncryptedVotes          bool   `json:"encryptedVotes"`
	AnonymousVotes          bool   `json:"anonymousVotes"`
	MaxVoteOverwrite        uint32 `json:"maxVoteOverwrite"`
}

// Factors is a struct that stores the constant factors required for calculating
// the price of an election. These factors adjust the influence of different
// aspects of the election (size, duration, encryption, anonymity, overwrite count) on the final price.
type Factors struct {
	K1 float64 `json:"k1" example:"0.002"`  // sizePriceFactor
	K2 float64 `json:"k2" example:"0.0005"` // durationPriceFactor
	K3 float64 `json:"k3" example:"0.005"`  // encryptedPriceFactor
	K4 float64 `json:"k4" example:"10"`     // anonymousPriceFactor
	K5 float64 `json:"k5" example:"3"`      // overwritePriceFactor
	K6 float64 `json:"k6" example:"0.0008"` // Size scaling factor for maxCensusSize
	K7 int     `json:"k7" example:"200"`    // Threshold for maxCensusSize scaling
}

// Calculator is a struct that stores the constant factors and basePrice
// required for calculating the price of an election.
type Calculator struct {
	BasePrice uint64     `json:"basePrice" example:"5"`   // base price for an election
	Capacity  uint64     `json:"capacity" example:"2000"` // capacity of the blockchain
	Factors   Factors    `json:"factors"`                 // factors affecting the price
	mutex     sync.Mutex `json:"-"`                       // mutex for thread-safe operations
	Disable   bool       `json:"-"`                       // if true, disables the calculator and makes the Price function return 0
}

// DefaultElectionPriceFactors is the default set of constant factors used for calculating the price.
var DefaultElectionPriceFactors = Factors{
	K1: 0.002,
	K2: 0.0005,
	K3: 0.005,
	K4: 10,
	K5: 3,
	K6: 0.0008,
	K7: 200,
}

// NewElectionPriceCalculator creates a new PriceCalculator with the given constant factors.
// The basePrice and maxCapacity should be properly set before using the calculator.
func NewElectionPriceCalculator(factors Factors) *Calculator {
	return &Calculator{
		Factors: factors,
	}
}

// Price computes the price of an election given the parameters.
// The price is calculated using the following formula:
// price = basePrice + sizePrice + durationPrice + encryptedPrice + anonymousPrice + overwritePrice
//
// Parameters:
//   - MaxCensusSize: The maximum number of votes casted allowed for an election.
//   - ElectionDuration: The number of blocks the election can last. Currently the block time is 10 seconds.
//   - EncryptedVotes: A boolean flag that indicates if the election requires encryption keys for the
//     secret-until-the-end property.
//   - AnonymousVotes: A boolean flag that indicates if the election is anonymous or not.
//   - MaxVoteOverwrite: The number of overwrites a voter can execute after sending the first vote.
//
// The output is a uint64 value representing the price in a suitable unit.
func (p *Calculator) Price(params *ElectionParameters) uint64 {
	// If the calculator is disabled, return 0 as the price
	if p.Disable {
		return 0
	}

	// If the election duration is specified in seconds, convert it to blocks
	// This is a temporary solution until the block duration support is removed.
	if params.ElectionDurationSeconds > 0 {
		params.ElectionDuration = params.ElectionDurationSeconds / uint32(types.DefaultBlockTime.Seconds())
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Compute the sizePrice: the price based on the maxCensusSize
	// This uses a formula that scales based on the capacity of the blockchain
	// The extra scaling factor for maxCensusSize is only applied if maxCensusSize >= k7
	// This is to prevent a negative result causing an underflow error
	sizePriceFactor := p.Factors.K1 * float64(params.MaxCensusSize) *
		(1 - (1 / float64(p.Capacity)))
	if params.MaxCensusSize >= uint64(p.Factors.K7) {
		sizePriceFactor *= 1 + p.Factors.K6*(float64(params.MaxCensusSize)-float64(p.Factors.K7))
	}
	sizePrice := uint64(sizePriceFactor)

	// Compute the durationPrice: the price based on the election duration
	// This uses a formula that scales with both the duration and the maxCensusSize
	durationPriceFactor := p.Factors.K2 * float64(params.ElectionDuration) *
		(1 + float64(params.MaxCensusSize)/float64(p.Capacity))
	durationPrice := uint64(durationPriceFactor)

	// Compute the encryptedPrice: the price based on whether encryption is required
	// If encryption is required, the price is scaled based on the maxCensusSize
	encryptedPrice := uint64(0)
	if params.EncryptedVotes {
		encryptedPrice = uint64(p.Factors.K3 * float64(params.MaxCensusSize))
	}

	// Compute the anonymousPrice: the price based on whether the election is anonymous
	// If the election is anonymous, a flat price is added
	anonymousPrice := uint64(0)
	if params.AnonymousVotes {
		anonymousPrice = uint64(p.Factors.K4)
	}

	// Compute the overwritePrice: the price based on the maximum number of vote overwrites allowed
	// This uses a formula that scales based on the maxCensusSize and the maximum number of overwrites
	overwritePriceFactor := p.Factors.K5 * float64(params.MaxVoteOverwrite) / float64(p.Capacity)
	overwritePrice := uint64(overwritePriceFactor * float64(params.MaxCensusSize))

	// Sum all the prices to get the final price
	price := p.BasePrice + sizePrice + durationPrice + encryptedPrice + anonymousPrice + overwritePrice

	return price
}

// SetCapacity sets the current capacity of the blockchain.
func (p *Calculator) SetCapacity(capacity uint64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.Capacity = capacity
}

// SetBasePrice sets the current capacity of the blockchain.
// If the basePrice is set to 0, the calculator will be disabled.
func (p *Calculator) SetBasePrice(basePrice uint64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.BasePrice = basePrice
	p.Disable = basePrice == 0
}
