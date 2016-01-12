ipfix
-----

Package ipfix implements an IPFIX (RFC 5101) parser and interpreter.

[![Build Status](https://img.shields.io/circleci/project/calmh/ipfix.svg?style=flat-square)](https://circleci.com/gh/calmh/ipfix)
[![API Documentation](http://img.shields.io/badge/api-Godoc-blue.svg?style=flat-square)](http://godoc.org/github.com/calmh/ipfix)
[![MIT License](http://img.shields.io/badge/license-MIT-blue.svg?style=flat-square)](http://opensource.org/licenses/MIT)

An input stream in the form of an `io.Reader`, `net.PacketConn`, or a
`[]byte` is read and chunked into messages. Template management and the
standard IPFIX types are implemented so a fully parsed data set can be
produced. Vendor fields can be added at runtime.

## Example

To read an IPFIX stream, create a Session around a Reader, then call
ParseReader repeatedly.


To add a vendor field to the dictionary so that it will be resolved by
Interpret, create a DictionaryEntry and call AddDictionaryEntry.

    e := ipfix.DictionaryEntry{Name: "someVendorField", FieldId: 42, EnterpriseId: 123456, Type: ipfix.Int32}
    s.AddDictionaryEntry(e)

## License

The MIT license.

## Usage

See the [documentation](http://godoc.org/github.com/calmh/ipfix).

