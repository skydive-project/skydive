package ipfix

import "strings"

// This implements RFC 5103, Bidirectional Flow Export Using IP Flow
// Information Export (IPFIX)

// IPFIX Reverse Information Element ("reverse PEN")
const reversePEN = 29305

func init() {
	for k, v := range builtinDictionary {
		if k.EnterpriseID != 0 {
			continue
		}

		switch k.FieldID {
		case 148, 145, 149, 137:
			// Not reversible: flowId, templateId, observationDomainId, and
			// commonPropertiesId

		case 130, 131, 217, 211, 212, 213, 214, 215, 216, 173:
			// Not reversible: process configuration elements defined in
			// Section 5.2 of RFC5102.

		case 41, 40, 42, 163, 164, 165, 166, 167, 168:
			// Not reversible: process statistics elements defined in Section
			// 5.3 of RFC5102.

		case 210:
			// Not reversible: paddingOctets

		default:
			// Reversible!
			v.Name = "reverse" + strings.ToUpper(v.Name[0:1]) + v.Name[1:]
			v.EnterpriseID = reversePEN
			k.EnterpriseID = reversePEN
			builtinDictionary[k] = v
		}
	}
}
