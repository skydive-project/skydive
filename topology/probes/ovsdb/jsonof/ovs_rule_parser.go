/*
 * Copyright (C) 2018 Orange.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

/* Parsing OpenVswitch syntax for rules is inherently hard. The main reason
 * is the lack of clear lexical class for a few characters notably colon that
 * is both an important action token and a character in IPv6 and mac addresses.
 *
 * We have decided to follow a two phase approach. First we split the text in
 * main components so that filters and individual actions are recognized,
 * then we split the elementary filters and actions in simpler components. For
 * filters it only means checking if there is a mask. For actions, parsing may
 * be more involved.
 */

package jsonof

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/skydive-project/skydive/logging"
)

// JSONRule is an openflow rule ready for JSON export
type JSONRule struct {
	Cookie   uint64    `json:"Cookie"`         // cookie value of the rule
	Table    int       `json:"Table"`          // table containing the rule
	Priority int       `json:"Priority"`       // priority of rule
	Meta     []*Meta   `json:"Meta,omitempty"` // anything that is not a filter.
	Filters  []*Filter `json:"Filters"`        // all the filter
	Actions  []*Action `json:"Actions"`        // all the actions
}

// JSONGroup is an openflow group ready for JSON export
type JSONGroup struct {
	GroupID uint      `json:"GroupId"`        // id of the group
	Type    string    `json:"Type"`           // group type
	Meta    []*Meta   `json:"Meta,omitempty"` // anything that is not a bucket
	Buckets []*Bucket `json:"Buckets"`        // buckets
}

// Bucket is the representation of a bucket in an openflow group
type Bucket struct {
	ID      uint      `json:"Id"`             // id of bucket
	Meta    []*Meta   `json:"Meta,omitempty"` // anything that is not an action
	Actions []*Action `json:"Actions"`        // action list
}

// Action represents an atomic action in an openflow rule
type Action struct {
	Action    string    `json:"Function"`            // Action name
	Arguments []*Action `json:"Arguments,omitempty"` // Arguments if it exists
	Key       string    `json:"Key,omitempty"`       // Key for aguments such as k=v
}

// Filter is an elementary filter in an openflow rule
type Filter struct {
	Key   string `json:"Key"`            // left hand side
	Value string `json:"Value"`          // right hand side
	Mask  string `json:"Mask,omitempty"` // mask if used
}

// Meta is anything not a filter or an action always as a pair key/value
type Meta struct {
	Key   string `json:"Key"`   // key
	Value string `json:"Value"` // raw value
}

// Token is a lexical entity
type Token int

const (
	// Token values as recognized by scan
	tNt Token = iota
	tEOF
	tText
	tSpace
	tComma
	tEqual
	tClosePar
)

const (
	kwActions  = "actions"
	kwBucket   = "bucket"
	kwBucketID = "bucket_id"
	kwCookie   = "cookie"
	kwGroupID  = "group_id"
	kwLoad     = "load"
	kwMove     = "move"
	kwPriority = "priority"
	kwSetField = "set_field"
	kwTable    = "table"
	kwType     = "type"
)

// TokenNames is the array of printable names for Token.
var TokenNames = []string{
	"NT",
	"EOF",
	"TEXT",
	"SPACE",
	"COMMA",
	"EQUAL",
	"CPAR",
}

var eof = rune(0)

// Stream represents a text buffer that can be scanned
type Stream struct {
	r     *bufio.Reader
	last  rune
	token Token
	value string
}

// NewStream returns a new instance of Stream.
func NewStream(r io.Reader) *Stream {
	return &Stream{r: bufio.NewReader(r), last: eof, token: tNt}
}

// isWhitespace check if the rune is a classical separator
// (space of tab or eol)
func isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n'
}

// read reads the next rune from the bufferred reader.
// Returns the rune(0) if an error occurs (or io.EOF is returned).
func (s *Stream) read() rune {
	if s.last != eof {
		ch := s.last
		s.last = eof
		return ch
	}
	ch, _, err := s.r.ReadRune()
	if err != nil {
		return eof
	}
	return ch
}

// unread places the previously read rune back on the reader.
func (s *Stream) unread(r rune) { s.last = r }

// unscan puts back the previously read token.
func (s *Stream) unscan(tok Token, lit string) {
	s.token = tok
	s.value = lit
}

// scan returns the next token and literal value.
// nolint: gocyclo
func (s *Stream) scan() (tok Token, lit string) {
	if s.token != tNt {
		tok := s.token
		s.token = tNt
		return tok, s.value
	}
	// Read the next rune.
	ch := s.read()

	// If we see whitespace then consume all contiguous whitespace.
	// If we see a letter then consume as an ident or reserved word.
	switch ch {
	case eof:
		return tEOF, ""
	case ' ', '\t', '\n':
		for {
			ch = s.read()
			if ch == eof {
				break
			} else if !isWhitespace(ch) {
				s.unread(ch)
				break
			}
		}
		return tSpace, ""
	case '=':
		return tEqual, ""
	case ',':
		return tComma, ""
	case ')':
		return tClosePar, ""
	default:
		var buf bytes.Buffer
		buf.WriteRune(ch)
		if ch == '(' {
			s.fill(&buf, 1)
		} else {
			s.fill(&buf, 0)
		}
		return tText, buf.String()
	}
}

func (s *Stream) fill(buf *bytes.Buffer, parLevel int) {
fillLoop:
	for {
		ch := s.read()
		switch ch {
		case eof:
			break fillLoop
		case ' ', '\t', '\n', ',', '=':
			if parLevel == 0 {
				s.unread(ch)
				break fillLoop
			}
		case '(':
			parLevel = parLevel + 1
		case ')':
			if parLevel == 0 {
				s.unread(ch)
				break fillLoop
			}
			parLevel = parLevel - 1
		}
		_, err := buf.WriteRune(ch)
		if err != nil {
			logging.GetLogger().Errorf(
				"jsonof: fill cannot write into buffer: %s", err)
		}
	}
}

// ParseRule is the main entry point for the rule parser.
func (s *Stream) ParseRule(result *JSONRule) error {
	tok, val := s.scan()
	if tok == tSpace {
		tok, val = s.scan()
	}
	if tok == tText {
		return s.ParseRuleEq(result, []*Meta{}, val)
	}
	return errors.New("expecting an ident")
}

func makeFilter(pair *Meta) *Filter {
	rhs := strings.Split(pair.Value, "/")
	if len(rhs) == 2 {
		return &Filter{Key: pair.Key, Value: rhs[0], Mask: rhs[1]}
	}
	return &Filter{Key: pair.Key, Value: pair.Value, Mask: ""}
}

// ParseRuleEq implements the state of the rule parser waiting for an equal
// sign or a break signifying a next block (happens with filter abbreviations
// like ip, tcp, etc.)
func (s *Stream) ParseRuleEq(result *JSONRule, stack []*Meta, lhs string) error {
	tok, val := s.scan()
	if tok == tEqual {
		tok, val = s.scan()
		if tok == tText {
			return s.parseRulePair(result, stack, lhs, val)
		}
		return errors.New("expecting a right hand side")
	}
	if tok == tComma || tok == tSpace {
		pair := &Meta{Key: lhs}
		stack = append(stack, pair)
		switch lhs {
		case "reset_counts", "no_packet_counts", "no_byte_counts":
			result.Meta = append(result.Meta, stack...)
			return s.ParseRule(result)
		default:
			s.unscan(tok, val)
			return s.ParseRuleSep(result, stack)
		}
	}
	return errors.New("expecting = , or ''")
}

func (s *Stream) parseRulePair(
	result *JSONRule,
	stack []*Meta,
	lhs string, rhs string,
) error {
	switch lhs {
	case kwCookie:
		v, err := strconv.ParseUint(rhs, 0, 64)
		result.Cookie = v
		if err != nil {
			logging.GetLogger().Errorf(
				"Cannot parse cookie in openflow rule: %s", rhs)
		}
	case kwTable:
		v, err := strconv.ParseUint(rhs, 10, 32)
		result.Table = int(v)
		if err != nil {
			logging.GetLogger().Errorf(
				"Cannot parse table in openflow rule: %s", rhs)
		}
	case kwPriority:
		v, err := strconv.ParseUint(rhs, 10, 32)
		result.Priority = int(v)
		if err != nil {
			logging.GetLogger().Errorf(
				"Cannot parse priority in openflow rule: %s", rhs)
		}
	case kwActions:
		result.Actions = append(result.Actions, makeAction(rhs))
		return s.ParseRuleAction(result)
	default:
		var pair = &Meta{Key: lhs, Value: rhs}
		stack = append(stack, pair)
	}
	return s.ParseRuleSep(result, stack)
}

// ParseRuleSep implements the state of the parser afer an x=y, a break
// is expected but it may be either a
func (s *Stream) ParseRuleSep(result *JSONRule, stack []*Meta) error {
	tok, _ := s.scan()
	if tok == tComma {
		tok2, val2 := s.scan()
		if tok2 == tSpace {
			result.Meta = append(result.Meta, stack...)
			return s.ParseRule(result)
		}
		if tok2 == tText {
			return s.ParseRuleEq(result, stack, val2)
		}
		return errors.New("expected text or space after comma")
	}
	if tok == tSpace {
		for _, meta := range stack {
			result.Filters = append(result.Filters, makeFilter(meta))
		}
		return s.ParseRule(result)
	}
	return errors.New("expecting a comma or a space")
}

func makeArg(raw string) *Action {
	braPos := strings.Index(raw, "[")
	if braPos == -1 {
		return &Action{Action: raw}
	}
	if raw[len(raw)-1] != ']' {
		logging.GetLogger().Errorf("Expecting a closing bracket in %s", raw)
		return nil
	}
	actRange := Action{Action: "range"}
	field := raw[0:braPos]
	actRange.Arguments = append(actRange.Arguments, &Action{Action: field})
	if len(raw)-braPos > 2 {
		content := raw[braPos+1 : len(raw)-1]
		dotPos := strings.Index(content, "..")
		if dotPos == -1 {
			actRange.Arguments = append(
				actRange.Arguments, &Action{Action: content})
		} else {
			start := content[0:dotPos]
			end := content[dotPos+2:]
			actRange.Arguments = append(
				actRange.Arguments,
				&Action{Action: start}, &Action{Action: end})
		}
	}
	return &actRange
}

func makeFieldAssign(action *Action, rem string) {
	arrowStart := strings.Index(rem, "->")
	if arrowStart == -1 {
		logging.GetLogger().Errorf(
			"Expecting an arrow in action %s:%s", action.Action, rem)
	} else {
		arg1 := rem[0:arrowStart]
		arg2 := rem[arrowStart+2:]
		switch action.Action {
		case kwLoad:
			action.Arguments = append(
				action.Arguments,
				&Action{Action: arg1}, makeArg(arg2))
		case kwMove:
			action.Arguments = append(
				action.Arguments,
				makeArg(arg1), makeArg(arg2))
		case kwSetField:
			slashPos := strings.Index(arg1, "/")
			if slashPos == -1 {
				action.Arguments = append(
					action.Arguments,
					&Action{Action: arg1}, nil, makeArg(arg2))
			} else {
				arg11 := arg1[0:slashPos]
				arg12 := arg1[slashPos+1:]
				action.Arguments = append(
					action.Arguments,
					&Action{Action: arg11}, &Action{Action: arg12},
					makeArg(arg2))
			}
		}
	}
}

func makeAction(raw string) *Action {
	actionSep := strings.IndexAny(raw, ":(")
	if actionSep == -1 {
		return makeArg(raw)
	}
	key := raw[0:actionSep]
	action := Action{Action: key}
	rem := raw[actionSep+1:]
	if raw[actionSep] == ':' {
		switch key {
		case kwSetField, kwLoad, kwMove:
			makeFieldAssign(&action, rem)
		case "enqueue":
			// This syntax is not consistent, poor choice of ovs
			// enqueue(port, queue) should be prefered.
			colonPos := strings.Index(rem, ":")
			if colonPos == -1 {
				// Probably should never happens
				action.Arguments = append(action.Arguments, makeArg(rem))
			} else {
				port := rem[0:colonPos]
				queue := rem[colonPos+1:]
				action.Arguments = append(
					action.Arguments,
					&Action{Action: port}, &Action{Action: queue})
			}
		default:
			action.Arguments = append(action.Arguments, makeArg(rem))
		}
	} else {
		s := NewStream(strings.NewReader(rem))
		err := s.ParseActionBody(&action, true)
		if key == "learn" {
			fixLearnArguments(&action)
		}
		if err != nil {
			logging.GetLogger().Errorf("Parsing arguments of %s: %s", raw, err)
		}
	}
	return &action
}

// Learn is a strange beast because some of the k=v arguments are not named
// arguments but field assignment. We need to transform them back in real
// actions and this can only be done if we have a dictionnary of named
// arguments for learn.
func fixLearnArguments(action *Action) {
	for i, arg := range action.Arguments {
		switch arg.Key {
		case "", "idle_timeout", "hard_timeout", kwPriority, "cookie",
			"fin_idle_timeout", "fin_hard_timeout", kwTable, "limit",
			"result_dst":
			continue
		default:
			actAssign := Action{
				Action:    "=",
				Arguments: []*Action{makeArg(arg.Key), arg},
			}
			arg.Key = ""
			action.Arguments[i] = &actAssign
		}
	}
}

// ParseActionBody reads the arguments of an action using parenthesis.
func (s *Stream) ParseActionBody(act *Action, isFirst bool) error {
	tok, v := s.scan()
	if tok == tText {
		var a *Action
		tok1, v1 := s.scan()
		if tok1 != tEqual {
			s.unscan(tok1, v1)
			a = makeAction(v)
		} else {
			tok2, v2 := s.scan()
			if tok2 == tText {
				a = makeAction(v2)
				a.Key = v
			} else {
				return errors.New("expecting argument after equal")
			}
		}
		act.Arguments = append(act.Arguments, a)
		tok4, _ := s.scan()
		if tok4 == tComma {
			return s.ParseActionBody(act, false)
		} else if tok4 == tClosePar {
			return nil
		} else {
			return errors.New("expecting comma or closing par")
		}
	} else if tok == tComma {
		act.Arguments = append(act.Arguments, nil)
		return s.ParseActionBody(act, false)
	} else if tok == tClosePar {
		if !isFirst {
			act.Arguments = append(act.Arguments, nil)
		}
		return nil
	}
	return errors.New("expecting argument or argument separator")
}

// ParseRuleAction implements the state of the parser while reading an action
// list. We only expect text separated by commas and ending of EOF.
func (s *Stream) ParseRuleAction(result *JSONRule) error {
	tok, _ := s.scan()
	if tok == tEOF {
		return nil
	} else if tok == tComma {
		tok, val := s.scan()
		if tok == tText {
			result.Actions = append(result.Actions, makeAction(val))
			return s.ParseRuleAction(result)
		}
		return errors.New("parseRuleAction: expecting an action after comma")
	}
	return errors.New("parseRuleAction: expecting a comma or eof")
}

// ParseGroup is the main entry point for the group parser.
func (s *Stream) ParseGroup(result *JSONGroup) error {
	tok, lhs := s.scan()
	if tok == tSpace {
		tok, lhs = s.scan()
	}
	if tok == tText {
		return s.parseGroupEq(result, lhs)
	}
	if tok == tEOF && len(result.Buckets) > 0 {
		return nil
	}
	return fmt.Errorf("expecting id or eof, got %s", TokenNames[tok])
}

func (s *Stream) parseGroupSep(result *JSONGroup) error {
	tok, _ := s.scan()
	if tok == tComma {
		tok2, lhs := s.scan()
		if tok2 == tText {
			return s.parseGroupEq(result, lhs)
		}
		return fmt.Errorf("expecting key after comma, got %s", TokenNames[tok])
	}
	if tok == tEOF {
		return nil
	}
	return fmt.Errorf("expecting comma or eof, got %s", TokenNames[tok])
}

func (s *Stream) parseGroupEq(result *JSONGroup, lhs string) error {
	tok, v := s.scan()
	var rhs string
	if tok != tEqual {
		s.unscan(tok, v)
		rhs = ""
	} else {
		if lhs == kwBucket {
			bucket := &Bucket{}
			if err := s.parseGroupBucket(bucket); err != nil {
				return err
			}
			result.Buckets = append(result.Buckets, bucket)
			return s.ParseGroup(result)
		}
		tok, rhs = s.scan()
		if tok != tText {
			return fmt.Errorf("expecting rhs of equal, got %s", TokenNames[tok])
		}
	}
	switch lhs {
	case kwGroupID:
		v, err := strconv.ParseUint(rhs, 0, 32)
		result.GroupID = uint(v)
		if err != nil {
			logging.GetLogger().Errorf(
				"Cannot parse group_id in openflow group: %s", rhs)
		}
	case kwType:
		result.Type = rhs
	default:
		result.Meta = append(result.Meta, &Meta{Key: lhs, Value: rhs})
	}
	return s.parseGroupSep(result)
}

func (s *Stream) parseGroupBucket(result *Bucket) error {
	for {
		tok, v := s.scan()
		if tok != tText {
			return fmt.Errorf("expecting id in pair, got %s", TokenNames[tok])
		}
		switch v {
		case kwActions:
			tok, _ = s.scan()
			if tok != tEqual {
				return fmt.Errorf("Expecting =, got %s", TokenNames[tok])
			}
			return s.parseGroupBucketActions(result)
		case kwBucket:
			s.unscan(tok, v)
			return nil
		default:
			if err := parseMetaBucket(result, v); err != nil {
				return err
			}
		}
		tok, _ = s.scan()
		if tok == tEOF {
			return nil
		}
		if tok != tComma {
			return fmt.Errorf(
				"cannot parse bucket, expecting comma or eof, got: %s",
				TokenNames[tok])
		}
	}
}

func parseMetaBucket(result *Bucket, v string) error {
	colonPos := strings.Index(v, ":")
	if colonPos == -1 {
		result.Meta = append(result.Meta, &Meta{Key: v, Value: ""})
	} else {
		lhs := v[0:colonPos]
		rhs := v[colonPos+1:]
		if lhs == kwBucketID {
			bID, err := strconv.ParseUint(rhs, 0, 32)
			if err != nil {
				return fmt.Errorf("Cannot parse bucket id: %s", rhs)
			}
			result.ID = uint(bID)
		} else {
			result.Meta = append(result.Meta, &Meta{Key: v, Value: ""})
		}
	}
	return nil
}

func (s *Stream) parseGroupBucketActions(result *Bucket) error {
	for {
		tok, v := s.scan()
		if tok != tText {
			return fmt.Errorf("expecting id, got %s", TokenNames[tok])
		}
		if v == kwBucket {
			s.unscan(tok, v)
			return nil
		}
		result.Actions = append(result.Actions, makeAction(v))
		tok, _ = s.scan()
		if tok == tComma {
			continue
		} else if tok == tEOF {
			return nil
		} else {
			return fmt.Errorf("expecting comma or eof, got %s", TokenNames[tok])
		}
	}
}

// ToAST transforms a string representing an openflow rule in an
// abstract syntax tree of the rule
func ToAST(rule string) (*JSONRule, error) {
	stream := NewStream(strings.NewReader(rule))
	var result JSONRule
	if err := stream.ParseRule(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ToASTGroup transforms a string representing an openflow rule in an
// abstract syntax tree of the rule
func ToASTGroup(group string) (*JSONGroup, error) {
	stream := NewStream(strings.NewReader(group))
	var result JSONGroup
	if err := stream.ParseGroup(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ToJSON transforms a string representing an openflow rule in a string
// that is the encoding of the rule.got
func ToJSON(rule string) (string, error) {
	ast, err := ToAST(rule)
	if err != nil {
		return "", err
	}
	jsBytes, err := json.Marshal(ast)
	if err != nil {
		return "", fmt.Errorf("cannot jsonify: %s", rule)
	}
	return string(jsBytes), nil
}

// ToJSONGroup transforms a string representing an openflow group in a string
// that is the encoding of the group.
func ToJSONGroup(group string) (string, error) {
	ast, err := ToASTGroup(group)
	if err != nil {
		return "", err
	}
	jsBytes, err := json.Marshal(ast)
	if err != nil {
		return "", fmt.Errorf("cannot jsonify: %s", group)
	}
	return string(jsBytes), nil
}

func writeAction(s *bytes.Buffer, a *Action) {
	if a == nil {
		return
	}
	if a.Key != "" {
		s.WriteString(a.Key) // nolint: gas
		s.Write([]byte("=")) // nolint: gas
	}
	if a.Action == "range" {
		writeAction(s, a.Arguments[0])
		s.Write([]byte("[")) // nolint: gas
		switch len(a.Arguments) {
		case 2:
			writeAction(s, a.Arguments[1])
		case 3:
			writeAction(s, a.Arguments[1])
			s.Write([]byte("..")) // nolint: gas
			writeAction(s, a.Arguments[2])
		}
		s.Write([]byte("]")) // nolint: gas
		return
	}
	if a.Action == "=" {
		writeAction(s, a.Arguments[0])
		// Design decision not to choose = as ovs which is pretty confusing.
		s.Write([]byte(":=")) // nolint: gas
		writeAction(s, a.Arguments[1])
		return
	}
	s.WriteString(a.Action) // nolint: gas
	if len(a.Arguments) > 0 {
		s.Write([]byte("(")) // nolint: gas
		for i, arg := range a.Arguments {
			if i > 0 {
				s.Write([]byte(",")) // nolint: gas
			}
			writeAction(s, arg)
		}
		s.Write([]byte(")")) // nolint: gas
	}
}

// PrettyAST gives back a string from a AST
//
// The syntax is close to the one used by OVS but without the quirks.
// The most significant differences are: move, load, set_field, enqueue
// (as regular actions) and fields in learn actions (using := instead of =).
func PrettyAST(ast *JSONRule) string {
	// TODO: use string buffer when go minimal version bumps to 1.10
	var s bytes.Buffer
	s.Write([]byte("cookie=0x"))
	s.WriteString(strconv.FormatUint(ast.Cookie, 16))
	s.Write([]byte(", table="))
	s.WriteString(strconv.Itoa(ast.Table))
	s.Write([]byte(", "))
	for _, meta := range ast.Meta {
		s.WriteString(meta.Key)
		if meta.Value != "" {
			s.Write([]byte("="))
			s.WriteString(meta.Value)
		}
		s.Write([]byte(", "))
	}
	s.Write([]byte("priority="))
	s.WriteString(strconv.Itoa(ast.Priority))
	for _, filter := range ast.Filters {
		s.Write([]byte(","))
		s.WriteString(filter.Key)
		if filter.Value != "" {
			s.Write([]byte("="))
			s.WriteString(filter.Value)
			if filter.Mask != "" {
				s.Write([]byte("/"))
				s.WriteString(filter.Mask)
			}
		}
	}
	s.Write([]byte(" actions="))
	for i, action := range ast.Actions {
		if i > 0 {
			s.Write([]byte(","))
		}
		writeAction(&s, action)
	}
	return s.String()
}

// PrettyASTGroup gives back a string from a AST
//
// The syntax is close to the one used by OVS but without the quirks.
func PrettyASTGroup(ast *JSONGroup) string {
	// TODO: use string buffer when go minimal version bumps to 1.10
	var s bytes.Buffer
	s.Write([]byte("group_id="))
	s.WriteString(strconv.FormatUint(uint64(ast.GroupID), 10))
	s.Write([]byte(", type="))
	s.WriteString(ast.Type)
	for _, meta := range ast.Meta {
		s.Write([]byte(", "))
		s.WriteString(meta.Key)
		if meta.Value != "" {
			s.Write([]byte("="))
			s.WriteString(meta.Value)
		}
	}
	for _, bucket := range ast.Buckets {
		s.Write([]byte(", bucket=bucket_id:"))
		s.WriteString(strconv.FormatUint(uint64(bucket.ID), 10))
		for _, meta := range bucket.Meta {
			s.Write([]byte(","))
			s.WriteString(meta.Key)
			if meta.Value != "" {
				s.Write([]byte(":"))
				s.WriteString(meta.Value)
			}
		}
		s.Write([]byte(",actions="))
		for i, action := range bucket.Actions {
			if i > 0 {
				s.Write([]byte(","))
			}
			writeAction(&s, action)
		}
	}
	return s.String()
}
