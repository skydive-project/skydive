/*
Package ipfix implements an IPFIX (RFC 5101) parser and interpreter.

An input stream in the form of an `io.Reader`, `net.PacketConn`, or a
`[]byte` is read and chunked into messages. Template management and the
standard IPFIX types are implemented so a fully parsed data set can be
produced. Vendor fields can be added at runtime.

License

The MIT license.
*/
package ipfix
