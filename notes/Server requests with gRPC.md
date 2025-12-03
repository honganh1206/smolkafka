# Server requests with gRPC

Problems: Library used on a single computer + Users have to learn the API.

Solution: Turn our library to a web service.

We must enable the gRPC plugin and compile our gRPC service:

```bash
go get google.golang.org/grpc@latest
go get google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

```Makefile
# Sequence: Run Go code generator (go_out) to generate structs, enums, helper types
# -> Run Go gRPC generator for interfaces and client/server stubs?
compile:
    protoc api/v1/*.proto \
        --go_out=. \
        --go-grpc_out=. \
        --go_opt=paths=source_relative \
        --go-grpc_opt=paths=source_relative \
        --proto_path=.
```

Our gRPC server will listen to potential incoming connections via a listener. The connections are from the log clients communicating via gRPC method?

## Error handling in gRPC

Our service's code is so short because we _send the client whatever error our library returns_.

However, developers from the client wants to know the errors, so we need to send back a human-readable version of the error for the client to show the user.

We use the `status` package from the Golang gRPC library for this.

```go
// On server side, if we cannot find a record
// we pass in the error string with the status code
err := status.Error(codes.NotFound, "id was not found")
return nil, err

// On client side
st, ok := status.FromError(err)
if !ok {
    // Error was not a status error
}
// Use st.Message() and st.Code()
```

We use `withDetails()` to provide more details for the errors:

```go
// On server side
st := status.New(codes.NotFound, "id was not found")
d := &errdetails.LocalizedMessage{
    Locale: "en-US",
    Message: fmt.Sprintf(
        "We couldn't find a user with the email address: %s",
        id,
    ),
}
var err error

st, err = st.WithDetails(d)
if err != nil {
    // If this errored, it will always error
    // here, so better panic so we can figure
    // out why than have this silently passing.
    panic(fmt.Sprintf("Unexpected error attaching metadata: %v", err))
}
return st.Err()

// On client side
st := status.Convert(err)
for _, detail := range st.Details() {
    switch t := detail.(type) {
    case *errdetails.LocalizedMessage:
        // send t.Message back to the user
    }
}
```
