go-sieve
====================

Sieve email filtering language ([RFC 5228]) interpreter implementation in Go.

## Features

* Binary representation for faster load/execute cycles (`Script.Save`, `sieve.RestoreSaved`).
* Integration tests harness for MTA integration testing (see `tests/execute.go`).

## Supported extensions

- envelope ([RFC 5228])
- fileinto ([RFC 5228])
- encoded-character ([RFC 5228])
- imap4flags ([RFC 5232])
- variables ([RFC 5229])
- relational ([RFC 5231])
- copy ([RFC 3894])
- reject/ereject ([RFC 5429])
- subaddress ([RFC 5233])
- environment ([RFC 5183])
- body ([RFC 5173])
- vacation ([RFC 5230])
- regex (draft-ietf-sieve-regex)
- date / index ([RFC 5260])
- editheader ([RFC 5293])
- mailbox ([RFC 5490])
- ManageSieve client+server ([RFC 5804])
- duplicate ([RFC 7352])
- ihave ([RFC 5463])

## Planned extensions

High priority:

- [ ] spamtest / virustest ([RFC 5235]) — spam/virus score testing
- [ ] enotify ([RFC 5435]) — event notifications (mailto:, xmpp:, …)
- [ ] include ([RFC 6609]) — include personal/global Sieve scripts
Medium priority:

- [x] ihave ([RFC 5463]) — runtime capability checking
- [ ] mboxmetadata / servermetadata ([RFC 5490] §4) — IMAP METADATA tests
- [ ] foreverypart + mime ([RFC 5703]) — MIME part iteration and header tests
- [ ] special-use ([RFC 8579]) — fileinto :specialuse "\\Junk"
- [ ] fcc ([RFC 8580]) — vacation :fcc — file carbon copy of auto-replies

Low priority:

- [ ] vacation-seconds ([RFC 6131]) — :seconds N parameter for vacation
- [ ] extlists ([RFC 6134]) — :list match type against external address books
- [ ] imapflags — compatibility alias for imap4flags

## Example

See ./cmd/sieve-run.

## Known issues

- Some invalid scripts are accepted as valid (see tests/compile_test.go)
- Comments in addresses are not ignored when testing equality, etc.
- Source routes in addresses are not ignored when testing equality, etc.

[RFC 5228]: https://datatracker.ietf.org/doc/html/rfc5228
[RFC 5229]: https://datatracker.ietf.org/doc/html/rfc5229
[RFC 5230]: https://datatracker.ietf.org/doc/html/rfc5230
[RFC 5231]: https://datatracker.ietf.org/doc/html/rfc5231
[RFC 3894]: https://datatracker.ietf.org/doc/html/rfc3894
[RFC 5429]: https://datatracker.ietf.org/doc/html/rfc5429
[RFC 5232]: https://datatracker.ietf.org/doc/html/rfc5232
[RFC 5233]: https://datatracker.ietf.org/doc/html/rfc5233
[RFC 5183]: https://datatracker.ietf.org/doc/html/rfc5183
[RFC 5173]: https://datatracker.ietf.org/doc/html/rfc5173
[RFC 5260]: https://datatracker.ietf.org/doc/html/rfc5260
[RFC 5293]: https://datatracker.ietf.org/doc/html/rfc5293
[RFC 5490]: https://datatracker.ietf.org/doc/html/rfc5490
[RFC 5804]: https://datatracker.ietf.org/doc/html/rfc5804
[RFC 5235]: https://datatracker.ietf.org/doc/html/rfc5235
[RFC 5435]: https://datatracker.ietf.org/doc/html/rfc5435
[RFC 5463]: https://datatracker.ietf.org/doc/html/rfc5463
[RFC 5703]: https://datatracker.ietf.org/doc/html/rfc5703
[RFC 6131]: https://datatracker.ietf.org/doc/html/rfc6131
[RFC 6134]: https://datatracker.ietf.org/doc/html/rfc6134
[RFC 6609]: https://datatracker.ietf.org/doc/html/rfc6609
[RFC 7352]: https://datatracker.ietf.org/doc/html/rfc7352
[RFC 8579]: https://datatracker.ietf.org/doc/html/rfc8579
[RFC 8580]: https://datatracker.ietf.org/doc/html/rfc8580
