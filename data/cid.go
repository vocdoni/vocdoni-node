package data

import (
	"crypto/sha256"
	"encoding/binary"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	"go.vocdoni.io/dvote/log"
)

const (
	// chunkSize is the size of the leaf blocks the content is split into,
	// matching the default IPFS fixed-size chunker ("size-262144").
	chunkSize = 256 << 10
	// maxLinks is the maximum number of children of an intermediate node,
	// matching the IPFS balanced DAG builder default.
	maxLinks = 174
	// unixfsTypeFile is the UnixFS Data.Type value for a file node.
	unixfsTypeFile = 2
)

// dagNode is an encoded dag-pb node of a UnixFS file DAG.
type dagNode struct {
	cid      []byte // binary CIDv1 (dag-pb, sha2-256)
	tsize    uint64 // encoded size of the node plus all its descendants
	fileSize uint64 // bytes of file content below this node
}

// CalculateCIDv1json returns the IPFS CIDv1 (dag-pb, sha2-256, base32) that
// "ipfs add --cid-version=1" assigns to data with the default fixed-size chunker,
// balanced layout and UnixFS (non-raw) leaves. The DAG is built in memory only.
func CalculateCIDv1json(data []byte) string {
	b := &dagBuilder{data: data}
	var root dagNode
	if len(data) == 0 {
		root = b.leaf(nil)
	} else {
		root = b.leaf(b.next())
		for depth := 1; !b.done(); depth++ {
			root = b.fill([]dagNode{root}, depth)
		}
	}
	c, err := cid.Cast(root.cid)
	if err != nil {
		// cannot happen: the bytes are built by this package
		log.Errorw(err, "could not cast computed cid")
		return ""
	}
	return c.String()
}

// CIDequals compares two CIDs (v0 or v1) and reports whether they address the
// same content. It compares the multihash, not the codec or encoding, and
// accepts the "ipfs://" and "/ipfs/" prefixes.
func CIDequals(cid1, cid2 string) bool {
	cid1 = strings.TrimPrefix(strings.TrimPrefix(cid1, "ipfs://"), "/ipfs/")
	cid2 = strings.TrimPrefix(strings.TrimPrefix(cid2, "ipfs://"), "/ipfs/")
	c1, err := cid.Decode(cid1)
	if err != nil {
		log.Debugw("could not decode cid", "cid", cid1, "err", err.Error())
		return false
	}
	c2, err := cid.Decode(cid2)
	if err != nil {
		log.Debugw("could not decode cid", "cid", cid2, "err", err.Error())
		return false
	}
	return c1.Hash().String() == c2.Hash().String()
}

// dagBuilder lays out the chunks of data as an IPFS balanced DAG.
type dagBuilder struct {
	data   []byte
	offset int
}

func (b *dagBuilder) done() bool {
	return b.offset >= len(b.data)
}

func (b *dagBuilder) next() []byte {
	end := min(b.offset+chunkSize, len(b.data))
	chunk := b.data[b.offset:end]
	b.offset = end
	return chunk
}

// fill adds children to an intermediate node, starting with the given ones,
// until it holds maxLinks or the data is exhausted. Children at depth 1 are
// leaves; deeper children are filled recursively.
func (b *dagBuilder) fill(children []dagNode, depth int) dagNode {
	for len(children) < maxLinks && !b.done() {
		if depth == 1 {
			children = append(children, b.leaf(b.next()))
		} else {
			children = append(children, b.fill(nil, depth-1))
		}
	}
	return b.inner(children)
}

// leaf encodes a UnixFS file node holding chunk as its data. A nil chunk
// (empty file) omits the data field.
func (*dagBuilder) leaf(chunk []byte) dagNode {
	fs := appendVarintField(nil, 1, unixfsTypeFile)
	if chunk != nil {
		fs = appendBytesField(fs, 2, chunk)
	}
	fs = appendVarintField(fs, 3, uint64(len(chunk)))
	enc := appendBytesField(nil, 1, fs)
	return dagNode{cid: sumCID(enc), tsize: uint64(len(enc)), fileSize: uint64(len(chunk))}
}

// inner encodes a UnixFS file node linking to children.
func (*dagBuilder) inner(children []dagNode) dagNode {
	var enc []byte
	var fileSize, tsize uint64
	for _, c := range children {
		fileSize += c.fileSize
		tsize += c.tsize
		link := appendBytesField(nil, 1, c.cid)
		link = appendBytesField(link, 2, nil) // empty name
		link = appendVarintField(link, 3, c.tsize)
		enc = appendBytesField(enc, 2, link)
	}
	fs := appendVarintField(nil, 1, unixfsTypeFile)
	fs = appendVarintField(fs, 3, fileSize)
	for _, c := range children {
		fs = appendVarintField(fs, 4, c.fileSize)
	}
	enc = appendBytesField(enc, 1, fs)
	return dagNode{cid: sumCID(enc), tsize: tsize + uint64(len(enc)), fileSize: fileSize}
}

// sumCID returns the binary CIDv1 (dag-pb, sha2-256) of an encoded node.
func sumCID(enc []byte) []byte {
	h := sha256.Sum256(enc)
	out := binary.AppendUvarint(nil, 1)
	out = binary.AppendUvarint(out, cid.DagProtobuf)
	out = binary.AppendUvarint(out, multihash.SHA2_256)
	out = binary.AppendUvarint(out, uint64(len(h)))
	return append(out, h[:]...)
}

// appendVarintField appends a protobuf varint field.
func appendVarintField(b []byte, field int, v uint64) []byte {
	b = binary.AppendUvarint(b, uint64(field)<<3)
	return binary.AppendUvarint(b, v)
}

// appendBytesField appends a protobuf length-delimited field.
func appendBytesField(b []byte, field int, v []byte) []byte {
	b = binary.AppendUvarint(b, uint64(field)<<3|2)
	b = binary.AppendUvarint(b, uint64(len(v)))
	return append(b, v...)
}
