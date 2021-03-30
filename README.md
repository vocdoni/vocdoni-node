# arbo [![GoDoc](https://godoc.org/github.com/arnaucube/arbo?status.svg)](https://godoc.org/github.com/arnaucube/arbo) [![Go Report Card](https://goreportcard.com/badge/github.com/arnaucube/arbo)](https://goreportcard.com/report/github.com/arnaucube/arbo) [![Test](https://github.com/arnaucube/arbo/workflows/Test/badge.svg)](https://github.com/arnaucube/arbo/actions?query=workflow%3ATest)

MerkleTree implementation in Go. Compatible with the circomlib implementation of
the MerkleTree (when using the Poseidon hash function), following the
specification from https://docs.iden3.io/publications/pdfs/Merkle-Tree.pdf and
https://eprint.iacr.org/2018/955.

Allows to define which hash function to use. So for example, when working
with zkSnarks the Poseidon hash function can be used, but when not, it can be
used the Blake3 hash function, which improves the computation time.
