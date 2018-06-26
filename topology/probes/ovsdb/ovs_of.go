/*
 * Copyright (C) 2017 Orange.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package ovsdb

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	uuid "github.com/nu7hatch/gouuid"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
)

// OvsOfProbe is the type of the probe retrieving Openflow rules on an Open Vswitch
type OvsOfProbe struct {
	sync.Mutex
	Host         string                    // The host
	Graph        *graph.Graph              // The graph that will receive the rules found
	Root         *graph.Node               // The root node of the host in the graph
	BridgeProbes map[string]*BridgeOfProbe // The table of probes associated to each bridge
	Translation  map[string]string         // A translation table to find the url for a given bridge knowing its name
	Certificate  string                    // Path to the certificate used for authenticated communication with bridges
	PrivateKey   string                    // Path of the private key authenticating the probe.
	CA           string                    // Path of the certicate of the Certificate authority used for authenticated communication with bridges
	sslOk        bool                      // cert private key and ca are provisionned.
}

// BridgeOfProbe is the type of the probe retrieving Openflow rules on a Bridge.
//
// An important notion is the rawUUID of a rule or the UUID obtained by ignoring the priority from the
// rule filter. Several rules may differ only by their priority (and associated actions). In practice the
// highest priority hides the other rules. It is important to handle rules with the same rawUUID as a group
// because ovs-ofctl monitor does not report priorities.
type BridgeOfProbe struct {
	Host       string             // The global host
	Bridge     string             // The bridge monitored
	UUID       string             // The UUID of the bridge node
	Address    string             // The address of the bridge if different from name
	BridgeNode *graph.Node        // the bridge node on which the rule nodes are attached.
	OvsOfProbe *OvsOfProbe        // Back pointer to the probe
	Rules      map[string][]*Rule // The set of rules found so far grouped by rawUUID
	cancel     context.CancelFunc
}

// Rule is an OpenFlow rule in a switch
type Rule struct {
	Cookie   uint64 // cookie value of the rule
	Table    int    // table containing the rule
	Priority int    // priority of rule
	Filter   string // all the filters as a comma separated string
	Actions  string // all the actions (comma separated)
	InPort   int    // -1 is any
	UUID     string // Unique id
}

// Event is an event as monitored by ovs-ofctl monitor <br> watch:
type Event struct {
	RawRule *Rule   // The rule from the event
	Rules   []*Rule // Rules found by ovs-ofctl matching the event rule filter.
	Date    int64   // the date of the event
	Action  string  // the action taken
	Bridge  string  // The bridge whtere it ocured
}

// ProtectCommas substitute commas with semicolon
// when inside parenthesis
func protectCommas(line string) string {
	work := []rune(line)
	var braces = 0
	for i, word := range work {
		switch word {
		case '(':
			braces++
		case ')':
			braces--
		case ',':
			if braces > 0 {
				work[i] = ';'
			}
		}
	}
	return string(work)
}

// fillIn is a utility function that takes a splitted rule line
// and fills a Rule/Event structure with it
func fillIn(components []string, rule *Rule, event *Event) {
	for _, component := range components {
		keyvalue := strings.SplitN(component, "=", 2)
		if len(keyvalue) == 2 {
			key := keyvalue[0]
			value := keyvalue[1]
			switch key {
			case "event":
				if event != nil {
					event.Action = value
				}
			case "actions":
				rule.Actions = value
			case "table":
				table, err := strconv.ParseInt(value, 10, 32)
				if err == nil {
					rule.Table = int(table)
				} else {
					logging.GetLogger().Errorf("Error while parsing table of rule: %s", err.Error())
				}
			case "cookie":
				v, err := strconv.ParseUint(value, 0, 64)
				if err == nil {
					rule.Cookie = v
				} else {
					logging.GetLogger().Errorf("Error while parsing cookie of rule: %s", err.Error())
				}
			}
		}
	}

}

// extractPriority parses the filter of a rule and extracts the priority if it exists.
func extractPriority(rule *Rule) {
	components := strings.Split(rule.Filter, ",")
	rule.Priority = 32768 // Default rule priority.
	for _, component := range components {
		keyvalue := strings.SplitN(component, "=", 2)
		if len(keyvalue) == 2 {
			key := keyvalue[0]
			value := keyvalue[1]
			switch key {
			case "priority":
				priority, err := strconv.ParseInt(value, 10, 32)
				if err == nil {
					rule.Priority = int(priority)
				} else {
					logging.GetLogger().Errorf("Error while parsing priority of rule: %s", err.Error())
				}
			}
		}
	}
}

type noEventError struct{}

func (e *noEventError) Error() string {
	return "No Event"
}

// parseEvent transforms a single line of ofctl monitor :watch
// in an event. The string must be terminated by a "\n"
// Protected commas will be replaced.
func parseEvent(line string, bridge string, prefix string) (Event, error) {
	var result Event
	var rule Rule

	if line[0] != ' ' {
		return result, &noEventError{}
	}
	if strings.ContainsRune(line, '(') {
		line = protectCommas(line)
	}
	components := strings.Split(line[1:len(line)-1], " ")
	fillIn(components, &rule, &result)
	if len(components) < 2 {
		return result, errors.New("Rule syntax")
	}
	tentative := len(components) - 1
	if strings.HasPrefix(components[tentative], "actions=") {
		tentative = tentative - 1
	}
	if !strings.HasPrefix(components[tentative], "cookie=") {
		rule.Filter = components[tentative]
	}
	result.RawRule = &rule
	fillUUID(&rule, prefix)
	result.Date = time.Now().Unix()
	result.Bridge = bridge
	return result, nil
}

// Generates a unique UUID for the rule
// prefix is a unique string per bridge using bridge and host names.
func fillUUID(rule *Rule, prefix string) {
	id := prefix + rule.Filter + "-" + string(rule.Table) + "-" + string(rule.Cookie)
	u, err := uuid.NewV5(uuid.NamespaceOID, []byte(id))
	if err == nil {
		rule.UUID = u.String()
	}
}

// parseRule transforms a single line of ofctl dump-flow in a rule.
// The line DOES NOT include the terminating newline. Protected commas will be replaced.
func parseRule(line string) (*Rule, error) {
	var rule Rule
	if len(line) == 0 || line[0] != ' ' {
		return nil, errors.New("No rule: " + line)
	}
	if strings.ContainsRune(line, '(') {
		line = protectCommas(line)
	}
	components := strings.Split(line[1:], ", ")
	if len(components) < 2 {
		return nil, errors.New("Rule syntax")
	}
	fillIn(components, &rule, nil)
	tail := components[len(components)-1]
	components = strings.Split(tail, " actions=")
	if len(components) == 2 {
		rule.Filter = components[0]
		rule.Actions = components[1]
	} else {
		return nil, errors.New("Rule syntax split filter and actions")
	}
	return &rule, nil
}

func makeFilter(rule *Rule) string {
	if rule.Filter == "" {
		return fmt.Sprintf("table=%d", rule.Table)
	}
	return fmt.Sprintf("table=%d,%s", rule.Table, rule.Filter)
}

// Execute exposes an interface to command launch on the OS
type Execute interface {
	ExecCommand(string, ...string) ([]byte, error)
	ExecCommandPipe(context.Context, string, ...string) (io.Reader, error)
}

// RealExecute is the actual implementation given below. It can be overridden for tests.
type RealExecute struct{}

var executor Execute = RealExecute{}

// ExecCommand executes a command on a host
func (r RealExecute) ExecCommand(com string, args ...string) ([]byte, error) {
	/* #nosec */
	command := exec.Command(com, args...)
	return command.CombinedOutput()
}

// ExecCommandPipe executes a command on a host and gives back a pipe to control it.
func (r RealExecute) ExecCommandPipe(ctx context.Context, com string, args ...string) (io.Reader, error) {
	/* #nosec */
	command := exec.CommandContext(ctx, com, args...)
	out, err := command.StdoutPipe()
	command.Stderr = command.Stdout
	if err != nil {
		return out, err
	}

	err = command.Start()
	return out, err
}

// launchOnSwitch launches a command on a given switch
func launchOnSwitch(cmd []string) (string, error) {
	bytes, err := executor.ExecCommand(cmd[0], cmd[1:]...)
	if err == nil {
		return string(bytes), nil
	}
	return "", err
}

// launchContinuousOnSwitch launches  a stream producing command on a given switch. The command is resilient
// and is relaunched until the context explicitly cancels it.
func launchContinuousOnSwitch(ctx context.Context, cmd []string) (<-chan string, error) {
	var cout = make(chan string, 10)

	go func() {
		for ctx.Err() == nil {
			out, err := executor.ExecCommandPipe(ctx, cmd[0], cmd[1:]...)
			if err != nil {
				logging.GetLogger().Errorf("Can't execute command %v", cmd)
				close(cout)
				return
			}
			reader := bufio.NewReader(out)
			var line string
			for ctx.Err() == nil {
				line, err = reader.ReadString('\n')
				if err == io.EOF {
					break
				} else if err != nil {
					logging.GetLogger().Errorf("IO Error on command %v: %s", cmd, err.Error())
				} else {
					cout <- line
				}
			}
			logging.GetLogger().Debugf("Closing command: %v", cmd)
			time.Sleep(time.Second)
		}
		close(cout)
		logging.GetLogger().Debugf("Terminating command: %v", cmd)
	}()

	return cout, nil
}

func countElements(filter string) int {
	if len(filter) == 0 {
		return 1
	}
	elts := strings.Split(filter, ",")
	l := len(elts) + 1
	for _, elt := range elts {
		if strings.HasPrefix(elt, "priority=") {
			l = l - 1
			break
		}
	}
	return l
}

// completeEvent completes the event by looking at it again but with dump-flows and a filter including table. This gives back more elements such as priority.
func completeEvent(ctx context.Context, o *OvsOfProbe, event *Event, prefix string) error {
	oldrule := event.RawRule
	bridge := event.Bridge
	// We want exactly n+1 items where n was the number of items in old filters. The reason is that now
	// the priority is provided. Another approach would be to use the shortest filter as it is the more generic
	expected := countElements(oldrule.Filter)
	filter := makeFilter(oldrule)
	versions := strings.Join(config.GetStringSlice("ovs.oflow.openflow_versions"), ",")
	command, err1 := o.makeCommand(
		[]string{"ovs-ofctl", "-O", versions, "dump-flows"},
		bridge, filter)
	if err1 != nil {
		return err1
	}
	lines, err := launchOnSwitch(command)
	if err != nil && ctx.Err() == nil {
		return fmt.Errorf("Cannot launch ovs-ofctl dump-flows on %s@%s with filter %s: %s", bridge, o.Host, filter, err.Error())
	}
	for _, line := range strings.Split(lines, "\n") {
		rule, err2 := parseRule(line)
		if err2 == nil && countElements(rule.Filter) == expected && oldrule.Cookie == rule.Cookie {
			fillUUID(rule, prefix)
			extractPriority(rule)
			event.Rules = append(event.Rules, rule)
		}
	}
	return nil
}

// addRule adds a rule to the graph and links it to the bridge.
func (probe *BridgeOfProbe) addRule(rule *Rule) {
	logging.GetLogger().Infof("New rule %v added", rule.UUID)
	g := probe.OvsOfProbe.Graph
	g.Lock()
	defer g.Unlock()
	bridgeNode := probe.BridgeNode
	metadata := graph.Metadata{
		"Type":     "ofrule",
		"cookie":   fmt.Sprintf("0x%x", rule.Cookie),
		"table":    rule.Table,
		"filters":  rule.Filter,
		"actions":  rule.Actions,
		"priority": rule.Priority,
		"UUID":     rule.UUID,
	}
	ruleNode := g.NewNode(graph.GenID(), metadata)
	g.Link(bridgeNode, ruleNode, graph.Metadata{"RelationType": "ownership"})
}

// delRule deletes a rule from the the graph.
func (probe *BridgeOfProbe) delRule(rule *Rule) {
	logging.GetLogger().Infof("Rule %v deleted", rule.UUID)
	g := probe.OvsOfProbe.Graph
	g.Lock()
	defer g.Unlock()

	ruleNode := g.LookupFirstNode(graph.Metadata{"UUID": rule.UUID})
	if ruleNode != nil {
		g.DelNode(ruleNode)
	}
}

func containsRule(rules []*Rule, searched *Rule) bool {
	for _, rule := range rules {
		if rule.UUID == searched.UUID {
			return true
		}
	}
	return false
}

// monitor monitors the openflow rules of a bridge by launching a goroutine. The context is used to control the execution of the routine.
func (probe *BridgeOfProbe) monitor(ctx context.Context) error {
	ofp := probe.OvsOfProbe
	command, err1 := ofp.makeCommand([]string{"ovs-ofctl", "monitor"}, probe.Bridge, "watch:")
	if err1 != nil {
		return err1
	}
	lines, err := launchContinuousOnSwitch(ctx, command)
	if err != nil {
		return err
	}
	go func() {
		prefix := probe.Host + "-" + probe.Bridge + "-"
		for line := range lines {
			if ctx.Err() != nil {
				break
			}
			event, err := parseEvent(line, probe.Bridge, prefix)
			if err == nil {
				err = completeEvent(ctx, ofp, &event, prefix)
				if err != nil {
					logging.GetLogger().Error(err.Error())
				}
				rawUUID := event.RawRule.UUID
				oldRules := probe.Rules[rawUUID]
				switch event.Action {
				case "ADDED":
					for _, rule := range event.Rules {
						if !containsRule(oldRules, rule) {
							oldRules = append(oldRules, rule)
							probe.addRule(rule)
						}
					}
					probe.Rules[rawUUID] = oldRules
				case "DELETED":
					for _, oldRule := range oldRules {
						if !containsRule(event.Rules, oldRule) {
							probe.delRule(oldRule)
						}
					}
					if len(event.Rules) == 0 {
						delete(probe.Rules, rawUUID)
					} else {
						probe.Rules[rawUUID] = event.Rules
					}
				}
			} else {
				if _, ok := err.(*noEventError); !ok {
					logging.GetLogger().Errorf("Error while monitoring %s@%s: %s", probe.Bridge, probe.Host, err.Error())
				}
			}

		}
	}()
	return nil
}

// NewBridgeProbe creates a probe and launch the active process
func (o *OvsOfProbe) NewBridgeProbe(host string, bridge string, uuid string, bridgeNode *graph.Node) (*BridgeOfProbe, error) {
	ctx, cancel := context.WithCancel(context.Background())
	address, ok := o.Translation[bridge]
	if !ok {
		address = bridge
	}
	probe := &BridgeOfProbe{
		Host:       host,
		Bridge:     bridge,
		UUID:       uuid,
		Address:    address,
		BridgeNode: bridgeNode,
		OvsOfProbe: o,
		Rules:      make(map[string][]*Rule),
		cancel:     cancel}
	err := probe.monitor(ctx)
	return probe, err
}

func (o *OvsOfProbe) makeCommand(commands []string, bridge string, args ...string) ([]string, error) {
	commandLine := []string{}
	commandLine = append(commandLine, commands...)
	if strings.HasPrefix(bridge, "ssl:") {
		if o.sslOk {
			commandLine = append(commandLine,
				bridge,
				"--certificate", o.Certificate,
				"--ca-cert", o.CA, "--private-key", o.PrivateKey)
		} else {
			return commands, errors.New("Certificate, CA and private keys are necessary for communication with switch over SSL")
		}
	} else {
		commandLine = append(commandLine, bridge)
	}
	commandLine = append(commandLine, args...)
	return commandLine, nil
}

// OnOvsBridgeAdd is called when a bridge is added
func (o *OvsOfProbe) OnOvsBridgeAdd(bridgeNode *graph.Node) {
	o.Lock()
	defer o.Unlock()
	metadata := bridgeNode.Metadata()
	bridgeName := metadata["Name"].(string)
	uuid := metadata["UUID"].(string)
	hostName := o.Host
	if _, ok := o.BridgeProbes[uuid]; ok {
		return
	}
	bridgeProbe, err := o.NewBridgeProbe(hostName, bridgeName, uuid, bridgeNode)
	if err == nil {
		logging.GetLogger().Infof("Probe added for %s on %s (%s)", bridgeName, hostName, uuid)
		o.BridgeProbes[uuid] = bridgeProbe
	} else {
		logging.GetLogger().Errorf("Cannot add probe for bridge %s on %s (%s)", bridgeName, hostName, uuid)
	}
}

// OnOvsBridgeDel is called when a bridge is deleted
func (o *OvsOfProbe) OnOvsBridgeDel(uuid string, bridgeNode *graph.Node) {
	o.Lock()
	defer o.Unlock()
	if bridgeProbe, ok := o.BridgeProbes[uuid]; ok {
		bridgeProbe.cancel()
		delete(o.BridgeProbes, uuid)
	}
	// Clean all the rules attached to the bridge.
	if bridgeNode != nil {
		rules := o.Graph.LookupChildren(bridgeNode, graph.Metadata{"Type": "ofrule"}, nil)
		for _, ruleNode := range rules {
			logging.GetLogger().Infof("Rule %v deleted (Bridge deleted)", ruleNode.Metadata()["UUID"])
			o.Graph.DelNode(ruleNode)
		}
	}
}

// NewOvsOfProbe creates a new probe associated to a given graph, root node and host.
func NewOvsOfProbe(g *graph.Graph, root *graph.Node, host string) *OvsOfProbe {
	enable := config.GetBool("ovs.oflow.enable")
	if !enable {
		return nil
	}
	logging.GetLogger().Infof("Adding OVS probe on %s", host)
	translate := config.GetStringMapString("ovs.oflow.address")
	cert := config.GetString("ovs.oflow.cert")
	pk := config.GetString("ovs.oflow.key")
	ca := config.GetString("ovs.oflow.ca")
	sslOk := (pk != "") && (ca != "") && (cert != "")
	o := &OvsOfProbe{
		Host:         host,
		Graph:        g,
		Root:         root,
		BridgeProbes: make(map[string]*BridgeOfProbe),
		Translation:  translate,
		Certificate:  cert,
		PrivateKey:   pk,
		CA:           ca,
		sslOk:        sslOk,
	}
	return o
}
