# Protocol Buffers

To send data (such as a struct) over a network, we need to _encode the data into a format to transmit_.

When we _don't_ control the client, using JSON makes perfect sense, since it is both readable for human and easy to parse for computers.

But when we _do_ control the client, we can use protocol buffers (a.k.a protobuf)

## How it works

Protobuf lets us define how we want our data structured, then our protobuf gets compiled into many languages. Our structured data then gets read from and written to different data streams.

## Why use Protol Buffers?

_Consistent schemas_ - Encode the semantics once and use them across our services. Protobuf ensures a _consistent data model_ throughout the whole system.

_Versioning for free_ - Think of protobuf message like a Go struct, because when you compile a message, it turns into a struct. We can number the fields on the messages to maintain backward compatibility?

Less boilerplate, extensibility, language agnoticism, performance...

## Installation

```bash
# NOTE: Check the latest version and change accordingly
PROTOC_ZIP=protoc-3.15.8-linux-x86_64.zip
curl -OL https://github.com/google/protobuf/releases/download/v3.15.8/$PROTOC_ZIP
sudo unzip -o $PROTOC_ZIP -d /usr/local bin/protoc
sudo unzip -o $PROTOC_ZIP -d /usr/local include/*
rm -f $PROTOC_ZIP
```

> Fields are immutable.

## How to use generated files from Protobuf

We use them as if we have written them ourselves.

Mostly we would use the getters.
