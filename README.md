[![Go Report Card](https://goreportcard.com/badge/github.com/scout-inc/scout-go)](https://goreportcard.com/report/github.com/scout-inc/scout-go)
[![GoDoc](https://godoc.org/github.com/scout-inc/scout-go?status.svg)](https://godoc.org/github.com/scout-inc/scout-go)
[![codecov](https://codecov.io/gh/scout-inc/scout-go/branch/main/graph/badge.svg)](https://codecov.io/gh/scout-inc/scout-go)

# scout-go

The official Go SDK for Scout. Read the docs at [https://docs.getscout.dev/sdks/go/overview](https://docs.getscout.dev/sdks/go/overview)

## Usage

Require package:
```
go get github.com/scout-inc/scout-go
```

In your entrypoint function:

```go
import "github.com/scout-inc/scout-go"

func main() {
	// some code

	scout.Init(
		scout.WithProjectID(SCOUT_PROJECT_ID)
	)
	defer scout.Stop()
	
	// some code
}
```

Scout provides middleware for the more common Go server frameworks:

`go-chi/chi`:
```go
import (
	"github.com/scout-inc/scout-go"
	s "github.com/scout-inc/scout-go/middleware/chi"
)

func main() {
	// some code
	scout.Init(
		scout.WithProjectID(SCOUT_PROJECT_ID)
	)
	defer scout.Stop()

	r := chi.NewMux()
	r.Use(s.Middleware)
	// some code
}
```

`gin-gonic/gin`:
```go
import (
	"github.com/scout-inc/scout-go"
	s "github.com/scout-inc/scout-go/middleware/gin"
)

func main() {
	// some code
	scout.Init(
		scout.WithProjectID(SCOUT_PROJECT_ID)
	)
	defer scout.Stop()

	r := chi.NewMux()
	r.Use(s.Middleware())
	// some code
}
```

See [https://docs.getscout.dev/sdks/go/frameworks](https://docs.getscout.dev/sdks/go/frameworks) for more examples.

To manually record an error:
```go
import (
	...
	"go.opentelemetry.io/otel/attribute"
)

func LoginWithOAuth(ctx context.Context, email string) {
	value, err := doSomething()
	if err != nil {
		// tags can be used to enrich errors and traces with additional information
		errorTags := []attribute.KeyValue{
			{
				Key: attribute.Key("user.email"),
				Value: attribute.StringValue(email)
			}
		}
		scout.RecordError(ctx, err, errorTags...)
	}
	// some code
}
```