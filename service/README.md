## dvot-census http service library

#### How to use it

```
processHttp.T.Init()
processHttp.Listen(1500, "http", "")
```

To enable authentication (using pubKey signature):

```
pubK := "39f54ce5293520b689f6658ea7f3401f4ff931fa3d90dea21ff901cdf82bb8aa"
processHttp.Listen(1500, "http", pubK)
```
