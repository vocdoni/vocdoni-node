# arbo [![GoDoc](https://godoc.org/github.com/vocdoni/arbo?status.svg)](https://godoc.org/github.com/vocdoni/arbo) [![Go Report Card](https://goreportcard.com/badge/github.com/vocdoni/arbo)](https://goreportcard.com/report/github.com/vocdoni/arbo) [![Test](https://github.com/vocdoni/arbo/workflows/Test/badge.svg)](https://github.com/vocdoni/arbo/actions?query=workflow%3ATest)

> *arbo*: tree in Esperanto.

MerkleTree implementation in Go. Compatible with the circomlib implementation of
the MerkleTree. Specification: https://docs.iden3.io/publications/pdfs/Merkle-Tree.pdf and https://eprint.iacr.org/2018/955.

Main characteristics of arbo are:
- Allows to define which hash function to use.
	- So for example, when working with zkSnarks the [Poseidon hash](https://eprint.iacr.org/2019/458.pdf) function can be used, but when not, it can be used the [Blake2b hash](https://www.blake2.net/blake2.pdf) function, which has much faster computation time.
	- New hash functions can be plugged by just implementing the interface
- Parallelizes computation by CPUs
	- See [AddBatch section](https://github.com/vocdoni/arbo#addbatch)

## AddBatch
The method `tree.AddBatch` is designed for the cases where there is a big amount of key-values to be added in the tree. It has the following characteristics:
- Parallelizes by available CPUs
	- If the tree size is not too big (under the configured threshold):
		- Makes a copy of the tree in memory (*VirtualTree*)
		- The *VirtualTree* does not compute any hash, only the relations between the nodes of the tree
			- This step (computing the *VirtualTree*) is done in parallel in each available CPU until level *log2(nCPU)*
		- Once the *VirtualTree* is updated with all the new leafs (key-values) in each corresponent position, it *computes all the hashes* of each node until the root
			- In this way, each node hash is computed only once, while when adding many key-values using `tree.Add` method, most of the intermediate nodes will be recalculated each time that a new leaf is added
			- This step (*computing all the hashes*) is done in parallel in each available CPU
	- If the tree size is avobe the configured threshold:
		- Virtually splits the tree in `n` sub-trees, where `n` is the number of available CPUs
		- Each CPU adds the corresponent new leaves into each sub-tree (working in a db tx)
		- Once all sub-trees are updated, puts them together again to compute the new tree root

As result, the method `tree.AddBatch` goes way faster thant looping over `tree.Add`, and can compute the tree with parallelization, so as more available CPUs, faster will do the computation.

As an example, this is the benchmark for adding `10k leaves` (with `4 CPU cores`, `AddBatch` would get faster with more CPUs (powers of 2)):
```
Intel(R) Core(TM) i5-7200U CPU @ 2.50GHz with 8GB of RAM
nCPU: 4, nLeafs: 10_000

Using Poseidon hash function:
(go) arbo.AddBatch:	436.866007ms
(go) arbo.Add loop:	5.341122678s
(go) iden3.Add loop:	8.581494317s
(js) circomlibjs:	2m09.351s
```
And, for example, if instead of using Poseidon hash function we use Blake2b, time is reduced to `80.862805ms`.

## Usage

```go
// create new database
database, err := db.NewBadgerDB(c.TempDir())

// create new Tree with maxLevels=100 and Blake2b hash function
tree, err := arbo.NewTree(database, 100, arbo.HashFunctionBlake2b)

key := []byte("hello")
value := []byte("world")
// Add a key & value into the merkle tree
err = tree.Add(key, value)

// There are cases where multiple key-values (leafs) are going to be added to a
// Tree, for these cases is more efficient to use:
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
tree, err := arbo.NewTree(database, 32, arbo.HashFunctionPoseidon)
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
