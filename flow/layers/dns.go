/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package layers

import (
	"errors"

	"github.com/google/gopacket/layers"
)

func newDNSRecordFromLayersRecord(rr *layers.DNSResourceRecord) DNSResourceRecord {
	return DNSResourceRecord{
		Name:       string(rr.Name),
		Type:       rr.Type.String(),
		Class:      rr.Class.String(),
		TTL:        rr.TTL,
		DataLength: rr.DataLength,
	}
}

func toFlowDNSResourceRecord(rr *layers.DNSResourceRecord) (DNSResourceRecord, error) {
	switch rr.Type {
	case layers.DNSTypeA, layers.DNSTypeAAAA:
		record := newDNSRecordFromLayersRecord(rr)
		record.IP = rr.IP.String()
		return record, nil
	case layers.DNSTypeSOA:
		record := newDNSRecordFromLayersRecord(rr)
		record.SOA = &DNSSOA{
			MName:   string(rr.SOA.MName),
			RName:   string(rr.SOA.RName),
			Serial:  rr.SOA.Serial,
			Refresh: rr.SOA.Refresh,
			Retry:   rr.SOA.Retry,
			Expire:  rr.SOA.Expire,
			Minimum: rr.SOA.Minimum,
		}
		return record, nil
	case layers.DNSTypeSRV:
		record := newDNSRecordFromLayersRecord(rr)
		record.SRV = &DNSSRV{
			Priority: rr.SRV.Priority,
			Weight:   rr.SRV.Weight,
			Port:     rr.SRV.Port,
			Name:     string(rr.SRV.Name),
		}
		return record, nil
	case layers.DNSTypeMX:
		record := newDNSRecordFromLayersRecord(rr)
		record.MX = &DNSMX{
			Preference: rr.MX.Preference,
			Name:       string(rr.MX.Name),
		}
		return record, nil
	case layers.DNSTypeTXT, layers.DNSTypeHINFO:
		record := newDNSRecordFromLayersRecord(rr)
		TXTs := make([]string, len(rr.TXTs))
		for i, v := range rr.TXTs {
			TXTs[i] = string(v)
		}
		record.TXTs = TXTs
		return record, nil
	case layers.DNSTypeNS:
		record := newDNSRecordFromLayersRecord(rr)
		record.NS = string(rr.NS)
		return record, nil
	case layers.DNSTypeCNAME:
		record := newDNSRecordFromLayersRecord(rr)
		record.CNAME = string(rr.CNAME)
		return record, nil
	case layers.DNSTypePTR:
		record := newDNSRecordFromLayersRecord(rr)
		record.PTR = string(rr.PTR)
		return record, nil
	case layers.DNSTypeOPT:
		record := newDNSRecordFromLayersRecord(rr)
		OPTs := make([]*DNSOPT, len(rr.OPT))
		for i, v := range rr.OPT {
			OPTs[i].Code = v.Code.String()
			OPTs[i].Data = string(v.Data)
		}
		record.OPT = OPTs
		return record, nil

	}

	return DNSResourceRecord{}, errors.New("No Matching Type")
}

// GetDNSQuestions returns the dns questions from a given packet
func GetDNSQuestions(dq []layers.DNSQuestion) []DNSQuestion {
	if len(dq) > 0 {
		questions := make([]DNSQuestion, len(dq))
		for i, v := range dq {
			questions[i] = DNSQuestion{
				Name:  string(v.Name),
				Type:  v.Type.String(),
				Class: v.Class.String(),
			}
		}
		return questions
	}
	return nil
}

// GetDNSRecords returns a dns record
func GetDNSRecords(records []layers.DNSResourceRecord) []DNSResourceRecord {
	if len(records) > 0 {
		holders := make([]DNSResourceRecord, 0, len(records))
		for _, v := range records {
			if resource, err := toFlowDNSResourceRecord(&v); err == nil {
				holders = append(holders, resource)
			}
		}
		return holders
	}
	return nil
}
