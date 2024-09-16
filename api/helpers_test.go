package api

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestAPIHelpers_encodeEVMResultsArgs(t *testing.T) {
	type args struct {
		electionId            common.Hash
		organizationId        common.Address
		censusRoot            common.Hash
		sourceContractAddress common.Address
		results               [][]*types.BigInt
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "encodeEVMResultsArgs0",
			args: args{
				electionId:            common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001"),
				organizationId:        common.HexToAddress("0x0000000000000000000000000000000000000001"),
				censusRoot:            common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001"),
				sourceContractAddress: common.HexToAddress("0x0000000000000000000000000000000000000001"),
				results: [][]*types.BigInt{
					{new(types.BigInt).SetUint64(1), new(types.BigInt).SetUint64(2), new(types.BigInt).SetUint64(3)},
					{new(types.BigInt).SetUint64(4), new(types.BigInt).SetUint64(5), new(types.BigInt).SetUint64(6)},
				},
			},
			want:    "0x000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000c000000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000050000000000000000000000000000000000000000000000000000000000000006",
			wantErr: false,
		},
		{
			name: "encodeEVMResultsArgs1",
			args: args{
				electionId:            common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001"),
				organizationId:        common.HexToAddress("0x326C977E6efc84E512bB9C30f76E30c160eD06FB"),
				censusRoot:            common.HexToHash("0xff00000000000000000000000000000000000000000000000000000000000002"),
				sourceContractAddress: common.HexToAddress("0xCC79157eb46F5624204f47AB42b3906cAA40eaB7"),
				results: [][]*types.BigInt{
					{new(types.BigInt).SetUint64(0), new(types.BigInt).SetUint64(0), new(types.BigInt).SetUint64(0)},
				},
			},
			want:    "0x0000000000000000000000000000000000000000000000000000000000000001000000000000000000000000326c977e6efc84e512bb9c30f76e30c160ed06fbff00000000000000000000000000000000000000000000000000000000000002000000000000000000000000cc79157eb46f5624204f47ab42b3906caa40eab700000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := encodeEVMResultsArgs(tt.args.electionId, tt.args.organizationId, tt.args.censusRoot, tt.args.sourceContractAddress, tt.args.results)
			if (err != nil) != tt.wantErr {
				t.Errorf("encodeEVMResultsArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("encodeEVMResultsArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

var snakeCaseJSON = `
{
	"header": {
		"version": {
			"block": 11,
			"app": 0
		}
	},
	"data": {
		"txs": []
	},
	"last_commit": {
		"signatures": [
			{
				"block_id_flag": 2,
				"validator_address": "2DECD25EBDD6E3FAB2F06AC0EE391C16C292DBAD",
				"timestamp": "2022-12-14T16:40:47.963888306Z",
				"signature": "ayj6CnGcD1zImfCiSIHNXaAujx4uxZdQ/NgWU/yxQJz/GJAMGIyD3C704cGtx3e2zJaXYDtZvwj5C+/Q/v2FBw=="
			},
			{
				"block_id_flag": 2,
				"validator_address": "3C6FF3D424901733818B954AA3AB3BC2E3695332",
				"timestamp": "2022-12-14T16:40:47.972896402Z",
				"signature": "QVZjYpjyVNjbLWaP552MyU3pRZ4Lw8FF98tsGKO7DAUN/QanEf6QJwK7maCesvgfeISG34tYWDKL/p+fUjYFAQ=="
			}
		]
	}
}`[1:]

var camelCaseJSON = `
{
	"data": {
		"txs": []
	},
	"header": {
		"version": {
			"app": 0,
			"block": 11
		}
	},
	"lastCommit": {
		"signatures": [
			{
				"blockIdFlag": 2,
				"signature": "ayj6CnGcD1zImfCiSIHNXaAujx4uxZdQ/NgWU/yxQJz/GJAMGIyD3C704cGtx3e2zJaXYDtZvwj5C+/Q/v2FBw==",
				"timestamp": "2022-12-14T16:40:47.963888306Z",
				"validatorAddress": "2DECD25EBDD6E3FAB2F06AC0EE391C16C292DBAD"
			},
			{
				"blockIdFlag": 2,
				"signature": "QVZjYpjyVNjbLWaP552MyU3pRZ4Lw8FF98tsGKO7DAUN/QanEf6QJwK7maCesvgfeISG34tYWDKL/p+fUjYFAQ==",
				"timestamp": "2022-12-14T16:40:47.972896402Z",
				"validatorAddress": "3C6FF3D424901733818B954AA3AB3BC2E3695332"
			}
		]
	}
}`[1:]

func TestConvertKeysToCamel(t *testing.T) {
	want := camelCaseJSON
	indent := func(data []byte) string {
		var buf bytes.Buffer
		if err := json.Indent(&buf, data, "", "\t"); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}

	// From snake case to camel case.
	got := indent(convertKeysToCamel([]byte(snakeCaseJSON)))
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatal(diff)
	}

	// Camel case should come out the same.
	got = indent(convertKeysToCamel([]byte(camelCaseJSON)))
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatal(diff)
	}
}

func TestProtoTxAsJSON(t *testing.T) {
	inputJSON := strings.TrimSpace(`
{
	"setProcess": {
		"txtype": "SET_PROCESS_CENSUS",
		"nonce": 1,
		"processId": "sx3/YYFNq5DWw6m2XWyQgwSA5Lda0y50eUICAAAAAAA=",
		"censusRoot": "zUU9BcTLBCnuXuGu/tAW9VO4AmtM7VsMNSkFv6U8foE=",
		"censusURI": "ipfs://bafybeicyfukarcryrvy5oe37ligmxwf55sbfiojori4t25wencma4ymxfa",
		"censusSize": "1000"
	}
}
`)
	wantJSON := strings.TrimSpace(`
{
	"setProcess": {
		"txtype": "SET_PROCESS_CENSUS",
		"nonce": 1,
		"processId": "b31dff61814dab90d6c3a9b65d6c90830480e4b75ad32e747942020000000000",
		"censusRoot": "cd453d05c4cb0429ee5ee1aefed016f553b8026b4ced5b0c352905bfa53c7e81",
		"censusURI": "ipfs://bafybeicyfukarcryrvy5oe37ligmxwf55sbfiojori4t25wencma4ymxfa",
		"censusSize": "1000"
	}
}
`)
	var ptx models.Tx
	err := protojson.Unmarshal([]byte(inputJSON), &ptx)
	qt.Assert(t, err, qt.IsNil)

	asProto, err := proto.Marshal(&ptx)
	qt.Assert(t, err, qt.IsNil)

	var dst bytes.Buffer
	err = json.Indent(&dst, protoTxAsJSON(asProto), "", "\t")
	qt.Assert(t, err, qt.IsNil)
	gotJSON := dst.String()

	qt.Assert(t, gotJSON, qt.Equals, wantJSON)
}
