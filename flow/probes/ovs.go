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

package probes

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/socketplane/libovsdb"

	"github.com/skydive-project/skydive/ovs/ovsdb"
)

func ovsProbeID(i string) string {
	return strings.Replace(i, "-", "_", -1)
}

func ovsNamedUUID(i string) string {
	return "row_" + strings.Replace(i, "-", "_", -1)
}

func ovsRowProbeID(row *map[string]interface{}) (string, error) {
	extIds := (*row)["external_ids"]
	switch extIds.(type) {
	case []interface{}:
		sl := extIds.([]interface{})
		bSliced, err := json.Marshal(sl)
		if err != nil {
			return "", err
		}

		switch sl[0] {
		case "map":
			var oMap libovsdb.OvsMap
			err = json.Unmarshal(bSliced, &oMap)
			if err != nil {
				return "", err
			}

			if value, ok := oMap.GoMap["skydive-probe-id"]; ok {
				return value.(string), nil
			}
		}
	}

	return "", errors.New("Not found")
}

func ovsRetrieveSkydiveProbeRowUUIDs(ovsClient *ovsdb.OvsClient, table string) ([]string, error) {
	condition := libovsdb.NewCondition("_uuid", "!=", libovsdb.UUID{GoUUID: "abc"})
	operations := []libovsdb.Operation{{Op: "select", Table: table, Where: []interface{}{condition}}}
	result, err := ovsClient.Exec(operations...)
	if err != nil {
		return nil, err
	}

	var uuids []string
	for _, o := range result {
		for _, row := range o.Rows {
			u := row["_uuid"].([]interface{})[1]
			uuid := u.(string)

			if _, err := ovsRowProbeID(&row); err == nil {
				uuids = append(uuids, uuid)
			}
		}
	}

	return uuids, nil
}

func ovsRetrieveSkydiveProbeRowUUID(ovsClient *ovsdb.OvsClient, table, id string) (string, error) {
	condition := libovsdb.NewCondition("_uuid", "!=", libovsdb.UUID{GoUUID: "abc"})
	operations := []libovsdb.Operation{{Op: "select", Table: table, Where: []interface{}{condition}}}
	result, err := ovsClient.Exec(operations...)
	if err != nil {
		return "", err
	}

	for _, o := range result {
		for _, row := range o.Rows {
			u := row["_uuid"].([]interface{})[1]
			uuid := u.(string)

			if probeID, err := ovsRowProbeID(&row); err == nil && probeID == id {
				return uuid, nil
			}
		}
	}

	return "", nil
}
