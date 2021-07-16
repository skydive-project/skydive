// Code generated by easyjson for marshaling/unmarshaling. DO NOT EDIT.

package ovnmodel

import (
	json "encoding/json"
	easyjson "github.com/mailru/easyjson"
	jlexer "github.com/mailru/easyjson/jlexer"
	jwriter "github.com/mailru/easyjson/jwriter"
)

// suppress unused package warning
var (
	_ *json.RawMessage
	_ *jlexer.Lexer
	_ *jwriter.Writer
	_ easyjson.Marshaler
)

func easyjson8063ea4DecodeGithubComSkydiveProjectSkydiveTopologyProbesOvnOvnmodel(in *jlexer.Lexer, out *SSL) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeFieldName(false)
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "UUID":
			out.UUID = string(in.String())
		case "BootstrapCaCert":
			out.BootstrapCaCert = bool(in.Bool())
		case "CaCert":
			out.CaCert = string(in.String())
		case "Certificate":
			out.Certificate = string(in.String())
		case "ExternalIDs":
			if in.IsNull() {
				in.Skip()
			} else {
				in.Delim('{')
				if !in.IsDelim('}') {
					out.ExternalIDs = make(map[string]string)
				} else {
					out.ExternalIDs = nil
				}
				for !in.IsDelim('}') {
					key := string(in.String())
					in.WantColon()
					var v1 string
					v1 = string(in.String())
					(out.ExternalIDs)[key] = v1
					in.WantComma()
				}
				in.Delim('}')
			}
		case "PrivateKey":
			out.PrivateKey = string(in.String())
		case "SSLCiphers":
			out.SSLCiphers = string(in.String())
		case "SSLProtocols":
			out.SSLProtocols = string(in.String())
		case "ExternalIDsMeta":
			(out.ExternalIDsMeta).UnmarshalEasyJSON(in)
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjson8063ea4EncodeGithubComSkydiveProjectSkydiveTopologyProbesOvnOvnmodel(out *jwriter.Writer, in SSL) {
	out.RawByte('{')
	first := true
	_ = first
	if in.UUID != "" {
		const prefix string = ",\"UUID\":"
		first = false
		out.RawString(prefix[1:])
		out.String(string(in.UUID))
	}
	if in.BootstrapCaCert {
		const prefix string = ",\"BootstrapCaCert\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.Bool(bool(in.BootstrapCaCert))
	}
	if in.CaCert != "" {
		const prefix string = ",\"CaCert\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.String(string(in.CaCert))
	}
	if in.Certificate != "" {
		const prefix string = ",\"Certificate\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.String(string(in.Certificate))
	}
	if len(in.ExternalIDs) != 0 {
		const prefix string = ",\"ExternalIDs\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		{
			out.RawByte('{')
			v2First := true
			for v2Name, v2Value := range in.ExternalIDs {
				if v2First {
					v2First = false
				} else {
					out.RawByte(',')
				}
				out.String(string(v2Name))
				out.RawByte(':')
				out.String(string(v2Value))
			}
			out.RawByte('}')
		}
	}
	if in.PrivateKey != "" {
		const prefix string = ",\"PrivateKey\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.String(string(in.PrivateKey))
	}
	if in.SSLCiphers != "" {
		const prefix string = ",\"SSLCiphers\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.String(string(in.SSLCiphers))
	}
	if in.SSLProtocols != "" {
		const prefix string = ",\"SSLProtocols\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.String(string(in.SSLProtocols))
	}
	if len(in.ExternalIDsMeta) != 0 {
		const prefix string = ",\"ExternalIDsMeta\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		(in.ExternalIDsMeta).MarshalEasyJSON(out)
	}
	out.RawByte('}')
}

// MarshalJSON supports json.Marshaler interface
func (v SSL) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjson8063ea4EncodeGithubComSkydiveProjectSkydiveTopologyProbesOvnOvnmodel(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v SSL) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson8063ea4EncodeGithubComSkydiveProjectSkydiveTopologyProbesOvnOvnmodel(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *SSL) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjson8063ea4DecodeGithubComSkydiveProjectSkydiveTopologyProbesOvnOvnmodel(&r, v)
	return r.Error()
}

// UnmarshalEasyJSON supports easyjson.Unmarshaler interface
func (v *SSL) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson8063ea4DecodeGithubComSkydiveProjectSkydiveTopologyProbesOvnOvnmodel(l, v)
}