# Test Suite

To run the tests:

```
docker-compose build
docker-compose up -d
go run ../../cmd/vochaintest/vochaintest.go --oracleKey 6aae1d165dd9776c580b8fdaf8622e39c5f943c715e20690080bbfce2c760223 --electionSize=1000
```

there's also a bash script:
```
./start-test.sh
```

if you run this interactively in a headless environment (remote server), you might face the following error:

```
failed to solve with frontend dockerfile.v0: failed to solve with frontend gateway.v0: rpc error: code = Unknown desc = error getting credentials - err: exit status 1, out: `Cannot autolaunch D-Bus without X11 $DISPLAY`
```
this means `docker login` is not finding `pass` command (i.e. it's not installed in your server) and falling back to launching a d-bus server, which fails.
you can simply install `pass`, or an even more simple workaround is to make a dummy symlink
```
# ln -s /bin/true /usr/local/bin/pass
```
since `docker login` just checks that `pass` is available, but doesn't actually need it for login, all of the repos accesed are public.
