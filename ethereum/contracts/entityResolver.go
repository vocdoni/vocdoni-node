// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// EntityResolverABI is the input ABI used to generate the binding from.
const EntityResolverABI = "[{\"inputs\":[{\"internalType\":\"contractENS\",\"name\":\"_ens\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"contentType\",\"type\":\"uint256\"}],\"name\":\"ABIChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"a\",\"type\":\"address\"}],\"name\":\"AddrChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"coinType\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"newAddress\",\"type\":\"bytes\"}],\"name\":\"AddressChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"target\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"isAuthorised\",\"type\":\"bool\"}],\"name\":\"AuthorisationChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"hash\",\"type\":\"bytes\"}],\"name\":\"ContenthashChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"bytes4\",\"name\":\"interfaceID\",\"type\":\"bytes4\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"implementer\",\"type\":\"address\"}],\"name\":\"InterfaceChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"ListItemChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"ListItemRemoved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"name\",\"type\":\"string\"}],\"name\":\"NameChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"x\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"y\",\"type\":\"bytes32\"}],\"name\":\"PubkeyChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"indexedKey\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"}],\"name\":\"TextChanged\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"contentTypes\",\"type\":\"uint256\"}],\"name\":\"ABI\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"}],\"name\":\"addr\",\"outputs\":[{\"internalType\":\"addresspayable\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"coinType\",\"type\":\"uint256\"}],\"name\":\"addr\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"authorisations\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"}],\"name\":\"contenthash\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"internalType\":\"bytes4\",\"name\":\"interfaceID\",\"type\":\"bytes4\"}],\"name\":\"interfaceImplementer\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"}],\"name\":\"list\",\"outputs\":[{\"internalType\":\"string[]\",\"name\":\"\",\"type\":\"string[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"listText\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes[]\",\"name\":\"data\",\"type\":\"bytes[]\"}],\"name\":\"multicall\",\"outputs\":[{\"internalType\":\"bytes[]\",\"name\":\"results\",\"type\":\"bytes[]\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"}],\"name\":\"name\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"}],\"name\":\"pubkey\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"x\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"y\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"value\",\"type\":\"string\"}],\"name\":\"pushListText\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"removeListIndex\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"contentType\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"setABI\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"coinType\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"a\",\"type\":\"bytes\"}],\"name\":\"setAddr\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"a\",\"type\":\"address\"}],\"name\":\"setAddr\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"target\",\"type\":\"address\"},{\"internalType\":\"bool\",\"name\":\"isAuthorised\",\"type\":\"bool\"}],\"name\":\"setAuthorisation\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"hash\",\"type\":\"bytes\"}],\"name\":\"setContenthash\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"internalType\":\"bytes4\",\"name\":\"interfaceID\",\"type\":\"bytes4\"},{\"internalType\":\"address\",\"name\":\"implementer\",\"type\":\"address\"}],\"name\":\"setInterface\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"value\",\"type\":\"string\"}],\"name\":\"setListText\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"internalType\":\"string\",\"name\":\"name\",\"type\":\"string\"}],\"name\":\"setName\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"x\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"y\",\"type\":\"bytes32\"}],\"name\":\"setPubkey\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"value\",\"type\":\"string\"}],\"name\":\"setText\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes4\",\"name\":\"interfaceID\",\"type\":\"bytes4\"}],\"name\":\"supportsInterface\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"node\",\"type\":\"bytes32\"},{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"}],\"name\":\"text\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]"

// EntityResolverBin is the compiled bytecode used for deploying new contracts.
var EntityResolverBin = "0x608060405234801561001057600080fd5b50604051620022a6380380620022a683398101604081905261003191610056565b600880546001600160a01b0319166001600160a01b0392909216919091179055610084565b600060208284031215610067578081fd5b81516001600160a01b038116811461007d578182fd5b9392505050565b61221280620000946000396000f3fe608060405234801561001057600080fd5b50600436106101735760003560e01c80636f473720116100de578063bc1c58d111610097578063e59d895d11610071578063e59d895d1461036f578063f1cb7e0614610382578063f86bc87914610395578063fdf720c6146103a857610173565b8063bc1c58d114610328578063c86902331461033b578063d5fa2b001461035c57610173565b80636f4737201461029c57806374c756ee146102af57806377372213146102cf5780638b95dd71146102e2578063a2df33e1146102f5578063ac9650d81461030857610173565b8063304e6ade11610130578063304e6ade1461021d5780633b3b57de146102305780633e9ce7941461024357806359d1d43c14610256578063623195b014610276578063691f34311461028957610173565b806301ffc9a714610178578063043a728d146101a157806310f13a8c146101b6578063124a319c146101c95780632203ab56146101e957806329cd62ea1461020a575b600080fd5b61018b610186366004611e9d565b6103bb565b604051610198919061201f565b60405180910390f35b6101b46101af366004611c35565b6103ce565b005b6101b46101c4366004611c35565b6104c2565b6101dc6101d7366004611ba1565b610582565b6040516101989190611f58565b6101fc6101f7366004611d7d565b6107ad565b604051610198929190612123565b6101b4610218366004611b76565b6108ce565b6101b461022b366004611beb565b610961565b6101dc61023e366004611ab4565b6109d3565b6101b4610251366004611b3c565b610a08565b610269610264366004611beb565b610a82565b6040516101989190612072565b6101b4610284366004611d9e565b610b44565b610269610297366004611ab4565b610bd2565b6101b46102aa366004611cac565b610c73565b6102c26102bd366004611beb565b610df3565b6040516101989190611fcc565b6101b46102dd366004611beb565b610f07565b6101b46102f0366004611def565b610f79565b6101b4610303366004611cfd565b611051565b61031b610316366004611a45565b611161565b6040516101989190611f6c565b610269610336366004611ab4565b611278565b61034e610349366004611ab4565b6112e0565b604051610198929190612033565b6101b461036a366004611acc565b6112fa565b6101b461037d366004611bc5565b611334565b610269610390366004611d7d565b6113d7565b61018b6103a3366004611afb565b611480565b6102696103b6366004611cac565b6114a6565b60006103c682611597565b90505b919050565b846103d8816115b4565b6103fd5760405162461bcd60e51b81526004016103f4906120d0565b60405180910390fd5b60008585604051602001610412929190611f2c565b60408051601f19818403018152918152815160209283012060008a815260078452828120828252845291822080546001810182559083529290912090925061045c9101858561182c565b5060008781526007602090815260408083208484529091529081902054905188917fad6a325380d85c50dfaa38511cbbbc97c46b9a52e342c8d705bf5515d0ef45b3916104b1918a918a916000190190612085565b60405180910390a250505050505050565b846104cc816115b4565b6104e85760405162461bcd60e51b81526004016103f4906120d0565b828260066000898152602001908152602001600020878760405161050d929190611f2c565b90815260405190819003602001902061052792909161182c565b508484604051610538929190611f2c565b6040518091039020867fd8c9334b1a9c2f9da342a0a2b32629c1a229b6445dad78947f674b44444a75508787604051610572929190612056565b60405180910390a3505050505050565b60008281526003602090815260408083206001600160e01b0319851684529091528120546001600160a01b031680156105bc5790506107a7565b60006105c7856109d3565b90506001600160a01b0381166105e2576000925050506107a7565b60006060826001600160a01b03166301ffc9a760e01b6040516024016106089190612041565b60408051601f198184030181529181526020820180516001600160e01b03166301ffc9a760e01b1790525161063d9190611f3c565b600060405180830381855afa9150503d8060008114610678576040519150601f19603f3d011682016040523d82523d6000602084013e61067d565b606091505b5091509150811580610690575060208151105b806106b4575080601f815181106106a357fe5b01602001516001600160f81b031916155b156106c65760009450505050506107a7565b826001600160a01b0316866040516024016106e19190612041565b60408051601f198184030181529181526020820180516001600160e01b03166301ffc9a760e01b179052516107169190611f3c565b600060405180830381855afa9150503d8060008114610751576040519150601f19603f3d011682016040523d82523d6000602084013e610756565b606091505b50909250905081158061076a575060208151105b8061078e575080601f8151811061077d57fe5b01602001516001600160f81b031916155b156107a05760009450505050506107a7565b5090925050505b92915050565b600082815260208190526040812060609060015b8481116108ae57808516158015906107f957506000818152602083905260409020546002600019610100600184161502019091160415155b156108a6576000818152602083815260409182902080548351601f60026000196101006001861615020190931692909204918201849004840281018401909452808452849391928391908301828280156108945780601f1061086957610100808354040283529160200191610894565b820191906000526020600020905b81548152906001019060200180831161087757829003601f168201915b505050505090509350935050506108c7565b60011b6107c1565b5060006040518060200160405280600081525092509250505b9250929050565b826108d8816115b4565b6108f45760405162461bcd60e51b81526004016103f4906120d0565b6040805180820182528481526020808201858152600088815260059092529083902091518255516001909101555184907f1d6f5e03d3f63eb58751986629a5439baee5079ff04f345becb66e23eb154e46906109539086908690612033565b60405180910390a250505050565b8261096b816115b4565b6109875760405162461bcd60e51b81526004016103f4906120d0565b60008481526002602052604090206109a090848461182c565b50837fe379c1624ed7e714cc0937528a32359d69d5281337765313dba4e081b72d75788484604051610953929190612056565b600060606109e283603c6113d7565b90508051600014156109f85760009150506103c9565b610a01816116cd565b9392505050565b6000838152600960209081526040808320338085529083528184206001600160a01b038716808652935292819020805460ff19168515151790555190919085907fe1c5610a6e0cbe10764ecd182adcef1ec338dc4e199c99c32ce98f38e12791df90610a7590869061201f565b60405180910390a4505050565b6060600660008581526020019081526020016000208383604051610aa7929190611f2c565b9081526040805160209281900383018120805460026001821615610100026000190190911604601f81018590048502830185019093528282529092909190830182828015610b365780601f10610b0b57610100808354040283529160200191610b36565b820191906000526020600020905b815481529060010190602001808311610b1957829003601f168201915b505050505090509392505050565b83610b4e816115b4565b610b6a5760405162461bcd60e51b81526004016103f4906120d0565b6000198401841615610b7b57600080fd5b6000858152602081815260408083208784529091529020610b9d90848461182c565b50604051849086907faa121bbeef5f32f5961a2a28966e769023910fc9479059ee3495d4c1a696efe390600090a35050505050565b60008181526004602090815260409182902080548351601f6002600019610100600186161502019093169290920491820184900484028101840190945280845260609392830182828015610c675780601f10610c3c57610100808354040283529160200191610c67565b820191906000526020600020905b815481529060010190602001808311610c4a57829003601f168201915b50505050509050919050565b83610c7d816115b4565b610c995760405162461bcd60e51b81526004016103f4906120d0565b60008484604051602001610cae929190611f2c565b60408051601f1981840301815291815281516020928301206000898152600784528281208282529093529120549091508310610cfc5760405162461bcd60e51b81526004016103f4906120a9565b600086815260076020908152604080832084845290915290208054906000198201828110610d2657fe5b600091825260208083208a845260078252604080852087865290925292208054929091019186908110610d5557fe5b906000526020600020019080546001816001161561010002031660029004610d7e9291906118aa565b5060008781526007602090815260408083208584529091529020805480610da157fe5b600190038181906000526020600020016000610dbd919061191f565b9055867f5dbbe7a1e616b629a61480b13fd4e89dfe8b604d35802cdf7b0bb03687e3eb498787876040516104b193929190612085565b606060008383604051602001610e0a929190611f2c565b60408051601f198184030181528282528051602091820120600089815260078352838120828252835283812080548085028701850190955284865291955090929184015b82821015610ef95760008481526020908190208301805460408051601f6002600019610100600187161502019094169390930492830185900485028101850190915281815292830182828015610ee55780601f10610eba57610100808354040283529160200191610ee5565b820191906000526020600020905b815481529060010190602001808311610ec857829003601f168201915b505050505081526020019060010190610e4e565b505050509150509392505050565b82610f11816115b4565b610f2d5760405162461bcd60e51b81526004016103f4906120d0565b6000848152600460205260409020610f4690848461182c565b50837fb7d29e911041e8d9b843369e890bcb72c9388692ba48b65ac54e7214c4c348f78484604051610953929190612056565b82610f83816115b4565b610f9f5760405162461bcd60e51b81526004016103f4906120d0565b837f65412581168e88a1e60c6459d7f44ae83ad0832e670826c05a4e2476b57af7528484604051610fd1929190612123565b60405180910390a2603c83141561102357837f52d7d861f09ab3d26239d492e8968629f95e9e318cf0b73bfddc441522a15fd261100d846116cd565b60405161101a9190611f58565b60405180910390a25b60008481526001602090815260408083208684528252909120835161104a92850190611966565b5050505050565b8561105b816115b4565b6110775760405162461bcd60e51b81526004016103f4906120d0565b6000868660405160200161108c929190611f2c565b60408051601f19818403018152918152815160209283012060008b81526007845282812082825290935291205490915085106110da5760405162461bcd60e51b81526004016103f4906120a9565b600088815260076020908152604080832084845290915290208054859185918890811061110357fe5b90600052602060002001919061111a92919061182c565b50877fad6a325380d85c50dfaa38511cbbbc97c46b9a52e342c8d705bf5515d0ef45b388888860405161114f93929190612085565b60405180910390a25050505050505050565b60608167ffffffffffffffff8111801561117a57600080fd5b506040519080825280602002602001820160405280156111ae57816020015b60608152602001906001900390816111995790505b50905060005b828110156112715760006060308686858181106111cd57fe5b90506020028101906111df919061213c565b6040516111ed929190611f2c565b600060405180830381855af49150503d8060008114611228576040519150601f19603f3d011682016040523d82523d6000602084013e61122d565b606091505b50915091508161124f5760405162461bcd60e51b81526004016103f4906120f6565b8084848151811061125c57fe5b602090810291909101015250506001016111b4565b5092915050565b600081815260026020818152604092839020805484516001821615610100026000190190911693909304601f81018390048302840183019094528383526060939091830182828015610c675780601f10610c3c57610100808354040283529160200191610c67565b600090815260056020526040902080546001909101549091565b81611304816115b4565b6113205760405162461bcd60e51b81526004016103f4906120d0565b61132f83603c6102f0856116ec565b505050565b8261133e816115b4565b61135a5760405162461bcd60e51b81526004016103f4906120d0565b60008481526003602090815260408083206001600160e01b0319871680855292529182902080546001600160a01b0319166001600160a01b038616179055905185907f7c69f06bea0bdef565b709e93a147836b0063ba2dd89f02d0b7e8d931e6a6daa906113c9908690611f58565b60405180910390a350505050565b600082815260016020818152604080842085855282529283902080548451600294821615610100026000190190911693909304601f810183900483028401830190945283835260609390918301828280156114735780601f1061144857610100808354040283529160200191611473565b820191906000526020600020905b81548152906001019060200180831161145657829003601f168201915b5050505050905092915050565b600960209081526000938452604080852082529284528284209052825290205460ff1681565b6060600084846040516020016114bd929190611f2c565b60408051601f1981840301815291815281516020928301206000898152600784528281208282529093529120805491925090849081106114f957fe5b600091825260209182902001805460408051601f60026000196101006001871615020190941693909304928301859004850281018501909152818152928301828280156115875780601f1061155c57610100808354040283529160200191611587565b820191906000526020600020905b81548152906001019060200180831161156a57829003601f168201915b5050505050915050949350505050565b60006001600160e01b0319821615806103c657506103c68261171c565b6008546000906001600160a01b03161561169d576008546040516302571be360e01b81526000916001600160a01b0316906302571be3906115f990869060040161202a565b60206040518083038186803b15801561161157600080fd5b505afa158015611625573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906116499190611a29565b90506001600160a01b03811633148061168c575060008381526009602090815260408083206001600160a01b0385168452825280832033845290915290205460ff165b1561169b5760019150506103c9565b505b81336040516020016116af9190611f0f565b60405160208183030381529060405280519060200120149050919050565b600081516014146116dd57600080fd5b5060200151600160601b900490565b604080516014808252818301909252606091602082018180368337505050600160601b9290920260208301525090565b60006001600160e01b03198216631674750f60e21b14806103c657506103c68260006001600160e01b0319821663c869023360e01b14806103c657506103c68260006001600160e01b0319821663691f343160e01b14806103c657506103c68260006001600160e01b031982166304928c6760e21b14806103c657506103c68260006001600160e01b0319821663bc1c58d160e01b14806103c657506103c68260006001600160e01b03198216631d9dabef60e11b14806117ed57506001600160e01b031982166378e5bf0360e11b145b806103c657506103c68260006001600160e01b03198216631101d5ab60e11b14806103c657506301ffc9a760e01b6001600160e01b03198316146103c6565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f1061186d5782800160ff1982351617855561189a565b8280016001018555821561189a579182015b8281111561189a57823582559160200191906001019061187f565b506118a69291506119d4565b5090565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f106118e3578054855561189a565b8280016001018555821561189a57600052602060002091601f016020900482015b8281111561189a578254825591600101919060010190611904565b50805460018160011615610100020316600290046000825580601f106119455750611963565b601f01602090049060005260206000209081019061196391906119d4565b50565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f106119a757805160ff191683800117855561189a565b8280016001018555821561189a579182015b8281111561189a5782518255916020019190600101906119b9565b5b808211156118a657600081556001016119d5565b60008083601f8401126119fa578182fd5b50813567ffffffffffffffff811115611a11578182fd5b6020830191508360208285010111156108c757600080fd5b600060208284031215611a3a578081fd5b8151610a01816121b1565b60008060208385031215611a57578081fd5b823567ffffffffffffffff80821115611a6e578283fd5b818501915085601f830112611a81578283fd5b813581811115611a8f578384fd5b8660208083028501011115611aa2578384fd5b60209290920196919550909350505050565b600060208284031215611ac5578081fd5b5035919050565b60008060408385031215611ade578182fd5b823591506020830135611af0816121b1565b809150509250929050565b600080600060608486031215611b0f578081fd5b833592506020840135611b21816121b1565b91506040840135611b31816121b1565b809150509250925092565b600080600060608486031215611b50578283fd5b833592506020840135611b62816121b1565b915060408401358015158114611b31578182fd5b600080600060608486031215611b8a578283fd5b505081359360208301359350604090920135919050565b60008060408385031215611bb3578182fd5b823591506020830135611af0816121c6565b600080600060608486031215611bd9578283fd5b833592506020840135611b21816121c6565b600080600060408486031215611bff578283fd5b83359250602084013567ffffffffffffffff811115611c1c578283fd5b611c28868287016119e9565b9497909650939450505050565b600080600080600060608688031215611c4c578081fd5b85359450602086013567ffffffffffffffff80821115611c6a578283fd5b611c7689838a016119e9565b90965094506040880135915080821115611c8e578283fd5b50611c9b888289016119e9565b969995985093965092949392505050565b60008060008060608587031215611cc1578182fd5b84359350602085013567ffffffffffffffff811115611cde578283fd5b611cea878288016119e9565b9598909750949560400135949350505050565b60008060008060008060808789031215611d15578384fd5b86359550602087013567ffffffffffffffff80821115611d33578586fd5b611d3f8a838b016119e9565b9097509550604089013594506060890135915080821115611d5e578283fd5b50611d6b89828a016119e9565b979a9699509497509295939492505050565b60008060408385031215611d8f578182fd5b50508035926020909101359150565b60008060008060608587031215611db3578182fd5b8435935060208501359250604085013567ffffffffffffffff811115611dd7578283fd5b611de3878288016119e9565b95989497509550505050565b600080600060608486031215611e03578081fd5b833592506020808501359250604085013567ffffffffffffffff80821115611e29578384fd5b818701915087601f830112611e3c578384fd5b813581811115611e4a578485fd5b604051601f8201601f1916810185018381118282101715611e69578687fd5b60405281815283820185018a1015611e7f578586fd5b81858501868301378585838301015280955050505050509250925092565b600060208284031215611eae578081fd5b8135610a01816121c6565b60008284528282602086013780602084860101526020601f19601f85011685010190509392505050565b60008151808452611efb816020860160208601612181565b601f01601f19169290920160200192915050565b60609190911b6bffffffffffffffffffffffff1916815260140190565b6000828483379101908152919050565b60008251611f4e818460208701612181565b9190910192915050565b6001600160a01b0391909116815260200190565b6000602080830181845280855180835260408601915060408482028701019250838701855b82811015611fbf57603f19888603018452611fad858351611ee3565b94509285019290850190600101611f91565b5092979650505050505050565b6000602080830181845280855180835260408601915060408482028701019250838701855b82811015611fbf57603f1988860301845261200d858351611ee3565b94509285019290850190600101611ff1565b901515815260200190565b90815260200190565b918252602082015260400190565b6001600160e01b031991909116815260200190565b60006020825261206a602083018486611eb9565b949350505050565b600060208252610a016020830184611ee3565b600060408252612099604083018587611eb9565b9050826020830152949350505050565b6020808252600d908201526c092dcecc2d8d2c840d2dcc8caf609b1b604082015260600190565b6020808252600c908201526b1d5b985d5d1a1bdc9a5e995960a21b604082015260600190565b60208082526013908201527219985a5b19590819195b1959d85d1958d85b1b606a1b604082015260600190565b60008382526040602083015261206a6040830184611ee3565b6000808335601e19843603018112612152578283fd5b83018035915067ffffffffffffffff82111561216c578283fd5b6020019150368190038213156108c757600080fd5b60005b8381101561219c578181015183820152602001612184565b838111156121ab576000848401525b50505050565b6001600160a01b038116811461196357600080fd5b6001600160e01b03198116811461196357600080fdfea264697066735822122004b7d1d378f58e56383f2e67635d406bc58a6eb17c5d278b7fd8c5532b15861964736f6c634300060c0033"

// DeployEntityResolver deploys a new Ethereum contract, binding an instance of EntityResolver to it.
func DeployEntityResolver(auth *bind.TransactOpts, backend bind.ContractBackend, _ens common.Address) (common.Address, *types.Transaction, *EntityResolver, error) {
	parsed, err := abi.JSON(strings.NewReader(EntityResolverABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(EntityResolverBin), backend, _ens)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &EntityResolver{EntityResolverCaller: EntityResolverCaller{contract: contract}, EntityResolverTransactor: EntityResolverTransactor{contract: contract}, EntityResolverFilterer: EntityResolverFilterer{contract: contract}}, nil
}

// EntityResolver is an auto generated Go binding around an Ethereum contract.
type EntityResolver struct {
	EntityResolverCaller     // Read-only binding to the contract
	EntityResolverTransactor // Write-only binding to the contract
	EntityResolverFilterer   // Log filterer for contract events
}

// EntityResolverCaller is an auto generated read-only Go binding around an Ethereum contract.
type EntityResolverCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EntityResolverTransactor is an auto generated write-only Go binding around an Ethereum contract.
type EntityResolverTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EntityResolverFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type EntityResolverFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EntityResolverSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type EntityResolverSession struct {
	Contract     *EntityResolver   // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// EntityResolverCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type EntityResolverCallerSession struct {
	Contract *EntityResolverCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts         // Call options to use throughout this session
}

// EntityResolverTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type EntityResolverTransactorSession struct {
	Contract     *EntityResolverTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts         // Transaction auth options to use throughout this session
}

// EntityResolverRaw is an auto generated low-level Go binding around an Ethereum contract.
type EntityResolverRaw struct {
	Contract *EntityResolver // Generic contract binding to access the raw methods on
}

// EntityResolverCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type EntityResolverCallerRaw struct {
	Contract *EntityResolverCaller // Generic read-only contract binding to access the raw methods on
}

// EntityResolverTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type EntityResolverTransactorRaw struct {
	Contract *EntityResolverTransactor // Generic write-only contract binding to access the raw methods on
}

// NewEntityResolver creates a new instance of EntityResolver, bound to a specific deployed contract.
func NewEntityResolver(address common.Address, backend bind.ContractBackend) (*EntityResolver, error) {
	contract, err := bindEntityResolver(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &EntityResolver{EntityResolverCaller: EntityResolverCaller{contract: contract}, EntityResolverTransactor: EntityResolverTransactor{contract: contract}, EntityResolverFilterer: EntityResolverFilterer{contract: contract}}, nil
}

// NewEntityResolverCaller creates a new read-only instance of EntityResolver, bound to a specific deployed contract.
func NewEntityResolverCaller(address common.Address, caller bind.ContractCaller) (*EntityResolverCaller, error) {
	contract, err := bindEntityResolver(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &EntityResolverCaller{contract: contract}, nil
}

// NewEntityResolverTransactor creates a new write-only instance of EntityResolver, bound to a specific deployed contract.
func NewEntityResolverTransactor(address common.Address, transactor bind.ContractTransactor) (*EntityResolverTransactor, error) {
	contract, err := bindEntityResolver(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &EntityResolverTransactor{contract: contract}, nil
}

// NewEntityResolverFilterer creates a new log filterer instance of EntityResolver, bound to a specific deployed contract.
func NewEntityResolverFilterer(address common.Address, filterer bind.ContractFilterer) (*EntityResolverFilterer, error) {
	contract, err := bindEntityResolver(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &EntityResolverFilterer{contract: contract}, nil
}

// bindEntityResolver binds a generic wrapper to an already deployed contract.
func bindEntityResolver(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(EntityResolverABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_EntityResolver *EntityResolverRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _EntityResolver.Contract.EntityResolverCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_EntityResolver *EntityResolverRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _EntityResolver.Contract.EntityResolverTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_EntityResolver *EntityResolverRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _EntityResolver.Contract.EntityResolverTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_EntityResolver *EntityResolverCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _EntityResolver.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_EntityResolver *EntityResolverTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _EntityResolver.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_EntityResolver *EntityResolverTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _EntityResolver.Contract.contract.Transact(opts, method, params...)
}

// ABI is a free data retrieval call binding the contract method 0x2203ab56.
//
// Solidity: function ABI(bytes32 node, uint256 contentTypes) view returns(uint256, bytes)
func (_EntityResolver *EntityResolverCaller) ABI(opts *bind.CallOpts, node [32]byte, contentTypes *big.Int) (*big.Int, []byte, error) {
	var out []interface{}
	err := _EntityResolver.contract.Call(opts, &out, "ABI", node, contentTypes)
	if err != nil {
		return *new(*big.Int), *new([]byte), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	out1 := *abi.ConvertType(out[1], new([]byte)).(*[]byte)

	return out0, out1, err
}

// ABI is a free data retrieval call binding the contract method 0x2203ab56.
//
// Solidity: function ABI(bytes32 node, uint256 contentTypes) view returns(uint256, bytes)
func (_EntityResolver *EntityResolverSession) ABI(node [32]byte, contentTypes *big.Int) (*big.Int, []byte, error) {
	return _EntityResolver.Contract.ABI(&_EntityResolver.CallOpts, node, contentTypes)
}

// ABI is a free data retrieval call binding the contract method 0x2203ab56.
//
// Solidity: function ABI(bytes32 node, uint256 contentTypes) view returns(uint256, bytes)
func (_EntityResolver *EntityResolverCallerSession) ABI(node [32]byte, contentTypes *big.Int) (*big.Int, []byte, error) {
	return _EntityResolver.Contract.ABI(&_EntityResolver.CallOpts, node, contentTypes)
}

// Addr is a free data retrieval call binding the contract method 0x3b3b57de.
//
// Solidity: function addr(bytes32 node) view returns(address)
func (_EntityResolver *EntityResolverCaller) Addr(opts *bind.CallOpts, node [32]byte) (common.Address, error) {
	var out []interface{}
	err := _EntityResolver.contract.Call(opts, &out, "addr", node)
	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err
}

// Addr is a free data retrieval call binding the contract method 0x3b3b57de.
//
// Solidity: function addr(bytes32 node) view returns(address)
func (_EntityResolver *EntityResolverSession) Addr(node [32]byte) (common.Address, error) {
	return _EntityResolver.Contract.Addr(&_EntityResolver.CallOpts, node)
}

// Addr is a free data retrieval call binding the contract method 0x3b3b57de.
//
// Solidity: function addr(bytes32 node) view returns(address)
func (_EntityResolver *EntityResolverCallerSession) Addr(node [32]byte) (common.Address, error) {
	return _EntityResolver.Contract.Addr(&_EntityResolver.CallOpts, node)
}

// Addr0 is a free data retrieval call binding the contract method 0xf1cb7e06.
//
// Solidity: function addr(bytes32 node, uint256 coinType) view returns(bytes)
func (_EntityResolver *EntityResolverCaller) Addr0(opts *bind.CallOpts, node [32]byte, coinType *big.Int) ([]byte, error) {
	var out []interface{}
	err := _EntityResolver.contract.Call(opts, &out, "addr0", node, coinType)
	if err != nil {
		return *new([]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([]byte)).(*[]byte)

	return out0, err
}

// Addr0 is a free data retrieval call binding the contract method 0xf1cb7e06.
//
// Solidity: function addr(bytes32 node, uint256 coinType) view returns(bytes)
func (_EntityResolver *EntityResolverSession) Addr0(node [32]byte, coinType *big.Int) ([]byte, error) {
	return _EntityResolver.Contract.Addr0(&_EntityResolver.CallOpts, node, coinType)
}

// Addr0 is a free data retrieval call binding the contract method 0xf1cb7e06.
//
// Solidity: function addr(bytes32 node, uint256 coinType) view returns(bytes)
func (_EntityResolver *EntityResolverCallerSession) Addr0(node [32]byte, coinType *big.Int) ([]byte, error) {
	return _EntityResolver.Contract.Addr0(&_EntityResolver.CallOpts, node, coinType)
}

// Authorisations is a free data retrieval call binding the contract method 0xf86bc879.
//
// Solidity: function authorisations(bytes32 , address , address ) view returns(bool)
func (_EntityResolver *EntityResolverCaller) Authorisations(opts *bind.CallOpts, arg0 [32]byte, arg1 common.Address, arg2 common.Address) (bool, error) {
	var out []interface{}
	err := _EntityResolver.contract.Call(opts, &out, "authorisations", arg0, arg1, arg2)
	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err
}

// Authorisations is a free data retrieval call binding the contract method 0xf86bc879.
//
// Solidity: function authorisations(bytes32 , address , address ) view returns(bool)
func (_EntityResolver *EntityResolverSession) Authorisations(arg0 [32]byte, arg1 common.Address, arg2 common.Address) (bool, error) {
	return _EntityResolver.Contract.Authorisations(&_EntityResolver.CallOpts, arg0, arg1, arg2)
}

// Authorisations is a free data retrieval call binding the contract method 0xf86bc879.
//
// Solidity: function authorisations(bytes32 , address , address ) view returns(bool)
func (_EntityResolver *EntityResolverCallerSession) Authorisations(arg0 [32]byte, arg1 common.Address, arg2 common.Address) (bool, error) {
	return _EntityResolver.Contract.Authorisations(&_EntityResolver.CallOpts, arg0, arg1, arg2)
}

// Contenthash is a free data retrieval call binding the contract method 0xbc1c58d1.
//
// Solidity: function contenthash(bytes32 node) view returns(bytes)
func (_EntityResolver *EntityResolverCaller) Contenthash(opts *bind.CallOpts, node [32]byte) ([]byte, error) {
	var out []interface{}
	err := _EntityResolver.contract.Call(opts, &out, "contenthash", node)
	if err != nil {
		return *new([]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([]byte)).(*[]byte)

	return out0, err
}

// Contenthash is a free data retrieval call binding the contract method 0xbc1c58d1.
//
// Solidity: function contenthash(bytes32 node) view returns(bytes)
func (_EntityResolver *EntityResolverSession) Contenthash(node [32]byte) ([]byte, error) {
	return _EntityResolver.Contract.Contenthash(&_EntityResolver.CallOpts, node)
}

// Contenthash is a free data retrieval call binding the contract method 0xbc1c58d1.
//
// Solidity: function contenthash(bytes32 node) view returns(bytes)
func (_EntityResolver *EntityResolverCallerSession) Contenthash(node [32]byte) ([]byte, error) {
	return _EntityResolver.Contract.Contenthash(&_EntityResolver.CallOpts, node)
}

// InterfaceImplementer is a free data retrieval call binding the contract method 0x124a319c.
//
// Solidity: function interfaceImplementer(bytes32 node, bytes4 interfaceID) view returns(address)
func (_EntityResolver *EntityResolverCaller) InterfaceImplementer(opts *bind.CallOpts, node [32]byte, interfaceID [4]byte) (common.Address, error) {
	var out []interface{}
	err := _EntityResolver.contract.Call(opts, &out, "interfaceImplementer", node, interfaceID)
	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err
}

// InterfaceImplementer is a free data retrieval call binding the contract method 0x124a319c.
//
// Solidity: function interfaceImplementer(bytes32 node, bytes4 interfaceID) view returns(address)
func (_EntityResolver *EntityResolverSession) InterfaceImplementer(node [32]byte, interfaceID [4]byte) (common.Address, error) {
	return _EntityResolver.Contract.InterfaceImplementer(&_EntityResolver.CallOpts, node, interfaceID)
}

// InterfaceImplementer is a free data retrieval call binding the contract method 0x124a319c.
//
// Solidity: function interfaceImplementer(bytes32 node, bytes4 interfaceID) view returns(address)
func (_EntityResolver *EntityResolverCallerSession) InterfaceImplementer(node [32]byte, interfaceID [4]byte) (common.Address, error) {
	return _EntityResolver.Contract.InterfaceImplementer(&_EntityResolver.CallOpts, node, interfaceID)
}

// List is a free data retrieval call binding the contract method 0x74c756ee.
//
// Solidity: function list(bytes32 node, string key) view returns(string[])
func (_EntityResolver *EntityResolverCaller) List(opts *bind.CallOpts, node [32]byte, key string) ([]string, error) {
	var out []interface{}
	err := _EntityResolver.contract.Call(opts, &out, "list", node, key)
	if err != nil {
		return *new([]string), err
	}

	out0 := *abi.ConvertType(out[0], new([]string)).(*[]string)

	return out0, err
}

// List is a free data retrieval call binding the contract method 0x74c756ee.
//
// Solidity: function list(bytes32 node, string key) view returns(string[])
func (_EntityResolver *EntityResolverSession) List(node [32]byte, key string) ([]string, error) {
	return _EntityResolver.Contract.List(&_EntityResolver.CallOpts, node, key)
}

// List is a free data retrieval call binding the contract method 0x74c756ee.
//
// Solidity: function list(bytes32 node, string key) view returns(string[])
func (_EntityResolver *EntityResolverCallerSession) List(node [32]byte, key string) ([]string, error) {
	return _EntityResolver.Contract.List(&_EntityResolver.CallOpts, node, key)
}

// ListText is a free data retrieval call binding the contract method 0xfdf720c6.
//
// Solidity: function listText(bytes32 node, string key, uint256 index) view returns(string)
func (_EntityResolver *EntityResolverCaller) ListText(opts *bind.CallOpts, node [32]byte, key string, index *big.Int) (string, error) {
	var out []interface{}
	err := _EntityResolver.contract.Call(opts, &out, "listText", node, key, index)
	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err
}

// ListText is a free data retrieval call binding the contract method 0xfdf720c6.
//
// Solidity: function listText(bytes32 node, string key, uint256 index) view returns(string)
func (_EntityResolver *EntityResolverSession) ListText(node [32]byte, key string, index *big.Int) (string, error) {
	return _EntityResolver.Contract.ListText(&_EntityResolver.CallOpts, node, key, index)
}

// ListText is a free data retrieval call binding the contract method 0xfdf720c6.
//
// Solidity: function listText(bytes32 node, string key, uint256 index) view returns(string)
func (_EntityResolver *EntityResolverCallerSession) ListText(node [32]byte, key string, index *big.Int) (string, error) {
	return _EntityResolver.Contract.ListText(&_EntityResolver.CallOpts, node, key, index)
}

// Name is a free data retrieval call binding the contract method 0x691f3431.
//
// Solidity: function name(bytes32 node) view returns(string)
func (_EntityResolver *EntityResolverCaller) Name(opts *bind.CallOpts, node [32]byte) (string, error) {
	var out []interface{}
	err := _EntityResolver.contract.Call(opts, &out, "name", node)
	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err
}

// Name is a free data retrieval call binding the contract method 0x691f3431.
//
// Solidity: function name(bytes32 node) view returns(string)
func (_EntityResolver *EntityResolverSession) Name(node [32]byte) (string, error) {
	return _EntityResolver.Contract.Name(&_EntityResolver.CallOpts, node)
}

// Name is a free data retrieval call binding the contract method 0x691f3431.
//
// Solidity: function name(bytes32 node) view returns(string)
func (_EntityResolver *EntityResolverCallerSession) Name(node [32]byte) (string, error) {
	return _EntityResolver.Contract.Name(&_EntityResolver.CallOpts, node)
}

// Pubkey is a free data retrieval call binding the contract method 0xc8690233.
//
// Solidity: function pubkey(bytes32 node) view returns(bytes32 x, bytes32 y)
func (_EntityResolver *EntityResolverCaller) Pubkey(opts *bind.CallOpts, node [32]byte) (struct {
	X [32]byte
	Y [32]byte
}, error) {
	var out []interface{}
	err := _EntityResolver.contract.Call(opts, &out, "pubkey", node)

	outstruct := new(struct {
		X [32]byte
		Y [32]byte
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.X = *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)
	outstruct.Y = *abi.ConvertType(out[1], new([32]byte)).(*[32]byte)

	return *outstruct, err
}

// Pubkey is a free data retrieval call binding the contract method 0xc8690233.
//
// Solidity: function pubkey(bytes32 node) view returns(bytes32 x, bytes32 y)
func (_EntityResolver *EntityResolverSession) Pubkey(node [32]byte) (struct {
	X [32]byte
	Y [32]byte
}, error) {
	return _EntityResolver.Contract.Pubkey(&_EntityResolver.CallOpts, node)
}

// Pubkey is a free data retrieval call binding the contract method 0xc8690233.
//
// Solidity: function pubkey(bytes32 node) view returns(bytes32 x, bytes32 y)
func (_EntityResolver *EntityResolverCallerSession) Pubkey(node [32]byte) (struct {
	X [32]byte
	Y [32]byte
}, error) {
	return _EntityResolver.Contract.Pubkey(&_EntityResolver.CallOpts, node)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceID) pure returns(bool)
func (_EntityResolver *EntityResolverCaller) SupportsInterface(opts *bind.CallOpts, interfaceID [4]byte) (bool, error) {
	var out []interface{}
	err := _EntityResolver.contract.Call(opts, &out, "supportsInterface", interfaceID)
	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceID) pure returns(bool)
func (_EntityResolver *EntityResolverSession) SupportsInterface(interfaceID [4]byte) (bool, error) {
	return _EntityResolver.Contract.SupportsInterface(&_EntityResolver.CallOpts, interfaceID)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceID) pure returns(bool)
func (_EntityResolver *EntityResolverCallerSession) SupportsInterface(interfaceID [4]byte) (bool, error) {
	return _EntityResolver.Contract.SupportsInterface(&_EntityResolver.CallOpts, interfaceID)
}

// Text is a free data retrieval call binding the contract method 0x59d1d43c.
//
// Solidity: function text(bytes32 node, string key) view returns(string)
func (_EntityResolver *EntityResolverCaller) Text(opts *bind.CallOpts, node [32]byte, key string) (string, error) {
	var out []interface{}
	err := _EntityResolver.contract.Call(opts, &out, "text", node, key)
	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err
}

// Text is a free data retrieval call binding the contract method 0x59d1d43c.
//
// Solidity: function text(bytes32 node, string key) view returns(string)
func (_EntityResolver *EntityResolverSession) Text(node [32]byte, key string) (string, error) {
	return _EntityResolver.Contract.Text(&_EntityResolver.CallOpts, node, key)
}

// Text is a free data retrieval call binding the contract method 0x59d1d43c.
//
// Solidity: function text(bytes32 node, string key) view returns(string)
func (_EntityResolver *EntityResolverCallerSession) Text(node [32]byte, key string) (string, error) {
	return _EntityResolver.Contract.Text(&_EntityResolver.CallOpts, node, key)
}

// Multicall is a paid mutator transaction binding the contract method 0xac9650d8.
//
// Solidity: function multicall(bytes[] data) returns(bytes[] results)
func (_EntityResolver *EntityResolverTransactor) Multicall(opts *bind.TransactOpts, data [][]byte) (*types.Transaction, error) {
	return _EntityResolver.contract.Transact(opts, "multicall", data)
}

// Multicall is a paid mutator transaction binding the contract method 0xac9650d8.
//
// Solidity: function multicall(bytes[] data) returns(bytes[] results)
func (_EntityResolver *EntityResolverSession) Multicall(data [][]byte) (*types.Transaction, error) {
	return _EntityResolver.Contract.Multicall(&_EntityResolver.TransactOpts, data)
}

// Multicall is a paid mutator transaction binding the contract method 0xac9650d8.
//
// Solidity: function multicall(bytes[] data) returns(bytes[] results)
func (_EntityResolver *EntityResolverTransactorSession) Multicall(data [][]byte) (*types.Transaction, error) {
	return _EntityResolver.Contract.Multicall(&_EntityResolver.TransactOpts, data)
}

// PushListText is a paid mutator transaction binding the contract method 0x043a728d.
//
// Solidity: function pushListText(bytes32 node, string key, string value) returns()
func (_EntityResolver *EntityResolverTransactor) PushListText(opts *bind.TransactOpts, node [32]byte, key string, value string) (*types.Transaction, error) {
	return _EntityResolver.contract.Transact(opts, "pushListText", node, key, value)
}

// PushListText is a paid mutator transaction binding the contract method 0x043a728d.
//
// Solidity: function pushListText(bytes32 node, string key, string value) returns()
func (_EntityResolver *EntityResolverSession) PushListText(node [32]byte, key string, value string) (*types.Transaction, error) {
	return _EntityResolver.Contract.PushListText(&_EntityResolver.TransactOpts, node, key, value)
}

// PushListText is a paid mutator transaction binding the contract method 0x043a728d.
//
// Solidity: function pushListText(bytes32 node, string key, string value) returns()
func (_EntityResolver *EntityResolverTransactorSession) PushListText(node [32]byte, key string, value string) (*types.Transaction, error) {
	return _EntityResolver.Contract.PushListText(&_EntityResolver.TransactOpts, node, key, value)
}

// RemoveListIndex is a paid mutator transaction binding the contract method 0x6f473720.
//
// Solidity: function removeListIndex(bytes32 node, string key, uint256 index) returns()
func (_EntityResolver *EntityResolverTransactor) RemoveListIndex(opts *bind.TransactOpts, node [32]byte, key string, index *big.Int) (*types.Transaction, error) {
	return _EntityResolver.contract.Transact(opts, "removeListIndex", node, key, index)
}

// RemoveListIndex is a paid mutator transaction binding the contract method 0x6f473720.
//
// Solidity: function removeListIndex(bytes32 node, string key, uint256 index) returns()
func (_EntityResolver *EntityResolverSession) RemoveListIndex(node [32]byte, key string, index *big.Int) (*types.Transaction, error) {
	return _EntityResolver.Contract.RemoveListIndex(&_EntityResolver.TransactOpts, node, key, index)
}

// RemoveListIndex is a paid mutator transaction binding the contract method 0x6f473720.
//
// Solidity: function removeListIndex(bytes32 node, string key, uint256 index) returns()
func (_EntityResolver *EntityResolverTransactorSession) RemoveListIndex(node [32]byte, key string, index *big.Int) (*types.Transaction, error) {
	return _EntityResolver.Contract.RemoveListIndex(&_EntityResolver.TransactOpts, node, key, index)
}

// SetABI is a paid mutator transaction binding the contract method 0x623195b0.
//
// Solidity: function setABI(bytes32 node, uint256 contentType, bytes data) returns()
func (_EntityResolver *EntityResolverTransactor) SetABI(opts *bind.TransactOpts, node [32]byte, contentType *big.Int, data []byte) (*types.Transaction, error) {
	return _EntityResolver.contract.Transact(opts, "setABI", node, contentType, data)
}

// SetABI is a paid mutator transaction binding the contract method 0x623195b0.
//
// Solidity: function setABI(bytes32 node, uint256 contentType, bytes data) returns()
func (_EntityResolver *EntityResolverSession) SetABI(node [32]byte, contentType *big.Int, data []byte) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetABI(&_EntityResolver.TransactOpts, node, contentType, data)
}

// SetABI is a paid mutator transaction binding the contract method 0x623195b0.
//
// Solidity: function setABI(bytes32 node, uint256 contentType, bytes data) returns()
func (_EntityResolver *EntityResolverTransactorSession) SetABI(node [32]byte, contentType *big.Int, data []byte) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetABI(&_EntityResolver.TransactOpts, node, contentType, data)
}

// SetAddr is a paid mutator transaction binding the contract method 0x8b95dd71.
//
// Solidity: function setAddr(bytes32 node, uint256 coinType, bytes a) returns()
func (_EntityResolver *EntityResolverTransactor) SetAddr(opts *bind.TransactOpts, node [32]byte, coinType *big.Int, a []byte) (*types.Transaction, error) {
	return _EntityResolver.contract.Transact(opts, "setAddr", node, coinType, a)
}

// SetAddr is a paid mutator transaction binding the contract method 0x8b95dd71.
//
// Solidity: function setAddr(bytes32 node, uint256 coinType, bytes a) returns()
func (_EntityResolver *EntityResolverSession) SetAddr(node [32]byte, coinType *big.Int, a []byte) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetAddr(&_EntityResolver.TransactOpts, node, coinType, a)
}

// SetAddr is a paid mutator transaction binding the contract method 0x8b95dd71.
//
// Solidity: function setAddr(bytes32 node, uint256 coinType, bytes a) returns()
func (_EntityResolver *EntityResolverTransactorSession) SetAddr(node [32]byte, coinType *big.Int, a []byte) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetAddr(&_EntityResolver.TransactOpts, node, coinType, a)
}

// SetAddr0 is a paid mutator transaction binding the contract method 0xd5fa2b00.
//
// Solidity: function setAddr(bytes32 node, address a) returns()
func (_EntityResolver *EntityResolverTransactor) SetAddr0(opts *bind.TransactOpts, node [32]byte, a common.Address) (*types.Transaction, error) {
	return _EntityResolver.contract.Transact(opts, "setAddr0", node, a)
}

// SetAddr0 is a paid mutator transaction binding the contract method 0xd5fa2b00.
//
// Solidity: function setAddr(bytes32 node, address a) returns()
func (_EntityResolver *EntityResolverSession) SetAddr0(node [32]byte, a common.Address) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetAddr0(&_EntityResolver.TransactOpts, node, a)
}

// SetAddr0 is a paid mutator transaction binding the contract method 0xd5fa2b00.
//
// Solidity: function setAddr(bytes32 node, address a) returns()
func (_EntityResolver *EntityResolverTransactorSession) SetAddr0(node [32]byte, a common.Address) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetAddr0(&_EntityResolver.TransactOpts, node, a)
}

// SetAuthorisation is a paid mutator transaction binding the contract method 0x3e9ce794.
//
// Solidity: function setAuthorisation(bytes32 node, address target, bool isAuthorised) returns()
func (_EntityResolver *EntityResolverTransactor) SetAuthorisation(opts *bind.TransactOpts, node [32]byte, target common.Address, isAuthorised bool) (*types.Transaction, error) {
	return _EntityResolver.contract.Transact(opts, "setAuthorisation", node, target, isAuthorised)
}

// SetAuthorisation is a paid mutator transaction binding the contract method 0x3e9ce794.
//
// Solidity: function setAuthorisation(bytes32 node, address target, bool isAuthorised) returns()
func (_EntityResolver *EntityResolverSession) SetAuthorisation(node [32]byte, target common.Address, isAuthorised bool) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetAuthorisation(&_EntityResolver.TransactOpts, node, target, isAuthorised)
}

// SetAuthorisation is a paid mutator transaction binding the contract method 0x3e9ce794.
//
// Solidity: function setAuthorisation(bytes32 node, address target, bool isAuthorised) returns()
func (_EntityResolver *EntityResolverTransactorSession) SetAuthorisation(node [32]byte, target common.Address, isAuthorised bool) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetAuthorisation(&_EntityResolver.TransactOpts, node, target, isAuthorised)
}

// SetContenthash is a paid mutator transaction binding the contract method 0x304e6ade.
//
// Solidity: function setContenthash(bytes32 node, bytes hash) returns()
func (_EntityResolver *EntityResolverTransactor) SetContenthash(opts *bind.TransactOpts, node [32]byte, hash []byte) (*types.Transaction, error) {
	return _EntityResolver.contract.Transact(opts, "setContenthash", node, hash)
}

// SetContenthash is a paid mutator transaction binding the contract method 0x304e6ade.
//
// Solidity: function setContenthash(bytes32 node, bytes hash) returns()
func (_EntityResolver *EntityResolverSession) SetContenthash(node [32]byte, hash []byte) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetContenthash(&_EntityResolver.TransactOpts, node, hash)
}

// SetContenthash is a paid mutator transaction binding the contract method 0x304e6ade.
//
// Solidity: function setContenthash(bytes32 node, bytes hash) returns()
func (_EntityResolver *EntityResolverTransactorSession) SetContenthash(node [32]byte, hash []byte) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetContenthash(&_EntityResolver.TransactOpts, node, hash)
}

// SetInterface is a paid mutator transaction binding the contract method 0xe59d895d.
//
// Solidity: function setInterface(bytes32 node, bytes4 interfaceID, address implementer) returns()
func (_EntityResolver *EntityResolverTransactor) SetInterface(opts *bind.TransactOpts, node [32]byte, interfaceID [4]byte, implementer common.Address) (*types.Transaction, error) {
	return _EntityResolver.contract.Transact(opts, "setInterface", node, interfaceID, implementer)
}

// SetInterface is a paid mutator transaction binding the contract method 0xe59d895d.
//
// Solidity: function setInterface(bytes32 node, bytes4 interfaceID, address implementer) returns()
func (_EntityResolver *EntityResolverSession) SetInterface(node [32]byte, interfaceID [4]byte, implementer common.Address) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetInterface(&_EntityResolver.TransactOpts, node, interfaceID, implementer)
}

// SetInterface is a paid mutator transaction binding the contract method 0xe59d895d.
//
// Solidity: function setInterface(bytes32 node, bytes4 interfaceID, address implementer) returns()
func (_EntityResolver *EntityResolverTransactorSession) SetInterface(node [32]byte, interfaceID [4]byte, implementer common.Address) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetInterface(&_EntityResolver.TransactOpts, node, interfaceID, implementer)
}

// SetListText is a paid mutator transaction binding the contract method 0xa2df33e1.
//
// Solidity: function setListText(bytes32 node, string key, uint256 index, string value) returns()
func (_EntityResolver *EntityResolverTransactor) SetListText(opts *bind.TransactOpts, node [32]byte, key string, index *big.Int, value string) (*types.Transaction, error) {
	return _EntityResolver.contract.Transact(opts, "setListText", node, key, index, value)
}

// SetListText is a paid mutator transaction binding the contract method 0xa2df33e1.
//
// Solidity: function setListText(bytes32 node, string key, uint256 index, string value) returns()
func (_EntityResolver *EntityResolverSession) SetListText(node [32]byte, key string, index *big.Int, value string) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetListText(&_EntityResolver.TransactOpts, node, key, index, value)
}

// SetListText is a paid mutator transaction binding the contract method 0xa2df33e1.
//
// Solidity: function setListText(bytes32 node, string key, uint256 index, string value) returns()
func (_EntityResolver *EntityResolverTransactorSession) SetListText(node [32]byte, key string, index *big.Int, value string) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetListText(&_EntityResolver.TransactOpts, node, key, index, value)
}

// SetName is a paid mutator transaction binding the contract method 0x77372213.
//
// Solidity: function setName(bytes32 node, string name) returns()
func (_EntityResolver *EntityResolverTransactor) SetName(opts *bind.TransactOpts, node [32]byte, name string) (*types.Transaction, error) {
	return _EntityResolver.contract.Transact(opts, "setName", node, name)
}

// SetName is a paid mutator transaction binding the contract method 0x77372213.
//
// Solidity: function setName(bytes32 node, string name) returns()
func (_EntityResolver *EntityResolverSession) SetName(node [32]byte, name string) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetName(&_EntityResolver.TransactOpts, node, name)
}

// SetName is a paid mutator transaction binding the contract method 0x77372213.
//
// Solidity: function setName(bytes32 node, string name) returns()
func (_EntityResolver *EntityResolverTransactorSession) SetName(node [32]byte, name string) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetName(&_EntityResolver.TransactOpts, node, name)
}

// SetPubkey is a paid mutator transaction binding the contract method 0x29cd62ea.
//
// Solidity: function setPubkey(bytes32 node, bytes32 x, bytes32 y) returns()
func (_EntityResolver *EntityResolverTransactor) SetPubkey(opts *bind.TransactOpts, node [32]byte, x [32]byte, y [32]byte) (*types.Transaction, error) {
	return _EntityResolver.contract.Transact(opts, "setPubkey", node, x, y)
}

// SetPubkey is a paid mutator transaction binding the contract method 0x29cd62ea.
//
// Solidity: function setPubkey(bytes32 node, bytes32 x, bytes32 y) returns()
func (_EntityResolver *EntityResolverSession) SetPubkey(node [32]byte, x [32]byte, y [32]byte) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetPubkey(&_EntityResolver.TransactOpts, node, x, y)
}

// SetPubkey is a paid mutator transaction binding the contract method 0x29cd62ea.
//
// Solidity: function setPubkey(bytes32 node, bytes32 x, bytes32 y) returns()
func (_EntityResolver *EntityResolverTransactorSession) SetPubkey(node [32]byte, x [32]byte, y [32]byte) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetPubkey(&_EntityResolver.TransactOpts, node, x, y)
}

// SetText is a paid mutator transaction binding the contract method 0x10f13a8c.
//
// Solidity: function setText(bytes32 node, string key, string value) returns()
func (_EntityResolver *EntityResolverTransactor) SetText(opts *bind.TransactOpts, node [32]byte, key string, value string) (*types.Transaction, error) {
	return _EntityResolver.contract.Transact(opts, "setText", node, key, value)
}

// SetText is a paid mutator transaction binding the contract method 0x10f13a8c.
//
// Solidity: function setText(bytes32 node, string key, string value) returns()
func (_EntityResolver *EntityResolverSession) SetText(node [32]byte, key string, value string) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetText(&_EntityResolver.TransactOpts, node, key, value)
}

// SetText is a paid mutator transaction binding the contract method 0x10f13a8c.
//
// Solidity: function setText(bytes32 node, string key, string value) returns()
func (_EntityResolver *EntityResolverTransactorSession) SetText(node [32]byte, key string, value string) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetText(&_EntityResolver.TransactOpts, node, key, value)
}

// EntityResolverABIChangedIterator is returned from FilterABIChanged and is used to iterate over the raw logs and unpacked data for ABIChanged events raised by the EntityResolver contract.
type EntityResolverABIChangedIterator struct {
	Event *EntityResolverABIChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EntityResolverABIChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EntityResolverABIChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EntityResolverABIChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EntityResolverABIChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EntityResolverABIChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EntityResolverABIChanged represents a ABIChanged event raised by the EntityResolver contract.
type EntityResolverABIChanged struct {
	Node        [32]byte
	ContentType *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterABIChanged is a free log retrieval operation binding the contract event 0xaa121bbeef5f32f5961a2a28966e769023910fc9479059ee3495d4c1a696efe3.
//
// Solidity: event ABIChanged(bytes32 indexed node, uint256 indexed contentType)
func (_EntityResolver *EntityResolverFilterer) FilterABIChanged(opts *bind.FilterOpts, node [][32]byte, contentType []*big.Int) (*EntityResolverABIChangedIterator, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}
	var contentTypeRule []interface{}
	for _, contentTypeItem := range contentType {
		contentTypeRule = append(contentTypeRule, contentTypeItem)
	}

	logs, sub, err := _EntityResolver.contract.FilterLogs(opts, "ABIChanged", nodeRule, contentTypeRule)
	if err != nil {
		return nil, err
	}
	return &EntityResolverABIChangedIterator{contract: _EntityResolver.contract, event: "ABIChanged", logs: logs, sub: sub}, nil
}

// WatchABIChanged is a free log subscription operation binding the contract event 0xaa121bbeef5f32f5961a2a28966e769023910fc9479059ee3495d4c1a696efe3.
//
// Solidity: event ABIChanged(bytes32 indexed node, uint256 indexed contentType)
func (_EntityResolver *EntityResolverFilterer) WatchABIChanged(opts *bind.WatchOpts, sink chan<- *EntityResolverABIChanged, node [][32]byte, contentType []*big.Int) (event.Subscription, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}
	var contentTypeRule []interface{}
	for _, contentTypeItem := range contentType {
		contentTypeRule = append(contentTypeRule, contentTypeItem)
	}

	logs, sub, err := _EntityResolver.contract.WatchLogs(opts, "ABIChanged", nodeRule, contentTypeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EntityResolverABIChanged)
				if err := _EntityResolver.contract.UnpackLog(event, "ABIChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseABIChanged is a log parse operation binding the contract event 0xaa121bbeef5f32f5961a2a28966e769023910fc9479059ee3495d4c1a696efe3.
//
// Solidity: event ABIChanged(bytes32 indexed node, uint256 indexed contentType)
func (_EntityResolver *EntityResolverFilterer) ParseABIChanged(log types.Log) (*EntityResolverABIChanged, error) {
	event := new(EntityResolverABIChanged)
	if err := _EntityResolver.contract.UnpackLog(event, "ABIChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EntityResolverAddrChangedIterator is returned from FilterAddrChanged and is used to iterate over the raw logs and unpacked data for AddrChanged events raised by the EntityResolver contract.
type EntityResolverAddrChangedIterator struct {
	Event *EntityResolverAddrChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EntityResolverAddrChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EntityResolverAddrChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EntityResolverAddrChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EntityResolverAddrChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EntityResolverAddrChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EntityResolverAddrChanged represents a AddrChanged event raised by the EntityResolver contract.
type EntityResolverAddrChanged struct {
	Node [32]byte
	A    common.Address
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterAddrChanged is a free log retrieval operation binding the contract event 0x52d7d861f09ab3d26239d492e8968629f95e9e318cf0b73bfddc441522a15fd2.
//
// Solidity: event AddrChanged(bytes32 indexed node, address a)
func (_EntityResolver *EntityResolverFilterer) FilterAddrChanged(opts *bind.FilterOpts, node [][32]byte) (*EntityResolverAddrChangedIterator, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.FilterLogs(opts, "AddrChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return &EntityResolverAddrChangedIterator{contract: _EntityResolver.contract, event: "AddrChanged", logs: logs, sub: sub}, nil
}

// WatchAddrChanged is a free log subscription operation binding the contract event 0x52d7d861f09ab3d26239d492e8968629f95e9e318cf0b73bfddc441522a15fd2.
//
// Solidity: event AddrChanged(bytes32 indexed node, address a)
func (_EntityResolver *EntityResolverFilterer) WatchAddrChanged(opts *bind.WatchOpts, sink chan<- *EntityResolverAddrChanged, node [][32]byte) (event.Subscription, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.WatchLogs(opts, "AddrChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EntityResolverAddrChanged)
				if err := _EntityResolver.contract.UnpackLog(event, "AddrChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAddrChanged is a log parse operation binding the contract event 0x52d7d861f09ab3d26239d492e8968629f95e9e318cf0b73bfddc441522a15fd2.
//
// Solidity: event AddrChanged(bytes32 indexed node, address a)
func (_EntityResolver *EntityResolverFilterer) ParseAddrChanged(log types.Log) (*EntityResolverAddrChanged, error) {
	event := new(EntityResolverAddrChanged)
	if err := _EntityResolver.contract.UnpackLog(event, "AddrChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EntityResolverAddressChangedIterator is returned from FilterAddressChanged and is used to iterate over the raw logs and unpacked data for AddressChanged events raised by the EntityResolver contract.
type EntityResolverAddressChangedIterator struct {
	Event *EntityResolverAddressChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EntityResolverAddressChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EntityResolverAddressChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EntityResolverAddressChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EntityResolverAddressChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EntityResolverAddressChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EntityResolverAddressChanged represents a AddressChanged event raised by the EntityResolver contract.
type EntityResolverAddressChanged struct {
	Node       [32]byte
	CoinType   *big.Int
	NewAddress []byte
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterAddressChanged is a free log retrieval operation binding the contract event 0x65412581168e88a1e60c6459d7f44ae83ad0832e670826c05a4e2476b57af752.
//
// Solidity: event AddressChanged(bytes32 indexed node, uint256 coinType, bytes newAddress)
func (_EntityResolver *EntityResolverFilterer) FilterAddressChanged(opts *bind.FilterOpts, node [][32]byte) (*EntityResolverAddressChangedIterator, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.FilterLogs(opts, "AddressChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return &EntityResolverAddressChangedIterator{contract: _EntityResolver.contract, event: "AddressChanged", logs: logs, sub: sub}, nil
}

// WatchAddressChanged is a free log subscription operation binding the contract event 0x65412581168e88a1e60c6459d7f44ae83ad0832e670826c05a4e2476b57af752.
//
// Solidity: event AddressChanged(bytes32 indexed node, uint256 coinType, bytes newAddress)
func (_EntityResolver *EntityResolverFilterer) WatchAddressChanged(opts *bind.WatchOpts, sink chan<- *EntityResolverAddressChanged, node [][32]byte) (event.Subscription, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.WatchLogs(opts, "AddressChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EntityResolverAddressChanged)
				if err := _EntityResolver.contract.UnpackLog(event, "AddressChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAddressChanged is a log parse operation binding the contract event 0x65412581168e88a1e60c6459d7f44ae83ad0832e670826c05a4e2476b57af752.
//
// Solidity: event AddressChanged(bytes32 indexed node, uint256 coinType, bytes newAddress)
func (_EntityResolver *EntityResolverFilterer) ParseAddressChanged(log types.Log) (*EntityResolverAddressChanged, error) {
	event := new(EntityResolverAddressChanged)
	if err := _EntityResolver.contract.UnpackLog(event, "AddressChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EntityResolverAuthorisationChangedIterator is returned from FilterAuthorisationChanged and is used to iterate over the raw logs and unpacked data for AuthorisationChanged events raised by the EntityResolver contract.
type EntityResolverAuthorisationChangedIterator struct {
	Event *EntityResolverAuthorisationChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EntityResolverAuthorisationChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EntityResolverAuthorisationChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EntityResolverAuthorisationChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EntityResolverAuthorisationChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EntityResolverAuthorisationChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EntityResolverAuthorisationChanged represents a AuthorisationChanged event raised by the EntityResolver contract.
type EntityResolverAuthorisationChanged struct {
	Node         [32]byte
	Owner        common.Address
	Target       common.Address
	IsAuthorised bool
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterAuthorisationChanged is a free log retrieval operation binding the contract event 0xe1c5610a6e0cbe10764ecd182adcef1ec338dc4e199c99c32ce98f38e12791df.
//
// Solidity: event AuthorisationChanged(bytes32 indexed node, address indexed owner, address indexed target, bool isAuthorised)
func (_EntityResolver *EntityResolverFilterer) FilterAuthorisationChanged(opts *bind.FilterOpts, node [][32]byte, owner []common.Address, target []common.Address) (*EntityResolverAuthorisationChangedIterator, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}
	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var targetRule []interface{}
	for _, targetItem := range target {
		targetRule = append(targetRule, targetItem)
	}

	logs, sub, err := _EntityResolver.contract.FilterLogs(opts, "AuthorisationChanged", nodeRule, ownerRule, targetRule)
	if err != nil {
		return nil, err
	}
	return &EntityResolverAuthorisationChangedIterator{contract: _EntityResolver.contract, event: "AuthorisationChanged", logs: logs, sub: sub}, nil
}

// WatchAuthorisationChanged is a free log subscription operation binding the contract event 0xe1c5610a6e0cbe10764ecd182adcef1ec338dc4e199c99c32ce98f38e12791df.
//
// Solidity: event AuthorisationChanged(bytes32 indexed node, address indexed owner, address indexed target, bool isAuthorised)
func (_EntityResolver *EntityResolverFilterer) WatchAuthorisationChanged(opts *bind.WatchOpts, sink chan<- *EntityResolverAuthorisationChanged, node [][32]byte, owner []common.Address, target []common.Address) (event.Subscription, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}
	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var targetRule []interface{}
	for _, targetItem := range target {
		targetRule = append(targetRule, targetItem)
	}

	logs, sub, err := _EntityResolver.contract.WatchLogs(opts, "AuthorisationChanged", nodeRule, ownerRule, targetRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EntityResolverAuthorisationChanged)
				if err := _EntityResolver.contract.UnpackLog(event, "AuthorisationChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAuthorisationChanged is a log parse operation binding the contract event 0xe1c5610a6e0cbe10764ecd182adcef1ec338dc4e199c99c32ce98f38e12791df.
//
// Solidity: event AuthorisationChanged(bytes32 indexed node, address indexed owner, address indexed target, bool isAuthorised)
func (_EntityResolver *EntityResolverFilterer) ParseAuthorisationChanged(log types.Log) (*EntityResolverAuthorisationChanged, error) {
	event := new(EntityResolverAuthorisationChanged)
	if err := _EntityResolver.contract.UnpackLog(event, "AuthorisationChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EntityResolverContenthashChangedIterator is returned from FilterContenthashChanged and is used to iterate over the raw logs and unpacked data for ContenthashChanged events raised by the EntityResolver contract.
type EntityResolverContenthashChangedIterator struct {
	Event *EntityResolverContenthashChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EntityResolverContenthashChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EntityResolverContenthashChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EntityResolverContenthashChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EntityResolverContenthashChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EntityResolverContenthashChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EntityResolverContenthashChanged represents a ContenthashChanged event raised by the EntityResolver contract.
type EntityResolverContenthashChanged struct {
	Node [32]byte
	Hash []byte
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterContenthashChanged is a free log retrieval operation binding the contract event 0xe379c1624ed7e714cc0937528a32359d69d5281337765313dba4e081b72d7578.
//
// Solidity: event ContenthashChanged(bytes32 indexed node, bytes hash)
func (_EntityResolver *EntityResolverFilterer) FilterContenthashChanged(opts *bind.FilterOpts, node [][32]byte) (*EntityResolverContenthashChangedIterator, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.FilterLogs(opts, "ContenthashChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return &EntityResolverContenthashChangedIterator{contract: _EntityResolver.contract, event: "ContenthashChanged", logs: logs, sub: sub}, nil
}

// WatchContenthashChanged is a free log subscription operation binding the contract event 0xe379c1624ed7e714cc0937528a32359d69d5281337765313dba4e081b72d7578.
//
// Solidity: event ContenthashChanged(bytes32 indexed node, bytes hash)
func (_EntityResolver *EntityResolverFilterer) WatchContenthashChanged(opts *bind.WatchOpts, sink chan<- *EntityResolverContenthashChanged, node [][32]byte) (event.Subscription, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.WatchLogs(opts, "ContenthashChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EntityResolverContenthashChanged)
				if err := _EntityResolver.contract.UnpackLog(event, "ContenthashChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseContenthashChanged is a log parse operation binding the contract event 0xe379c1624ed7e714cc0937528a32359d69d5281337765313dba4e081b72d7578.
//
// Solidity: event ContenthashChanged(bytes32 indexed node, bytes hash)
func (_EntityResolver *EntityResolverFilterer) ParseContenthashChanged(log types.Log) (*EntityResolverContenthashChanged, error) {
	event := new(EntityResolverContenthashChanged)
	if err := _EntityResolver.contract.UnpackLog(event, "ContenthashChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EntityResolverInterfaceChangedIterator is returned from FilterInterfaceChanged and is used to iterate over the raw logs and unpacked data for InterfaceChanged events raised by the EntityResolver contract.
type EntityResolverInterfaceChangedIterator struct {
	Event *EntityResolverInterfaceChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EntityResolverInterfaceChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EntityResolverInterfaceChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EntityResolverInterfaceChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EntityResolverInterfaceChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EntityResolverInterfaceChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EntityResolverInterfaceChanged represents a InterfaceChanged event raised by the EntityResolver contract.
type EntityResolverInterfaceChanged struct {
	Node        [32]byte
	InterfaceID [4]byte
	Implementer common.Address
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterInterfaceChanged is a free log retrieval operation binding the contract event 0x7c69f06bea0bdef565b709e93a147836b0063ba2dd89f02d0b7e8d931e6a6daa.
//
// Solidity: event InterfaceChanged(bytes32 indexed node, bytes4 indexed interfaceID, address implementer)
func (_EntityResolver *EntityResolverFilterer) FilterInterfaceChanged(opts *bind.FilterOpts, node [][32]byte, interfaceID [][4]byte) (*EntityResolverInterfaceChangedIterator, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}
	var interfaceIDRule []interface{}
	for _, interfaceIDItem := range interfaceID {
		interfaceIDRule = append(interfaceIDRule, interfaceIDItem)
	}

	logs, sub, err := _EntityResolver.contract.FilterLogs(opts, "InterfaceChanged", nodeRule, interfaceIDRule)
	if err != nil {
		return nil, err
	}
	return &EntityResolverInterfaceChangedIterator{contract: _EntityResolver.contract, event: "InterfaceChanged", logs: logs, sub: sub}, nil
}

// WatchInterfaceChanged is a free log subscription operation binding the contract event 0x7c69f06bea0bdef565b709e93a147836b0063ba2dd89f02d0b7e8d931e6a6daa.
//
// Solidity: event InterfaceChanged(bytes32 indexed node, bytes4 indexed interfaceID, address implementer)
func (_EntityResolver *EntityResolverFilterer) WatchInterfaceChanged(opts *bind.WatchOpts, sink chan<- *EntityResolverInterfaceChanged, node [][32]byte, interfaceID [][4]byte) (event.Subscription, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}
	var interfaceIDRule []interface{}
	for _, interfaceIDItem := range interfaceID {
		interfaceIDRule = append(interfaceIDRule, interfaceIDItem)
	}

	logs, sub, err := _EntityResolver.contract.WatchLogs(opts, "InterfaceChanged", nodeRule, interfaceIDRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EntityResolverInterfaceChanged)
				if err := _EntityResolver.contract.UnpackLog(event, "InterfaceChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseInterfaceChanged is a log parse operation binding the contract event 0x7c69f06bea0bdef565b709e93a147836b0063ba2dd89f02d0b7e8d931e6a6daa.
//
// Solidity: event InterfaceChanged(bytes32 indexed node, bytes4 indexed interfaceID, address implementer)
func (_EntityResolver *EntityResolverFilterer) ParseInterfaceChanged(log types.Log) (*EntityResolverInterfaceChanged, error) {
	event := new(EntityResolverInterfaceChanged)
	if err := _EntityResolver.contract.UnpackLog(event, "InterfaceChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EntityResolverListItemChangedIterator is returned from FilterListItemChanged and is used to iterate over the raw logs and unpacked data for ListItemChanged events raised by the EntityResolver contract.
type EntityResolverListItemChangedIterator struct {
	Event *EntityResolverListItemChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EntityResolverListItemChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EntityResolverListItemChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EntityResolverListItemChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EntityResolverListItemChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EntityResolverListItemChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EntityResolverListItemChanged represents a ListItemChanged event raised by the EntityResolver contract.
type EntityResolverListItemChanged struct {
	Node  [32]byte
	Key   string
	Index *big.Int
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterListItemChanged is a free log retrieval operation binding the contract event 0xad6a325380d85c50dfaa38511cbbbc97c46b9a52e342c8d705bf5515d0ef45b3.
//
// Solidity: event ListItemChanged(bytes32 indexed node, string key, uint256 index)
func (_EntityResolver *EntityResolverFilterer) FilterListItemChanged(opts *bind.FilterOpts, node [][32]byte) (*EntityResolverListItemChangedIterator, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.FilterLogs(opts, "ListItemChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return &EntityResolverListItemChangedIterator{contract: _EntityResolver.contract, event: "ListItemChanged", logs: logs, sub: sub}, nil
}

// WatchListItemChanged is a free log subscription operation binding the contract event 0xad6a325380d85c50dfaa38511cbbbc97c46b9a52e342c8d705bf5515d0ef45b3.
//
// Solidity: event ListItemChanged(bytes32 indexed node, string key, uint256 index)
func (_EntityResolver *EntityResolverFilterer) WatchListItemChanged(opts *bind.WatchOpts, sink chan<- *EntityResolverListItemChanged, node [][32]byte) (event.Subscription, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.WatchLogs(opts, "ListItemChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EntityResolverListItemChanged)
				if err := _EntityResolver.contract.UnpackLog(event, "ListItemChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseListItemChanged is a log parse operation binding the contract event 0xad6a325380d85c50dfaa38511cbbbc97c46b9a52e342c8d705bf5515d0ef45b3.
//
// Solidity: event ListItemChanged(bytes32 indexed node, string key, uint256 index)
func (_EntityResolver *EntityResolverFilterer) ParseListItemChanged(log types.Log) (*EntityResolverListItemChanged, error) {
	event := new(EntityResolverListItemChanged)
	if err := _EntityResolver.contract.UnpackLog(event, "ListItemChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EntityResolverListItemRemovedIterator is returned from FilterListItemRemoved and is used to iterate over the raw logs and unpacked data for ListItemRemoved events raised by the EntityResolver contract.
type EntityResolverListItemRemovedIterator struct {
	Event *EntityResolverListItemRemoved // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EntityResolverListItemRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EntityResolverListItemRemoved)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EntityResolverListItemRemoved)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EntityResolverListItemRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EntityResolverListItemRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EntityResolverListItemRemoved represents a ListItemRemoved event raised by the EntityResolver contract.
type EntityResolverListItemRemoved struct {
	Node  [32]byte
	Key   string
	Index *big.Int
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterListItemRemoved is a free log retrieval operation binding the contract event 0x5dbbe7a1e616b629a61480b13fd4e89dfe8b604d35802cdf7b0bb03687e3eb49.
//
// Solidity: event ListItemRemoved(bytes32 indexed node, string key, uint256 index)
func (_EntityResolver *EntityResolverFilterer) FilterListItemRemoved(opts *bind.FilterOpts, node [][32]byte) (*EntityResolverListItemRemovedIterator, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.FilterLogs(opts, "ListItemRemoved", nodeRule)
	if err != nil {
		return nil, err
	}
	return &EntityResolverListItemRemovedIterator{contract: _EntityResolver.contract, event: "ListItemRemoved", logs: logs, sub: sub}, nil
}

// WatchListItemRemoved is a free log subscription operation binding the contract event 0x5dbbe7a1e616b629a61480b13fd4e89dfe8b604d35802cdf7b0bb03687e3eb49.
//
// Solidity: event ListItemRemoved(bytes32 indexed node, string key, uint256 index)
func (_EntityResolver *EntityResolverFilterer) WatchListItemRemoved(opts *bind.WatchOpts, sink chan<- *EntityResolverListItemRemoved, node [][32]byte) (event.Subscription, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.WatchLogs(opts, "ListItemRemoved", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EntityResolverListItemRemoved)
				if err := _EntityResolver.contract.UnpackLog(event, "ListItemRemoved", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseListItemRemoved is a log parse operation binding the contract event 0x5dbbe7a1e616b629a61480b13fd4e89dfe8b604d35802cdf7b0bb03687e3eb49.
//
// Solidity: event ListItemRemoved(bytes32 indexed node, string key, uint256 index)
func (_EntityResolver *EntityResolverFilterer) ParseListItemRemoved(log types.Log) (*EntityResolverListItemRemoved, error) {
	event := new(EntityResolverListItemRemoved)
	if err := _EntityResolver.contract.UnpackLog(event, "ListItemRemoved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EntityResolverNameChangedIterator is returned from FilterNameChanged and is used to iterate over the raw logs and unpacked data for NameChanged events raised by the EntityResolver contract.
type EntityResolverNameChangedIterator struct {
	Event *EntityResolverNameChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EntityResolverNameChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EntityResolverNameChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EntityResolverNameChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EntityResolverNameChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EntityResolverNameChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EntityResolverNameChanged represents a NameChanged event raised by the EntityResolver contract.
type EntityResolverNameChanged struct {
	Node [32]byte
	Name string
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterNameChanged is a free log retrieval operation binding the contract event 0xb7d29e911041e8d9b843369e890bcb72c9388692ba48b65ac54e7214c4c348f7.
//
// Solidity: event NameChanged(bytes32 indexed node, string name)
func (_EntityResolver *EntityResolverFilterer) FilterNameChanged(opts *bind.FilterOpts, node [][32]byte) (*EntityResolverNameChangedIterator, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.FilterLogs(opts, "NameChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return &EntityResolverNameChangedIterator{contract: _EntityResolver.contract, event: "NameChanged", logs: logs, sub: sub}, nil
}

// WatchNameChanged is a free log subscription operation binding the contract event 0xb7d29e911041e8d9b843369e890bcb72c9388692ba48b65ac54e7214c4c348f7.
//
// Solidity: event NameChanged(bytes32 indexed node, string name)
func (_EntityResolver *EntityResolverFilterer) WatchNameChanged(opts *bind.WatchOpts, sink chan<- *EntityResolverNameChanged, node [][32]byte) (event.Subscription, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.WatchLogs(opts, "NameChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EntityResolverNameChanged)
				if err := _EntityResolver.contract.UnpackLog(event, "NameChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseNameChanged is a log parse operation binding the contract event 0xb7d29e911041e8d9b843369e890bcb72c9388692ba48b65ac54e7214c4c348f7.
//
// Solidity: event NameChanged(bytes32 indexed node, string name)
func (_EntityResolver *EntityResolverFilterer) ParseNameChanged(log types.Log) (*EntityResolverNameChanged, error) {
	event := new(EntityResolverNameChanged)
	if err := _EntityResolver.contract.UnpackLog(event, "NameChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EntityResolverPubkeyChangedIterator is returned from FilterPubkeyChanged and is used to iterate over the raw logs and unpacked data for PubkeyChanged events raised by the EntityResolver contract.
type EntityResolverPubkeyChangedIterator struct {
	Event *EntityResolverPubkeyChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EntityResolverPubkeyChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EntityResolverPubkeyChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EntityResolverPubkeyChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EntityResolverPubkeyChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EntityResolverPubkeyChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EntityResolverPubkeyChanged represents a PubkeyChanged event raised by the EntityResolver contract.
type EntityResolverPubkeyChanged struct {
	Node [32]byte
	X    [32]byte
	Y    [32]byte
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterPubkeyChanged is a free log retrieval operation binding the contract event 0x1d6f5e03d3f63eb58751986629a5439baee5079ff04f345becb66e23eb154e46.
//
// Solidity: event PubkeyChanged(bytes32 indexed node, bytes32 x, bytes32 y)
func (_EntityResolver *EntityResolverFilterer) FilterPubkeyChanged(opts *bind.FilterOpts, node [][32]byte) (*EntityResolverPubkeyChangedIterator, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.FilterLogs(opts, "PubkeyChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return &EntityResolverPubkeyChangedIterator{contract: _EntityResolver.contract, event: "PubkeyChanged", logs: logs, sub: sub}, nil
}

// WatchPubkeyChanged is a free log subscription operation binding the contract event 0x1d6f5e03d3f63eb58751986629a5439baee5079ff04f345becb66e23eb154e46.
//
// Solidity: event PubkeyChanged(bytes32 indexed node, bytes32 x, bytes32 y)
func (_EntityResolver *EntityResolverFilterer) WatchPubkeyChanged(opts *bind.WatchOpts, sink chan<- *EntityResolverPubkeyChanged, node [][32]byte) (event.Subscription, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.WatchLogs(opts, "PubkeyChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EntityResolverPubkeyChanged)
				if err := _EntityResolver.contract.UnpackLog(event, "PubkeyChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParsePubkeyChanged is a log parse operation binding the contract event 0x1d6f5e03d3f63eb58751986629a5439baee5079ff04f345becb66e23eb154e46.
//
// Solidity: event PubkeyChanged(bytes32 indexed node, bytes32 x, bytes32 y)
func (_EntityResolver *EntityResolverFilterer) ParsePubkeyChanged(log types.Log) (*EntityResolverPubkeyChanged, error) {
	event := new(EntityResolverPubkeyChanged)
	if err := _EntityResolver.contract.UnpackLog(event, "PubkeyChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EntityResolverTextChangedIterator is returned from FilterTextChanged and is used to iterate over the raw logs and unpacked data for TextChanged events raised by the EntityResolver contract.
type EntityResolverTextChangedIterator struct {
	Event *EntityResolverTextChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EntityResolverTextChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EntityResolverTextChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EntityResolverTextChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EntityResolverTextChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EntityResolverTextChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EntityResolverTextChanged represents a TextChanged event raised by the EntityResolver contract.
type EntityResolverTextChanged struct {
	Node       [32]byte
	IndexedKey common.Hash
	Key        string
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterTextChanged is a free log retrieval operation binding the contract event 0xd8c9334b1a9c2f9da342a0a2b32629c1a229b6445dad78947f674b44444a7550.
//
// Solidity: event TextChanged(bytes32 indexed node, string indexed indexedKey, string key)
func (_EntityResolver *EntityResolverFilterer) FilterTextChanged(opts *bind.FilterOpts, node [][32]byte, indexedKey []string) (*EntityResolverTextChangedIterator, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}
	var indexedKeyRule []interface{}
	for _, indexedKeyItem := range indexedKey {
		indexedKeyRule = append(indexedKeyRule, indexedKeyItem)
	}

	logs, sub, err := _EntityResolver.contract.FilterLogs(opts, "TextChanged", nodeRule, indexedKeyRule)
	if err != nil {
		return nil, err
	}
	return &EntityResolverTextChangedIterator{contract: _EntityResolver.contract, event: "TextChanged", logs: logs, sub: sub}, nil
}

// WatchTextChanged is a free log subscription operation binding the contract event 0xd8c9334b1a9c2f9da342a0a2b32629c1a229b6445dad78947f674b44444a7550.
//
// Solidity: event TextChanged(bytes32 indexed node, string indexed indexedKey, string key)
func (_EntityResolver *EntityResolverFilterer) WatchTextChanged(opts *bind.WatchOpts, sink chan<- *EntityResolverTextChanged, node [][32]byte, indexedKey []string) (event.Subscription, error) {
	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}
	var indexedKeyRule []interface{}
	for _, indexedKeyItem := range indexedKey {
		indexedKeyRule = append(indexedKeyRule, indexedKeyItem)
	}

	logs, sub, err := _EntityResolver.contract.WatchLogs(opts, "TextChanged", nodeRule, indexedKeyRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EntityResolverTextChanged)
				if err := _EntityResolver.contract.UnpackLog(event, "TextChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseTextChanged is a log parse operation binding the contract event 0xd8c9334b1a9c2f9da342a0a2b32629c1a229b6445dad78947f674b44444a7550.
//
// Solidity: event TextChanged(bytes32 indexed node, string indexed indexedKey, string key)
func (_EntityResolver *EntityResolverFilterer) ParseTextChanged(log types.Log) (*EntityResolverTextChanged, error) {
	event := new(EntityResolverTextChanged)
	if err := _EntityResolver.contract.UnpackLog(event, "TextChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
