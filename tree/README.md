#### Dvote Tree

Implementation of dvote tree structure. Currently based on iden3 merkle tree.

Example of usage:

```
  T := tree.Tree {namespace: "vocdoni"}
  if T.init() != nil { fmt.Println("Cannot create tree database") }
  err := T.addClaim([]byte("Hello you!"))
  if err != nil {
    fmt.Println("Claim already exist")
  }
  mpHex, err := T.genProof([]byte("Hello you!"))
  fmt.Println(mpHex)
  fmt.Println(T.checkProof([]byte("Hello you!"), mpHex))
  T.close()
```

### To-Do

+ Add export/import methods
