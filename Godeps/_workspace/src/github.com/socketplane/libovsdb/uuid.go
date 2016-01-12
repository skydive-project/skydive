package libovsdb

import (
	"encoding/json"
	"errors"
	"regexp"
)

type UUID struct {
	GoUuid string `json:"uuid"`
}

// <set> notation requires special marshaling
func (u UUID) MarshalJSON() ([]byte, error) {
	var uuidSlice []string
	err := u.validateUUID()
	if err == nil {
		uuidSlice = []string{"uuid", u.GoUuid}
	} else {
		uuidSlice = []string{"named-uuid", u.GoUuid}
	}

	return json.Marshal(uuidSlice)
}

func (u *UUID) UnmarshalJSON(b []byte) (err error) {
	var ovsUuid []string
	if err := json.Unmarshal(b, &ovsUuid); err == nil {
		u.GoUuid = ovsUuid[1]
	}
	return err
}

func (u UUID) validateUUID() error {
	if len(u.GoUuid) != 36 {
		return errors.New("uuid exceeds 36 characters")
	}

	var validUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

	if !validUUID.MatchString(u.GoUuid) {
		return errors.New("uuid does not match regexp")
	}

	return nil
}
