package api

import (
	"bytes"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/go-cmp/cmp"
)

func TestAPIHelpers_encodeEVMResultsArgs(t *testing.T) {
	type args struct {
		electionId            common.Hash
		organizationId        common.Address
		censusRoot            common.Hash
		sourceContractAddress common.Address
		results               [][]*big.Int
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
				results: [][]*big.Int{
					{big.NewInt(1), big.NewInt(2), big.NewInt(3)},
					{big.NewInt(4), big.NewInt(5), big.NewInt(6)},
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
				results: [][]*big.Int{
					{big.NewInt(0), big.NewInt(0), big.NewInt(0)},
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

// TODO: txs is turned from an array into an object!
var camelCaseJSON = `
{
	"data": {
		"txs": {}
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
