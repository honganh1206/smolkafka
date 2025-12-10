# Secure your services

We need to build a tool that _streams data securely_.

Security is the topmost priority:

- Security saves your ass from being hacked (Think of data breaches making you on the news)
- Security wins deals with potential customers.
- Security is painful to deal with later on our work. Better deal with it at the start.

Three steps, that's all:

1. [[Encrypt data in-flight]]
2. [[Authenticate to identify clients]]
3. [[Authorize to give authenticated clients necessary permissions]]

We need authorization with granular access control. We will operate our own certificate authority (CA) with `cfssl` by Cloudflare.

For our simple internal services, we do not need trusted certificate from a 3rd-party authority like Let's Encrypt.

## `cfssl` package from Cloudflare

A toolkit for verifying, signing and bundling TLS certificates. Think of it like like our own CA.

Two tools we need:

- `cfssl` to sign, verify and bundle TLS certificates and output the result as JSON.
- `cfssl` to take that output and _split that into separate key, certificate, CSR and bundle files_
