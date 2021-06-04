# arbo [![GoDoc](https://godoc.org/github.com/arnaucube/arbo?status.svg)](https://godoc.org/github.com/arnaucube/arbo) [![Go Report Card](https://goreportcard.com/badge/github.com/arnaucube/arbo)](https://goreportcard.com/report/github.com/arnaucube/arbo) [![Test](https://github.com/arnaucube/arbo/workflows/Test/badge.svg)](https://github.com/arnaucube/arbo/actions?query=workflow%3ATest)

> *arbo*: tree in Esperanto.

MerkleTree implementation in Go. Compatible with the circomlib implementation of
the MerkleTree, following the specification from
https://docs.iden3.io/publications/pdfs/Merkle-Tree.pdf and
https://eprint.iacr.org/2018/955.

Allows to define which hash function to use. So for example, when working with
zkSnarks the Poseidon hash function can be used, but when not, it can be used
the Blake2b hash function, which has much faster computation time.

## Usage

```go
// create new database
database, err := db.NewBadgerDB(c.TempDir())

// create new Tree with maxLevels=100 and Blake2b hash function
tree, err := arbo.NewTree(database, 100, arbo.HashFunctionBlake2b)


key := []byte("hello")
value := []byte("world")
err = tree.Add(key, value)


// There are cases where multiple key-values (leafs) are going to be added to a
// Tree, for these cases is more effitient to use:
invalids, err := tree.AddBatch(keys, values)

// generate the merkle proof of a leaf by it's key
value, siblings, err := tree.GenProof(key)

// verify the proof
verified, err := arbo.CheckProof(tree.hashFunction, key, value, tree.Root(), siblings)
if !verified {
	fmt.Println("proof could not be verified")
}

// get the value of a leaf assigned to a key
gettedKey, gettedValue, err := tree.Get(key)

// update the value of a leaf assigned to a key
err = tree.Update(key, value)

// dump the tree (the leafs)
dump, err := tree.Dump(nil) // instead of nil, a root to start from can be used

// import the dump into a tree
err = tree.ImportDump(dump)

// print graphviz diagram of the tree
err = tree.PrintGraphviz(nil) // instead of nil, a root to start from can be used
```

### Usage with SNARKs compatibility
Arbo is designed to be compatible with [circom merkle
tree](https://github.com/iden3/circomlib/tree/master/circuits/smt)'s
snark-friendly merkletree.
The only change needed is the hash function used for the Tree, for example using
the Poseidon hash function:
```go
tree, err := arbo.NewTree(database, 100, arbo.HashFunctionPoseidon)
```
Be aware of the characteristics of this kind of hashes, such as using values
inside the finite field used by the hash, and also the computation time.

The interface of arbo uses byte arrays, and for the case of these kind of hashes
(that usually work directly with finite field elements), arbo expects those
values to be represented by little-endian byte arrays. There is a helper method
to convert a `*big.Int` to `[]byte` using little-endian:
```go
bLen := tree.HashFunction().Len()
kBigInt := big.NewInt(100)

// convert *big.Int to byte array
kBytes := arbo.BigIntToBytes(bLen, kBigInt)

// convert byte array to *big.Int
kBigInt2 := arbo.BytesToBigInt(kBytes)
```
