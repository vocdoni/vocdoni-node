## dvote Tree

Implementation of dvote tree structure. Currently based on iden3 merkle tree.

Example of usage:

```
  T := tree.Tree
  if T.Init() != nil { fmt.Println("Cannot create tree database") }
  err := T.AddClaim([]byte("Hello you!"))
  if err != nil {
    fmt.Println("Claim already exist")
  }
  mpHex, err := T.GenProof([]byte("Hello you!"))
  fmt.Println(mpHex)
  fmt.Println(T.CheckProof([]byte("Hello you!"), mpHex))
  T.Close()
```

#### To-Do

+ Add export/import methods
