// +build gofuzz

package ipfix

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"code.google.com/p/gcfg"
)

var extra []DictionaryEntry

func init() {
	if dictFile := os.Getenv("IPFIXDICT"); dictFile != "" {
		var dict UserDictionary
		err := gcfg.ReadFileInto(&dict, dictFile)
		if err != nil {
			panic(err)
		}

		for name, field := range dict.Field {
			extra = append(extra, field.DictionaryEntry(name))
		}
	}
}

func Fuzz(bs []byte) int {
	r := bytes.NewReader(bs)

	s := NewSession()
	i := NewInterpreter(s)
	for _, e := range extra {
		i.AddDictionaryEntry(e)
	}

	msg, err := s.ParseReader(r)
	for err == nil {
		for _, rec := range msg.DataRecords {
			i.Interpret(rec)
		}
		msg, err = s.ParseReader(r)
	}
	if err == io.EOF {
		return 1
	}

	return 0
}

type UserDictionary struct {
	Field map[string]*Field
}

type Field struct {
	ID         uint16
	Enterprise uint32
	Type       string
}

func (f Field) DictionaryEntry(name string) DictionaryEntry {
	return DictionaryEntry{
		Name:         name,
		EnterpriseID: f.Enterprise,
		FieldID:      f.ID,
		Type:         FieldTypes[f.Type],
	}
}
