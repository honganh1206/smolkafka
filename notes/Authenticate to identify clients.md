# Authenticate to identify clients

The process of identifying _who the client is_

Most web services use TLS for one-way authentication and only authenticate the server, because users must know they're talking to legitimate server. The server determines legitimate cients via user-password credentials and tokens.

In machine-to-machine scenarios, we would use TLS mutual authentication (two-way authentication), in which _the server and the client validate each other's communication_.
