# kat
## Sample usage

```
var (
    err     error
    config  *rest.Config
    session kat.Session
)

if config, err = rest.InClusterConfig(); err != nil {
    log.Fatalln(err.Error())
}

if session, err = kat.New(
    config, "dcr.repo.io/image:label",
); err != nil {
    log.Fatalln(err.Error())
}

if err = session.Start(); err != nil {      // Start() is blocking until Pod comes alive
    log.Fatalln(err.Error())
}

var (
    data       io.Reader      // Arrange for input-data in here
    sout, serr *bytes.Buffer
)

sout, serr = bytes.NewBuffer(make([]byte, 32768)), bytes.NewBuffer(make([]byte, 32768))
if err = session.Attach(data, sout, serr); err != nil {
    log.Fatalln(err.Error())
}
```
