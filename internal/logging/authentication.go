package logging

import "github.com/hashicorp/terraform-plugin-framework/types"

// AuthenticationValue returns the named string attribute of an `authentication`
// object, or an empty string when the object or attribute is null or unknown.
func AuthenticationValue(auth types.Object, name string) types.String {
	if auth.IsNull() || auth.IsUnknown() {
		return types.StringValue("")
	}
	value, ok := auth.Attributes()[name]
	if !ok || value == nil || value.IsNull() || value.IsUnknown() {
		return types.StringValue("")
	}
	stringValue, ok := value.(types.String)
	if !ok {
		return types.StringValue("")
	}
	return stringValue
}
