# ws
A Generic WS Client


### Warning

this is work in progress and not even remotely stable code and very limited in its functionality.


### Usage

```go
import (
ws "github.com/codingsandmore/ws"
)
ws := *ws.NewDefaultClient("wss://pumpportal.fun/api/data")

ws.Subscribe....
```