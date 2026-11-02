package translator

// DomainMapping tells how a custom value is sent to an HA entity domain:
// an optional service override, the payload key and a value conversion.
type DomainMapping struct {
	Service string
	Param   string
	Convert func(float64) any
}

func asIs(v float64) any  { return v }
func asInt(v float64) any { return int(v) }

var defaultDomainMapping = DomainMapping{Param: "value", Convert: asIs}

var domainMappings = map[string]DomainMapping{
	"light":        {Param: "brightness", Convert: asIs},
	"cover":        {Service: "set_cover_position", Param: "position", Convert: asInt},
	"climate":      {Service: "set_temperature", Param: "temperature", Convert: asIs},
	"input_number": {Service: "set_value", Param: "value", Convert: asIs},
}
